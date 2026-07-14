// Package otp implements OTP code generation, hashing, verification,
// token generation, and rate limiting for the wotp-core engine.
package otp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/wotp/core/internal/store"
)

// Predefined errors for the OTP engine.
var (
	ErrRateLimited      = errors.New("otp: rate limit exceeded for this phone number")
	ErrTokenNotFound     = errors.New("otp: token not found")
	ErrTokenExpired      = errors.New("otp: token has expired")
	ErrMaxAttempts       = errors.New("otp: maximum verification attempts exceeded")
	ErrAlreadyVerified   = errors.New("otp: token already verified")
	ErrInvalidCode       = errors.New("otp: invalid verification code")
)

// SendResult is returned after a successful OTP send.
type SendResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Code      string    `json:"-"` // plaintext code, only used internally to send the message
}

// VerifyResult is returned after a verification attempt.
type VerifyResult struct {
	Verified          bool   `json:"verified"`
	Phone             string `json:"phone,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	AttemptsRemaining int    `json:"attempts_remaining,omitempty"`
}

// EngineConfig holds the runtime parameters for the OTP engine.
type EngineConfig struct {
	CodeLength             int
	ExpiryMinutes          int
	MaxAttempts            int
	RateLimitPerPhonePerHr int
}

// Engine orchestrates OTP generation, storage, and verification.
type Engine struct {
	store  store.Store
	config EngineConfig
}

// NewEngine creates a new OTP engine with the given store and config.
func NewEngine(s store.Store, cfg EngineConfig) *Engine {
	return &Engine{
		store:  s,
		config: cfg,
	}
}

// Store returns the underlying datastore.
func (e *Engine) Store() store.Store {
	return e.store
}

// GenerateCode produces a cryptographically random numeric code of the configured length.
func GenerateCode(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", fmt.Errorf("otp: code length must be between 4 and 10, got %d", length)
	}

	// Calculate the upper bound (10^length)
	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("otp: generate random code: %w", err)
	}

	// Pad with leading zeros to ensure consistent length
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n), nil
}

// HashCode produces a bcrypt hash of the given OTP code.
func HashCode(code string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("otp: hash code: %w", err)
	}
	return string(hash), nil
}

// VerifyCodeHash checks a plaintext code against a bcrypt hash.
func VerifyCodeHash(code, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
}

// GenerateToken creates an opaque token with the otp_tok_ prefix.
func GenerateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "otp_tok_" + hex.EncodeToString(b)
}

// Send generates a new OTP, stores it, and returns the token and code.
// The caller is responsible for actually sending the code via WhatsApp.
func (e *Engine) Send(ctx context.Context, phone string) (*SendResult, error) {
	// Rate limiting: check how many OTPs were sent in the last hour
	since := time.Now().Add(-1 * time.Hour)
	count, err := e.store.CountRecentOTPs(ctx, phone, since)
	if err != nil {
		return nil, fmt.Errorf("otp: check rate limit: %w", err)
	}
	if count >= e.config.RateLimitPerPhonePerHr {
		return nil, ErrRateLimited
	}

	// Generate code and hash it
	code, err := GenerateCode(e.config.CodeLength)
	if err != nil {
		return nil, err
	}
	codeHash, err := HashCode(code)
	if err != nil {
		return nil, err
	}

	token := GenerateToken()
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(e.config.ExpiryMinutes) * time.Minute)

	req := &store.OTPRequest{
		ID:        uuid.New().String(),
		Token:     token,
		Phone:     phone,
		CodeHash:  codeHash,
		Status:    store.StatusPending,
		Attempts:  0,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := e.store.CreateOTPRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("otp: store request: %w", err)
	}

	return &SendResult{
		Token:     token,
		ExpiresAt: expiresAt,
		Code:      code,
	}, nil
}

// Verify checks the provided code against the stored hash for the given token.
func (e *Engine) Verify(ctx context.Context, token, code string) (*VerifyResult, error) {
	req, err := e.store.GetOTPRequestByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("otp: lookup token: %w", err)
	}
	if req == nil {
		return nil, ErrTokenNotFound
	}

	// Check if already verified
	if req.Status == store.StatusVerified {
		return nil, ErrAlreadyVerified
	}

	// Check expiry
	if time.Now().After(req.ExpiresAt) {
		_ = e.store.UpdateOTPStatus(ctx, token, store.StatusExpired)
		return nil, ErrTokenExpired
	}

	// Check max attempts
	if req.Attempts >= e.config.MaxAttempts {
		return nil, ErrMaxAttempts
	}

	// Increment attempt counter
	attempts, err := e.store.IncrementAttempts(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("otp: increment attempts: %w", err)
	}
	remaining := e.config.MaxAttempts - attempts

	// Verify the code
	if !VerifyCodeHash(code, req.CodeHash) {
		return &VerifyResult{
			Verified:          false,
			AttemptsRemaining: remaining,
		}, ErrInvalidCode
	}

	// Mark as verified
	if err := e.store.UpdateOTPStatus(ctx, token, store.StatusVerified); err != nil {
		return nil, fmt.Errorf("otp: mark verified: %w", err)
	}

	return &VerifyResult{
		Verified: true,
		Phone:    req.Phone,
	}, nil
}

// MarkSent updates the OTP status to sent and stores the WhatsApp message ID.
func (e *Engine) MarkSent(ctx context.Context, token, messageID string) error {
	if err := e.store.UpdateOTPMessageID(ctx, token, messageID); err != nil {
		return err
	}
	return e.store.UpdateOTPStatus(ctx, token, store.StatusSent)
}

// UpdateDeliveryStatus updates the OTP status based on WhatsApp delivery receipts.
func (e *Engine) UpdateDeliveryStatus(ctx context.Context, token string, status store.OTPStatus) error {
	return e.store.UpdateOTPStatus(ctx, token, status)
}
