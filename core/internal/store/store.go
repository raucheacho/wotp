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
	KeyHash   string    `json:"-"`
	KeyPrefix string    `json:"key_prefix"`
	Tier      string    `json:"tier"` // "anon" or "service"
	CreatedAt time.Time `json:"created_at"`
}

// Store is the data access interface for wotp-core.
// Implementations must be safe for concurrent use.
type Store interface {
	// OTP operations
	CreateOTPRequest(ctx context.Context, req *OTPRequest) error
	GetOTPRequestByToken(ctx context.Context, token string) (*OTPRequest, error)
	UpdateOTPStatus(ctx context.Context, token string, status OTPStatus) error
	UpdateOTPMessageID(ctx context.Context, token string, messageID string) error
	IncrementAttempts(ctx context.Context, token string) (int, error)
	CountRecentOTPs(ctx context.Context, phone string, since time.Time) (int, error)
	GetRecentOTPs(ctx context.Context, limit int) ([]OTPRequest, error)
	ExpireStaleOTPs(ctx context.Context, now time.Time) (int64, error)

	// API key operations
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error)
	ListAPIKeys(ctx context.Context) ([]APIKey, error)
	DeleteAPIKeysByTier(ctx context.Context, tier string) error

	// Lifecycle
	Close() error
}
