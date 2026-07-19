package whatsapp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	waStore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// device wraps the single whatsmeow connection (one WhatsApp number) a Pool
// manages.
type device struct {
	client *whatsmeow.Client

	mu        sync.RWMutex
	jid       types.JID
	phone     string
	connected bool
}

// ErrAlreadyPaired is returned by Pair when the project already has a
// number — see the Pool doc comment for why a project is capped at one.
var ErrAlreadyPaired = errors.New("whatsapp: this project already has a number paired — each project is limited to one")

func (d *device) isConnected() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.connected
}

// Pool manages the single WhatsApp number (whatsmeow device) belonging to a
// project. A project is capped at exactly one number: round-robin across
// several numbers doesn't help a conversational project (a customer's reply
// could land on a different number than the one they messaged) and isn't a
// serious fix for OTP ban risk either — it dilutes but doesn't remove that
// risk, at the real operational cost of maintaining several actual phone
// numbers. A project that needs real OTP scale/reliability should use the
// Cloud API backend (CloudClient) instead of asking more of whatsmeow.
type Pool struct {
	container *sqlstore.Container
	logger    *slog.Logger

	deviceName     string
	backoff        []int
	simulateTyping bool
	ignoreGroups   bool
	ignoreStatus   bool

	events chan Event

	mu     sync.RWMutex
	device *device // nil until paired (or reconnected via LoadExisting)
}

// PoolConfig configures a Pool.
type PoolConfig struct {
	DBPath         string
	DeviceName     string
	Backoff        []int
	Logger         *slog.Logger
	SimulateTyping bool
	IgnoreGroups   bool
	IgnoreStatus   bool
}

// NewPool opens (or creates) the shared session store for a project and
// returns an empty Pool. Call LoadExisting to reconnect a previously paired
// number, and/or Pair to add one.
func NewPool(cfg PoolConfig) (*Pool, error) {
	container, err := sqlstore.New(context.Background(), "sqlite3",
		fmt.Sprintf("file:%s?_foreign_keys=on", cfg.DBPath),
		waLog.Noop,
	)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: open session store: %w", err)
	}

	return &Pool{
		container:      container,
		logger:         cfg.Logger,
		deviceName:     cfg.DeviceName,
		backoff:        cfg.Backoff,
		simulateTyping: cfg.SimulateTyping,
		ignoreGroups:   cfg.IgnoreGroups,
		ignoreStatus:   cfg.IgnoreStatus,
		events:         make(chan Event, 256),
	}, nil
}

// LoadExisting reconnects this project's paired number, if any. Call once
// at startup, after NewPool. A session store created before Pool was capped
// at one device could in principle still hold more than one paired device;
// if so, only the first is reconnected and the rest are logged and
// ignored, rather than silently reviving the multi-device behavior this
// type no longer supports.
func (p *Pool) LoadExisting(ctx context.Context) error {
	deviceStores, err := p.container.GetAllDevices(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp: get all devices: %w", err)
	}

	reconnected := false
	for _, ds := range deviceStores {
		if ds.ID == nil {
			continue // unpaired device store, nothing to reconnect
		}
		if reconnected {
			p.logger.Warn("whatsapp: ignoring extra paired device from before projects were capped at one number", "jid", ds.ID.String())
			continue
		}
		if err := p.connectDevice(ds); err != nil {
			p.logger.Error("whatsapp: failed to reconnect device", "jid", ds.ID.String(), "error", err)
			continue
		}
		reconnected = true
	}
	return nil
}

// connectDevice wraps an existing (already-paired) whatsmeow device store
// into a running client and sets it as this pool's device.
func (p *Pool) connectDevice(deviceStore *waStore.Device) error {
	waClient := whatsmeow.NewClient(deviceStore, waLog.Noop)
	d := &device{client: waClient}
	waClient.AddEventHandler(func(rawEvt any) { p.handleEvent(d, rawEvt) })

	if err := waClient.Connect(); err != nil {
		return fmt.Errorf("whatsapp: connect device: %w", err)
	}

	d.mu.Lock()
	d.connected = true
	if waClient.Store.ID != nil {
		d.jid = *waClient.Store.ID
		d.phone = waClient.Store.ID.User
	}
	d.mu.Unlock()

	p.mu.Lock()
	p.device = d
	p.mu.Unlock()
	p.logger.Info("whatsapp device reconnected", "phone", d.phone)
	return nil
}

// Pair starts pairing a brand-new WhatsApp number for this project and
// returns a channel of QR code strings to display. It refuses to start if
// this project already has a number paired — see the Pool doc comment for
// why a project can't have more than one.
func (p *Pool) Pair(ctx context.Context) (<-chan string, error) {
	p.mu.RLock()
	already := p.device != nil
	p.mu.RUnlock()
	if already {
		return nil, ErrAlreadyPaired
	}

	deviceStore := p.container.NewDevice()
	waClient := whatsmeow.NewClient(deviceStore, waLog.Noop)
	d := &device{client: waClient}
	waClient.AddEventHandler(func(rawEvt any) { p.handleEvent(d, rawEvt) })

	qrChan, err := waClient.GetQRChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: get qr channel: %w", err)
	}
	if err := waClient.Connect(); err != nil {
		return nil, fmt.Errorf("whatsapp: connect for pairing: %w", err)
	}

	out := make(chan string, 8)
	go func() {
		defer close(out)
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				select {
				case out <- evt.Code:
				case <-ctx.Done():
					return
				}
			case "success":
				d.mu.Lock()
				d.connected = true
				if waClient.Store.ID != nil {
					d.jid = *waClient.Store.ID
					d.phone = waClient.Store.ID.User
				}
				d.mu.Unlock()

				p.mu.Lock()
				p.device = d
				p.mu.Unlock()
				p.logger.Info("whatsapp number paired", "phone", d.phone)
				return
			case "timeout":
				p.logger.Warn("whatsapp pairing QR timed out")
				return
			}
		}
	}()

	return out, nil
}

// current returns this project's device, or an error if none is paired or
// the paired one is currently disconnected.
func (p *Pool) current() (*device, error) {
	p.mu.RLock()
	d := p.device
	p.mu.RUnlock()

	if d == nil {
		return nil, fmt.Errorf("whatsapp: no number paired for this project")
	}
	if !d.isConnected() {
		return nil, fmt.Errorf("whatsapp: number is disconnected")
	}
	return d, nil
}

// fetchMediaData resolves the raw bytes for SendMedia, from base64 or a URL
// download.
func fetchMediaData(ctx context.Context, url, base64Data string) ([]byte, error) {
	if base64Data != "" {
		data, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: invalid base64 data: %w", err)
		}
		return data, nil
	}
	if url != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: invalid url: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: download media: %w", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: read media: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("whatsapp: neither url nor base64Data provided")
}

// SendMessage sends a text message through this project's number.
func (p *Pool) SendMessage(ctx context.Context, phone, message string) (*SendResult, error) {
	d, err := p.current()
	if err != nil {
		return nil, err
	}
	jid := toJID(phone)

	if p.simulateTyping {
		_ = d.client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		time.Sleep(2 * time.Second)
		_ = d.client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}

	resp, err := d.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(message),
	})
	if err != nil {
		p.emitEvent(d, Event{Type: EventMessageFailed, Phone: phone, Error: err.Error(), At: time.Now().UTC()})
		return nil, fmt.Errorf("whatsapp: send message: %w", err)
	}

	p.emitEvent(d, Event{Type: EventMessageSent, Phone: phone, MessageID: resp.ID, At: time.Now().UTC()})
	return &SendResult{MessageID: resp.ID}, nil
}

// SendMedia sends a media message through this project's number.
func (p *Pool) SendMedia(ctx context.Context, phone, url, base64Data, caption string) (*SendResult, error) {
	d, err := p.current()
	if err != nil {
		return nil, err
	}
	data, err := fetchMediaData(ctx, url, base64Data)
	if err != nil {
		return nil, err
	}
	contentType := http.DetectContentType(data)
	jid := toJID(phone)

	if p.simulateTyping {
		_ = d.client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		time.Sleep(2 * time.Second)
		_ = d.client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}

	uploadResp, err := d.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: upload media: %w", err)
	}

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(contentType),
			URL:           proto.String(uploadResp.URL),
			DirectPath:    proto.String(uploadResp.DirectPath),
			MediaKey:      uploadResp.MediaKey,
			FileEncSHA256: uploadResp.FileEncSHA256,
			FileSHA256:    uploadResp.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(data))),
		},
	}

	sendResp, err := d.client.SendMessage(ctx, jid, msg)
	if err != nil {
		p.emitEvent(d, Event{Type: EventMessageFailed, Phone: phone, Error: err.Error(), At: time.Now().UTC()})
		return nil, fmt.Errorf("whatsapp: send media: %w", err)
	}

	p.emitEvent(d, Event{Type: EventMessageSent, Phone: phone, MessageID: sendResp.ID, At: time.Now().UTC()})
	return &SendResult{MessageID: sendResp.ID}, nil
}

// SetPresence sets the chat presence (typing indicator) for the given phone
// number, through this project's number.
func (p *Pool) SetPresence(ctx context.Context, phone, state string) error {
	d, err := p.current()
	if err != nil {
		return err
	}

	var presence types.ChatPresence
	switch state {
	case PresenceTyping:
		presence = types.ChatPresenceComposing
	case PresencePaused:
		presence = types.ChatPresencePaused
	default:
		return fmt.Errorf("whatsapp: invalid presence state %q, must be %q or %q", state, PresenceTyping, PresencePaused)
	}

	if err := d.client.SendChatPresence(ctx, toJID(phone), presence, types.ChatPresenceMediaText); err != nil {
		return fmt.Errorf("whatsapp: send presence: %w", err)
	}
	return nil
}

// GetChats returns this project's number's contacts.
func (p *Pool) GetChats(ctx context.Context) ([]Chat, error) {
	d, err := p.current()
	if err != nil {
		return nil, err
	}

	contacts, err := d.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: get contacts: %w", err)
	}
	chats := make([]Chat, 0, len(contacts))
	for jid, contact := range contacts {
		chats = append(chats, Chat{JID: jid.String(), Name: contact.FullName})
	}
	return chats, nil
}

// IsConnected reports whether this project's number is currently connected.
func (p *Pool) IsConnected() bool {
	p.mu.RLock()
	d := p.device
	p.mu.RUnlock()
	return d != nil && d.isConnected()
}

// Number describes the WhatsApp number in a project's pool. There is at
// most one.
type Number struct {
	JID       string `json:"jid"`
	Phone     string `json:"phone"`
	Connected bool   `json:"connected"`
}

// Numbers returns this project's number, or an empty (non-nil) slice if
// none is paired yet.
func (p *Pool) Numbers() []Number {
	p.mu.RLock()
	d := p.device
	p.mu.RUnlock()

	out := make([]Number, 0, 1)
	if d == nil {
		return out
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append(out, Number{JID: d.jid.String(), Phone: d.phone, Connected: d.connected})
}

// Events returns the pool's event channel.
func (p *Pool) Events() <-chan Event {
	return p.events
}

func (p *Pool) emitEvent(d *device, evt Event) {
	evt.From = d.jid.String()
	select {
	case p.events <- evt:
	default:
		p.logger.Warn("whatsapp event channel full, dropping event", "type", evt.Type, "from", evt.From)
	}
}

// Disconnect cleanly disconnects this project's number (if any) and closes
// the shared session store.
func (p *Pool) Disconnect() {
	p.mu.RLock()
	d := p.device
	p.mu.RUnlock()

	if d != nil {
		d.client.Disconnect()
		d.mu.Lock()
		d.connected = false
		d.mu.Unlock()
	}

	if p.container != nil {
		p.container.Close()
	}
	p.logger.Info("whatsapp pool disconnected")
}

func (p *Pool) handleEvent(d *device, rawEvt any) {
	switch evt := rawEvt.(type) {
	case *events.Receipt:
		p.handleReceipt(d, evt)
	case *events.Disconnected:
		d.mu.Lock()
		d.connected = false
		d.mu.Unlock()

		p.emitEvent(d, Event{
			Type: EventSessionDisconnect,
			At:   time.Now().UTC(),
		})
		p.logger.Warn("whatsapp device disconnected, will attempt reconnect", "phone", d.phone)
		go p.reconnectWithBackoff(d)
	case *events.Connected:
		d.mu.Lock()
		d.connected = true
		d.mu.Unlock()
		p.emitEvent(d, Event{
			Type: EventSessionReconnect,
			At:   time.Now().UTC(),
		})
		p.logger.Info("whatsapp device reconnected", "phone", d.phone)
	case *events.Message:
		if p.ignoreGroups && evt.Info.IsGroup {
			return
		}
		if p.ignoreStatus && evt.Info.Chat == types.StatusBroadcastJID {
			return
		}

		var text string
		if evt.Message.GetConversation() != "" {
			text = evt.Message.GetConversation()
		} else if evt.Message.GetExtendedTextMessage() != nil {
			text = evt.Message.GetExtendedTextMessage().GetText()
		}

		data := map[string]interface{}{
			"text":     text,
			"pushName": evt.Info.PushName,
			"sender":   evt.Info.Sender.String(),
		}

		p.emitEvent(d, Event{
			Type:      EventMessageReceived,
			Phone:     evt.Info.Sender.User,
			MessageID: evt.Info.ID,
			At:        evt.Info.Timestamp,
			Data:      data,
		})
	}
}

func (p *Pool) handleReceipt(d *device, evt *events.Receipt) {
	if len(evt.MessageIDs) == 0 {
		return
	}
	msgID := evt.MessageIDs[0]

	var evtType string
	switch evt.Type {
	case types.ReceiptTypeDelivered:
		evtType = EventMessageDelivered
	case types.ReceiptTypeRead:
		evtType = EventMessageRead
	default:
		return
	}

	p.emitEvent(d, Event{
		Type:      evtType,
		MessageID: msgID,
		At:        time.Now().UTC(),
	})
}

func (p *Pool) reconnectWithBackoff(d *device) {
	for i, delay := range p.backoff {
		time.Sleep(time.Duration(delay) * time.Second)
		p.logger.Info("attempting device reconnect", "phone", d.phone, "attempt", i+1, "delay_s", delay)

		if err := d.client.Connect(); err != nil {
			p.logger.Error("device reconnect failed", "phone", d.phone, "attempt", i+1, "error", err)
			continue
		}

		d.mu.Lock()
		d.connected = true
		d.mu.Unlock()
		p.logger.Info("device reconnected successfully", "phone", d.phone, "attempt", i+1)
		return
	}
	p.logger.Error("all device reconnect attempts exhausted", "phone", d.phone)
}
