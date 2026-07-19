// Package whatsapp provides a WhatsApp client interface and whatsmeow-based
// production implementation for wotp-core. The interface allows testing
// without a real WhatsApp connection.
package whatsapp

import (
	"context"
	"time"
)

// Event represents a WhatsApp event emitted by the client.
type Event struct {
	Type      string    `json:"type"`
	Phone     string    `json:"phone,omitempty"`
	MessageID string    `json:"message_id,omitempty"`
	Error     string    `json:"error,omitempty"`
	At        time.Time `json:"at"`
	Data      any       `json:"data,omitempty"`
	// From is the JID of the number that produced this event. Only set by
	// Pool, which fans multiple devices' events into one shared channel —
	// a single-device MeowClient leaves it empty since there's only ever
	// one possible source.
	From string `json:"from,omitempty"`
}

// Event types matching spec §6.3.
const (
	EventMessageSent       = "message.sent"
	EventMessageDelivered  = "message.delivered"
	EventMessageRead       = "message.read"
	EventMessageFailed     = "message.failed"
	EventMessageReceived   = "message.received"
	EventSessionDisconnect = "session.disconnected"
	EventSessionReconnect  = "session.reconnected"
)

// Presence states accepted by Client.SetPresence.
const (
	PresenceTyping = "typing"
	PresencePaused = "paused"
)

// SendResult holds the result of sending a WhatsApp message.
type SendResult struct {
	MessageID string
}

// Chat holds basic chat information.
type Chat struct {
	JID  string `json:"jid"`
	Name string `json:"name"`
}

// Client is the interface for WhatsApp operations.
// Implementations must be safe for concurrent use.
type Client interface {
	// Connect initiates the WhatsApp connection.
	// Returns a channel that receives QR code strings for pairing,
	// or nil if already authenticated.
	Connect(ctx context.Context) (<-chan string, error)

	// SendMessage sends a text message to the given phone number.
	SendMessage(ctx context.Context, phone, message string) (*SendResult, error)

	// SendMedia sends a media message to the given phone number.
	SendMedia(ctx context.Context, phone, url, base64Data, caption string) (*SendResult, error)

	// SetPresence sets the chat presence (typing indicator) for the given phone
	// number without sending a message. state must be "typing" or "paused".
	SetPresence(ctx context.Context, phone, state string) error

	// GetChats returns a list of existing chats.
	GetChats(ctx context.Context) ([]Chat, error)

	// IsConnected returns true if the client is currently connected to WhatsApp.
	IsConnected() bool

	// GetPhoneNumber returns the connected phone number, or empty string if not connected.
	GetPhoneNumber() string

	// Disconnect cleanly disconnects from WhatsApp.
	Disconnect()

	// Events returns a read-only channel of WhatsApp events.
	Events() <-chan Event
}

// TemplateOTPSender is implemented by backends that must send OTP codes
// through a pre-approved message template rather than free text. The Meta
// Cloud API requires this: an OTP is structurally always the first message
// in a conversation, so it's always outside the 24-hour customer-service
// window free-form text needs — there's no way to send a plain-text OTP on
// that backend. whatsmeow-backed clients don't implement this; they send
// whatever text the OTP template renders to via plain SendMessage.
type TemplateOTPSender interface {
	SendOTPTemplate(ctx context.Context, phone, code string, expiryMinutes int) (*SendResult, error)
}
