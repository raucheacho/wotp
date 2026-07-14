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
}

// Event types matching spec §6.3.
const (
	EventMessageSent       = "message.sent"
	EventMessageDelivered  = "message.delivered"
	EventMessageRead       = "message.read"
	EventMessageFailed     = "message.failed"
	EventSessionDisconnect = "session.disconnected"
	EventSessionReconnect  = "session.reconnected"
)

// SendResult holds the result of sending a WhatsApp message.
type SendResult struct {
	MessageID string
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

	// IsConnected returns true if the client is currently connected to WhatsApp.
	IsConnected() bool

	// GetPhoneNumber returns the connected phone number, or empty string if not connected.
	GetPhoneNumber() string

	// Disconnect cleanly disconnects from WhatsApp.
	Disconnect()

	// Events returns a read-only channel of WhatsApp events.
	Events() <-chan Event
}
