// Package whatsapp provides a WhatsApp client interface and whatsmeow-based
// production implementation for wotp-core. The interface allows testing
// without a real WhatsApp connection.
package whatsapp

import (
	"context"
	"fmt"
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

// MediaKind identifies the WhatsApp message type for a media send — it
// determines both which whatsmeow upload/message type is used and which
// Cloud API "type" value is sent, so both backends accept and render the
// same request the same way. "image" is also what a bare/legacy "media"
// request type maps to (see api.handleMessagesSend's backward-compat
// alias), so callers written before Kind existed keep working unchanged.
type MediaKind string

const (
	MediaKindImage    MediaKind = "image"
	MediaKindVideo    MediaKind = "video"
	MediaKindAudio    MediaKind = "audio"
	MediaKindDocument MediaKind = "document"
)

// MediaSendOptions bundles a media send's parameters. A struct instead of
// further positional args — Kind/Filename are only meaningful for some
// kinds (Filename: document-only; Caption: every kind except audio, which
// WhatsApp's protocol carries no caption field for at all), and a send
// call already had four positional string params before this; a fifth or
// sixth invites exactly the swapped-argument mistakes named args exist to
// prevent.
type MediaSendOptions struct {
	URL        string
	Base64Data string
	Caption    string
	Kind       MediaKind
	// Filename is shown as the file name in the recipient's chat.
	// Document-only — ignored for every other kind.
	Filename string
}

// LocationSendOptions bundles a location send's parameters — a struct
// rather than two adjacent float64 positional args, which is exactly the
// shape most likely to get silently swapped at a call site (see
// MediaSendOptions' doc comment for the same reasoning).
type LocationSendOptions struct {
	Latitude  float64
	Longitude float64
	// Name/Address are optional — a bare pin with no label is a valid send.
	Name    string
	Address string
}

// FormatLocationText renders a received location as human-readable text —
// the place name if the sender's app included one, otherwise the raw
// coordinates. wotp has no dedicated lat/long storage for inbound messages
// (InboundMessage.Content is a plain string, same as an outbound location's
// stored Content — see api.handleMessagesSend) — this is the one place
// that plain-string content gets produced for an *inbound* location, shared
// across whatsmeow (pool.go, meow.go) and Cloud (cloud_webhook.go) so both
// backends render a received pin identically.
func FormatLocationText(name string, latitude, longitude float64) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("%.6f, %.6f", latitude, longitude)
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
	SendMedia(ctx context.Context, phone string, opts MediaSendOptions) (*SendResult, error)

	// SendLocation sends a location pin to the given phone number.
	SendLocation(ctx context.Context, phone string, opts LocationSendOptions) (*SendResult, error)

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
