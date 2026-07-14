package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// MeowClient is the whatsmeow-based production WhatsApp client.
type MeowClient struct {
	client      *whatsmeow.Client
	container   *sqlstore.Container
	logger      *slog.Logger
	events      chan Event
	phoneNumber string
	deviceName  string
	backoff        []int
	simulateTyping bool
	mu             sync.RWMutex
	connected      bool
	reconnecting   bool
}

// MeowConfig holds configuration for the whatsmeow client.
type MeowConfig struct {
	DBPath         string
	DeviceName     string
	Backoff        []int
	Logger         *slog.Logger
	SimulateTyping bool
}

// NewMeowClient creates a new whatsmeow-based WhatsApp client.
func NewMeowClient(cfg MeowConfig) (*MeowClient, error) {
	dbLog := waLog.Noop

	container, err := sqlstore.New(context.Background(), "sqlite3",
		fmt.Sprintf("file:%s?_foreign_keys=on", cfg.DBPath),
		dbLog,
	)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: open session store: %w", err)
	}

	return &MeowClient{
		container:  container,
		logger:     cfg.Logger,
		events:         make(chan Event, 256),
		deviceName:     cfg.DeviceName,
		backoff:        cfg.Backoff,
		simulateTyping: cfg.SimulateTyping,
	}, nil
}

// Connect initiates the WhatsApp connection and returns a QR channel if pairing is needed.
func (m *MeowClient) Connect(ctx context.Context) (<-chan string, error) {
	deviceStore, err := m.container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: get device: %w", err)
	}

	m.client = whatsmeow.NewClient(deviceStore, waLog.Noop)
	m.client.AddEventHandler(m.handleEvent)

	if m.client.Store.ID == nil {
		// New device, need QR pairing
		qrChan, _ := m.client.GetQRChannel(ctx)
		if err := m.client.Connect(); err != nil {
			return nil, fmt.Errorf("whatsapp: connect for QR: %w", err)
		}

		// Convert whatsmeow QR events to simple string channel
		out := make(chan string, 8)
		go func() {
			defer close(out)
			for evt := range qrChan {
				if evt.Event == "code" {
					select {
					case out <- evt.Code:
					case <-ctx.Done():
						return
					}
				} else if evt.Event == "success" {
					m.mu.Lock()
					m.connected = true
					if m.client.Store.ID != nil {
						m.phoneNumber = m.client.Store.ID.User
					}
					m.mu.Unlock()
					m.logger.Info("whatsapp connected via QR", "phone", m.phoneNumber)
					return
				} else if evt.Event == "timeout" {
					m.logger.Warn("QR code timed out")
					return
				}
			}
		}()
		return out, nil
	}

	// Already paired, just connect
	if err := m.client.Connect(); err != nil {
		return nil, fmt.Errorf("whatsapp: connect: %w", err)
	}

	m.mu.Lock()
	m.connected = true
	m.phoneNumber = m.client.Store.ID.User
	m.mu.Unlock()

	m.logger.Info("whatsapp connected (existing session)", "phone", m.phoneNumber)
	return nil, nil
}

// SendMessage sends a text message to the given phone number via WhatsApp.
func (m *MeowClient) SendMessage(ctx context.Context, phone, message string) (*SendResult, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("whatsapp: not connected")
	}

	// Strip non-digit characters for WhatsApp JID
	var cleanPhone string
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			cleanPhone += string(r)
		}
	}
	jid := types.NewJID(cleanPhone, types.DefaultUserServer)

	if m.simulateTyping {
		_ = m.client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		time.Sleep(2 * time.Second)
		_ = m.client.SendChatPresence(ctx, jid, types.ChatPresencePaused, types.ChatPresenceMediaText)
	}

	resp, err := m.client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(message),
	})
	if err != nil {
		m.emitEvent(Event{
			Type:  EventMessageFailed,
			Phone: phone,
			Error: err.Error(),
			At:    time.Now().UTC(),
		})
		return nil, fmt.Errorf("whatsapp: send message: %w", err)
	}

	msgID := resp.ID
	m.emitEvent(Event{
		Type:      EventMessageSent,
		Phone:     phone,
		MessageID: msgID,
		At:        time.Now().UTC(),
	})

	return &SendResult{MessageID: msgID}, nil
}

// IsConnected returns whether the client is currently connected.
func (m *MeowClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// GetPhoneNumber returns the connected phone number.
func (m *MeowClient) GetPhoneNumber() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.phoneNumber
}

// Disconnect cleanly disconnects from WhatsApp.
func (m *MeowClient) Disconnect() {
	if m.client != nil {
		m.client.Disconnect()
	}
	if m.container != nil {
		m.container.Close()
	}
	m.mu.Lock()
	m.connected = false
	m.mu.Unlock()
	m.logger.Info("whatsapp disconnected")
}

// Events returns the event channel.
func (m *MeowClient) Events() <-chan Event {
	return m.events
}

func (m *MeowClient) emitEvent(evt Event) {
	select {
	case m.events <- evt:
	default:
		m.logger.Warn("whatsapp event channel full, dropping event", "type", evt.Type)
	}
}

func (m *MeowClient) handleEvent(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.Receipt:
		m.handleReceipt(evt)
	case *events.Disconnected:
		m.mu.Lock()
		m.connected = false
		shouldReconnect := !m.reconnecting
		if shouldReconnect {
			m.reconnecting = true
		}
		m.mu.Unlock()
		
		m.emitEvent(Event{
			Type: EventSessionDisconnect,
			At:   time.Now().UTC(),
		})
		
		if shouldReconnect {
			m.logger.Warn("whatsapp disconnected, will attempt reconnect")
			go m.reconnectWithBackoff()
		}
	case *events.Connected:
		m.mu.Lock()
		m.connected = true
		m.mu.Unlock()
		m.emitEvent(Event{
			Type: EventSessionReconnect,
			At:   time.Now().UTC(),
		})
		m.logger.Info("whatsapp reconnected")
	case *events.Message:
		// Only capture incoming text messages for now or all messages.
		var text string
		if evt.Message.GetConversation() != "" {
			text = evt.Message.GetConversation()
		} else if evt.Message.GetExtendedTextMessage() != nil {
			text = evt.Message.GetExtendedTextMessage().GetText()
		}
		
		data := map[string]interface{}{
			"text": text,
			"pushName": evt.Info.PushName,
			"sender": evt.Info.Sender.String(),
		}

		m.emitEvent(Event{
			Type:      EventMessageReceived,
			Phone:     evt.Info.Sender.User,
			MessageID: evt.Info.ID,
			At:        evt.Info.Timestamp,
			Data:      data,
		})
	}
}

func (m *MeowClient) handleReceipt(evt *events.Receipt) {
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

	m.emitEvent(Event{
		Type:      evtType,
		MessageID: msgID,
		At:        time.Now().UTC(),
	})
}

func (m *MeowClient) reconnectWithBackoff() {
	defer func() {
		m.mu.Lock()
		m.reconnecting = false
		m.mu.Unlock()
	}()

	for i, delay := range m.backoff {
		time.Sleep(time.Duration(delay) * time.Second)
		m.logger.Info("attempting reconnect", "attempt", i+1, "delay_s", delay)

		if err := m.client.Connect(); err != nil {
			m.logger.Error("reconnect failed", "attempt", i+1, "error", err)
			continue
		}

		m.mu.Lock()
		m.connected = true
		m.mu.Unlock()
		m.logger.Info("reconnected successfully", "attempt", i+1)
		return
	}

	m.logger.Error("all reconnect attempts exhausted")
}
