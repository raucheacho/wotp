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
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Without this check, an error page (often HTML — a 403 from a
			// host that gates on User-Agent, an expired link, ...) gets
			// silently uploaded and "sent" as if it were the real file:
			// WhatsApp accepts the upload (it's just bytes) and the send
			// call returns a message id, but the recipient never sees a
			// working image/video/audio/document — a send that looks
			// successful end to end while actually delivering garbage.
			return nil, fmt.Errorf("whatsapp: fetch media url: status %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("whatsapp: read media: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("whatsapp: neither url nor base64Data provided")
}

// SendLocation sends a location pin through this project's number.
// opts.Name/Address are optional — a bare pin with no label is valid.
func (p *Pool) SendLocation(ctx context.Context, phone string, opts LocationSendOptions) (*SendResult, error) {
	d, err := p.current()
	if err != nil {
		return nil, err
	}
	jid := toJID(phone)

	loc := &waE2E.LocationMessage{
		DegreesLatitude:  proto.Float64(opts.Latitude),
		DegreesLongitude: proto.Float64(opts.Longitude),
	}
	if opts.Name != "" {
		loc.Name = proto.String(opts.Name)
	}
	if opts.Address != "" {
		loc.Address = proto.String(opts.Address)
	}

	resp, err := d.client.SendMessage(ctx, jid, &waE2E.Message{LocationMessage: loc})
	if err != nil {
		p.emitEvent(d, Event{Type: EventMessageFailed, Phone: phone, Error: err.Error(), At: time.Now().UTC()})
		return nil, fmt.Errorf("whatsapp: send location: %w", err)
	}

	p.emitEvent(d, Event{Type: EventMessageSent, Phone: phone, MessageID: resp.ID, At: time.Now().UTC()})
	return &SendResult{MessageID: resp.ID}, nil
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

// SendMedia sends a media message through this project's number. Kind
// selects the whatsmeow upload type and waE2E.Message field — see
// mediaMessageForKind.
func (p *Pool) SendMedia(ctx context.Context, phone string, opts MediaSendOptions) (*SendResult, error) {
	d, err := p.current()
	if err != nil {
		return nil, err
	}
	data, err := fetchMediaData(ctx, opts.URL, opts.Base64Data)
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

	mediaType, buildMessage := mediaKindHandlers(opts.Kind)
	uploadResp, err := d.client.Upload(ctx, data, mediaType)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: upload media: %w", err)
	}
	msg := buildMessage(uploadResp, contentType, uint64(len(data)), opts.Caption, opts.Filename)

	sendResp, err := d.client.SendMessage(ctx, jid, msg)
	if err != nil {
		p.emitEvent(d, Event{Type: EventMessageFailed, Phone: phone, Error: err.Error(), At: time.Now().UTC()})
		return nil, fmt.Errorf("whatsapp: send media: %w", err)
	}

	p.emitEvent(d, Event{Type: EventMessageSent, Phone: phone, MessageID: sendResp.ID, At: time.Now().UTC()})
	return &SendResult{MessageID: sendResp.ID}, nil
}

// mediaKindHandlers returns the whatsmeow upload MediaType and a builder
// for the matching waE2E.Message field for kind, defaulting to image for
// an empty/unrecognized kind (mirrors the "media" legacy alias in
// api.handleMessagesSend). All four waE2E media message types carry the
// same upload-response fields (URL, Mimetype, FileSHA256, FileLength,
// extractInboundMedia detects whether msg carries an image/video/audio/
// document attachment and, if so, downloads+decrypts it via client.
// whatsmeow keeps the decryption keys (MediaKey/FileEncSHA256/...) only in
// this in-memory message — there's no way to re-fetch them once handleEvent
// returns — so the download has to happen synchronously right here, not
// lazily when a dev later asks for the file (see GET /v1/media/{id}).
// Returns ("", "", "", nil) for a message with no media attachment. On a
// download failure, still returns the caption/kind/mimetype with a nil
// data — fail open, same as the rest of this event path: the message still
// gets recorded, just without retrievable media.
func extractInboundMedia(ctx context.Context, client *whatsmeow.Client, msg *waE2E.Message) (caption, kind, mimeType string, data []byte) {
	var downloadable whatsmeow.DownloadableMessage
	switch {
	case msg.GetImageMessage() != nil:
		m := msg.GetImageMessage()
		caption, kind, mimeType, downloadable = m.GetCaption(), string(MediaKindImage), m.GetMimetype(), m
	case msg.GetVideoMessage() != nil:
		m := msg.GetVideoMessage()
		caption, kind, mimeType, downloadable = m.GetCaption(), string(MediaKindVideo), m.GetMimetype(), m
	case msg.GetAudioMessage() != nil:
		// No caption field on audio — WhatsApp's protocol carries none for
		// voice notes (see mediaKindHandlers' doc comment, same fact on
		// the send side).
		m := msg.GetAudioMessage()
		kind, mimeType, downloadable = string(MediaKindAudio), m.GetMimetype(), m
	case msg.GetDocumentMessage() != nil:
		m := msg.GetDocumentMessage()
		caption, kind, mimeType, downloadable = m.GetCaption(), string(MediaKindDocument), m.GetMimetype(), m
	default:
		return "", "", "", nil
	}

	bytes, err := client.Download(ctx, downloadable)
	if err != nil {
		return caption, kind, mimeType, nil
	}
	return caption, kind, mimeType, bytes
}

// MediaKey, FileEncSHA256, DirectPath) — only Caption/Filename
// availability differs: audio has no Caption field at all (WhatsApp's
// protocol carries none for voice notes), and only document has FileName.
func mediaKindHandlers(kind MediaKind) (whatsmeow.MediaType, func(resp whatsmeow.UploadResponse, mimetype string, length uint64, caption, filename string) *waE2E.Message) {
	switch kind {
	case MediaKindVideo:
		return whatsmeow.MediaVideo, func(r whatsmeow.UploadResponse, mimetype string, length uint64, caption, _ string) *waE2E.Message {
			return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
				Caption: proto.String(caption), Mimetype: proto.String(mimetype),
				URL: proto.String(r.URL), DirectPath: proto.String(r.DirectPath),
				MediaKey: r.MediaKey, FileEncSHA256: r.FileEncSHA256, FileSHA256: r.FileSHA256,
				FileLength: proto.Uint64(length),
			}}
		}
	case MediaKindAudio:
		return whatsmeow.MediaAudio, func(r whatsmeow.UploadResponse, mimetype string, length uint64, _, _ string) *waE2E.Message {
			return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
				Mimetype: proto.String(mimetype),
				URL:      proto.String(r.URL), DirectPath: proto.String(r.DirectPath),
				MediaKey: r.MediaKey, FileEncSHA256: r.FileEncSHA256, FileSHA256: r.FileSHA256,
				FileLength: proto.Uint64(length),
			}}
		}
	case MediaKindDocument:
		return whatsmeow.MediaDocument, func(r whatsmeow.UploadResponse, mimetype string, length uint64, caption, filename string) *waE2E.Message {
			return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
				Caption: proto.String(caption), FileName: proto.String(filename), Mimetype: proto.String(mimetype),
				URL: proto.String(r.URL), DirectPath: proto.String(r.DirectPath),
				MediaKey: r.MediaKey, FileEncSHA256: r.FileEncSHA256, FileSHA256: r.FileSHA256,
				FileLength: proto.Uint64(length),
			}}
		}
	default: // MediaKindImage and unrecognized/empty
		return whatsmeow.MediaImage, func(r whatsmeow.UploadResponse, mimetype string, length uint64, caption, _ string) *waE2E.Message {
			return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
				Caption: proto.String(caption), Mimetype: proto.String(mimetype),
				URL: proto.String(r.URL), DirectPath: proto.String(r.DirectPath),
				MediaKey: r.MediaKey, FileEncSHA256: r.FileEncSHA256, FileSHA256: r.FileSHA256,
				FileLength: proto.Uint64(length),
			}}
		}
	}
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
		// IsFromMe means this event is an echo of a message this WhatsApp
		// account sent — either via this API or from another linked
		// device/the phone itself (WhatsApp multi-device syncs every send
		// to every linked device, including this one). It is never a
		// genuine reply from the other party. Without this check, a human
		// operator replying manually from their own phone — or the bot's
		// own sent messages syncing back — would be misread as inbound
		// customer messages, at worst causing the bot to reply to its own
		// messages in a loop.
		if evt.Info.IsFromMe {
			return
		}
		if p.ignoreGroups && evt.Info.IsGroup {
			return
		}
		if p.ignoreStatus && evt.Info.Chat == types.StatusBroadcastJID {
			return
		}
		// Protocol messages (revokes, history-sync notifications, app-state
		// key shares, ephemeral-timer changes, ...) aren't real user
		// content — WhatsApp sends a burst of these right after a device
		// links, wrapped in the same *events.Message type. Emitting them as
		// EventMessageReceived would flood webhooks/conversations with
		// empty-text noise.
		if evt.Message.GetProtocolMessage() != nil {
			return
		}

		var text, mediaKind, mediaMimeType string
		var mediaBytes []byte
		if evt.Message.GetConversation() != "" {
			text = evt.Message.GetConversation()
		} else if evt.Message.GetExtendedTextMessage() != nil {
			text = evt.Message.GetExtendedTextMessage().GetText()
		} else if loc := evt.Message.GetLocationMessage(); loc != nil {
			text = FormatLocationText(loc.GetName(), loc.GetDegreesLatitude(), loc.GetDegreesLongitude())
		} else {
			text, mediaKind, mediaMimeType, mediaBytes = extractInboundMedia(context.Background(), d.client, evt.Message)
		}

		// WhatsApp increasingly addresses senders by LID (a privacy-preserving
		// numeric ID, JID server "lid") instead of their real phone number.
		// When that happens, Sender.User isn't a real MSISDN — SenderAlt
		// carries the actual phone-number JID, so prefer it whenever present.
		senderJID := evt.Info.Sender
		if senderJID.Server == types.HiddenUserServer && !evt.Info.SenderAlt.IsEmpty() {
			senderJID = evt.Info.SenderAlt
		}

		data := map[string]interface{}{
			"text":     text,
			"pushName": evt.Info.PushName,
			"sender":   senderJID.String(),
		}
		if mediaKind != "" {
			data["mediaKind"] = mediaKind
			data["mediaMimeType"] = mediaMimeType
			if mediaBytes != nil {
				data["mediaBytes"] = mediaBytes
			}
		}

		p.emitEvent(d, Event{
			Type:      EventMessageReceived,
			Phone:     senderJID.User,
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
