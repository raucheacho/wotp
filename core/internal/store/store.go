// Package store defines the data access interface for wotp-core
// and provides a SQLite implementation.
package store

import (
	"context"
	"time"
)

// OTPStatus represents the lifecycle state of an OTP request.
type OTPStatus string

const (
	StatusPending   OTPStatus = "pending"
	StatusSent      OTPStatus = "sent"
	StatusDelivered OTPStatus = "delivered"
	StatusRead      OTPStatus = "read"
	StatusVerified  OTPStatus = "verified"
	StatusExpired   OTPStatus = "expired"
	StatusFailed    OTPStatus = "failed"
)

// OTPRequest represents a single OTP verification request stored in the database.
type OTPRequest struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Phone     string    `json:"phone"`
	CodeHash  string    `json:"-"`
	Status    OTPStatus `json:"status"`
	MessageID string    `json:"message_id,omitempty"`
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// APIKey represents a stored API key.
type APIKey struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id,omitempty"` // set once callers move to ControlStore (see control.go)
	KeyHash   string    `json:"-"`
	KeyPrefix string    `json:"key_prefix"`
	Tier      string    `json:"tier"` // "anon" or "service"
	CreatedAt time.Time `json:"created_at"`
}

// ProjectStore is the data access interface for a single project's data plane
// (otps, generic messages, webhook logs). Every project on a wotp-core
// instance gets its own ProjectStore, backed by its own SQLite file — see
// ControlStore (control.go) for the shared, instance-wide data (projects,
// api_keys, numbers).
// Implementations must be safe for concurrent use.
type ProjectStore interface {
	// OTP operations
	CreateOTPRequest(ctx context.Context, req *OTPRequest) error
	GetOTPRequestByToken(ctx context.Context, token string) (*OTPRequest, error)
	UpdateOTPStatus(ctx context.Context, token string, status OTPStatus) error
	UpdateOTPStatusByMessageID(ctx context.Context, messageID string, status OTPStatus) error
	UpdateOTPMessageID(ctx context.Context, token string, messageID string) error
	IncrementAttempts(ctx context.Context, token string) (int, error)
	CountRecentOTPs(ctx context.Context, phone string, since time.Time) (int, error)
	GetRecentOTPs(ctx context.Context, limit int) ([]OTPRequest, error)
	ExpireStaleOTPs(ctx context.Context, now time.Time) (int64, error)

	// Generic Messages
	SaveGenericMessage(ctx context.Context, msg *GenericMessage) error
	UpdateGenericMessageStatus(ctx context.Context, id string, status string, errStr string) error
	GetGenericMessages(ctx context.Context, limit int) ([]GenericMessage, error)

	// Webhooks
	SaveWebhookLog(ctx context.Context, log *WebhookLog) error
	GetWebhookLogs(ctx context.Context, limit int) ([]WebhookLog, error)

	// Lifecycle
	Close() error
}

// GenericMessage represents a generic WhatsApp message stored in the database.
type GenericMessage struct {
	ID          string    `json:"id"`
	Phone       string    `json:"phone"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookLog represents a dispatched webhook stored in the database.
type WebhookLog struct {
	ID         string    `json:"id"`
	EventType  string    `json:"event_type"`
	Payload    string    `json:"payload"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

