package otp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wotp/core/internal/store"
)

// --- In-memory store for testing ---

type memStore struct {
	otps map[string]*store.OTPRequest
}

func newMemStore() *memStore {
	return &memStore{
		otps: make(map[string]*store.OTPRequest),
	}
}

func (m *memStore) CreateOTPRequest(_ context.Context, req *store.OTPRequest) error {
	m.otps[req.Token] = req
	return nil
}

func (m *memStore) GetOTPRequestByToken(_ context.Context, token string) (*store.OTPRequest, error) {
	r, ok := m.otps[token]
	if !ok {
		return nil, nil
	}
	// Return a copy to avoid mutation issues
	cp := *r
	return &cp, nil
}

func (m *memStore) UpdateOTPStatus(_ context.Context, token string, status store.OTPStatus) error {
	if r, ok := m.otps[token]; ok {
		r.Status = status
	}
	return nil
}

func (m *memStore) UpdateOTPMessageID(_ context.Context, token string, messageID string) error {
	if r, ok := m.otps[token]; ok {
		r.MessageID = messageID
	}
	return nil
}

func (m *memStore) IncrementAttempts(_ context.Context, token string) (int, error) {
	if r, ok := m.otps[token]; ok {
		r.Attempts++
		return r.Attempts, nil
	}
	return 0, nil
}

func (m *memStore) CountRecentOTPs(_ context.Context, phone string, since time.Time) (int, error) {
	count := 0
	for _, r := range m.otps {
		if r.Phone == phone && r.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

func (m *memStore) ExpireStaleOTPs(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (m *memStore) GetRecentOTPs(_ context.Context, limit int) ([]store.OTPRequest, error) {
	var out []store.OTPRequest
	for _, r := range m.otps {
		out = append(out, *r)
	}
	return out, nil
}

func (m *memStore) SaveGenericMessage(_ context.Context, msg *store.GenericMessage) error { return nil }
func (m *memStore) UpdateGenericMessageStatus(_ context.Context, messageID string, status string, errorMsg string) error { return nil }
func (m *memStore) GetGenericMessages(_ context.Context, limit int) ([]store.GenericMessage, error) { return nil, nil }
func (m *memStore) GetGenericMessageByID(_ context.Context, messageID string) (*store.GenericMessage, error) { return nil, nil }
func (m *memStore) SaveWebhookLog(_ context.Context, log *store.WebhookLog) error { return nil }
func (m *memStore) GetWebhookLogs(_ context.Context, limit int) ([]store.WebhookLog, error) { return nil, nil }
func (m *memStore) UpdateOTPStatusByMessageID(_ context.Context, messageID string, status store.OTPStatus) error { return nil }

func (m *memStore) Close() error { return nil }

// --- Tests ---

func TestGenerateCode(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"6 digits", 6, false},
		{"4 digits", 4, false},
		{"10 digits", 10, false},
		{"too short", 3, true},
		{"too long", 11, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := GenerateCode(tt.length)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got code=%q", code)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(code) != tt.length {
				t.Errorf("expected length %d, got %d (%q)", tt.length, len(code), code)
			}
			// Ensure all chars are digits
			for _, c := range code {
				if c < '0' || c > '9' {
					t.Errorf("non-digit character %q in code %q", c, code)
				}
			}
		})
	}
}

func TestGenerateCodeUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := GenerateCode(6)
		if err != nil {
			t.Fatal(err)
		}
		seen[code] = true
	}
	// Statistically, 100 random 6-digit codes should produce at least 90 unique codes
	if len(seen) < 90 {
		t.Errorf("expected at least 90 unique codes from 100 generations, got %d", len(seen))
	}
}

func TestHashAndVerify(t *testing.T) {
	code := "483920"
	hash, err := HashCode(code)
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	if !VerifyCodeHash(code, hash) {
		t.Error("expected code to match its hash")
	}
	if VerifyCodeHash("000000", hash) {
		t.Error("expected wrong code to not match hash")
	}
}

func TestGenerateToken(t *testing.T) {
	token := GenerateToken()
	if !strings.HasPrefix(token, "otp_tok_") {
		t.Errorf("token %q does not have otp_tok_ prefix", token)
	}
	if len(token) != len("otp_tok_")+32 { // 16 bytes = 32 hex chars
		t.Errorf("unexpected token length: %d", len(token))
	}

	// Uniqueness
	token2 := GenerateToken()
	if token == token2 {
		t.Error("two generated tokens should not be identical")
	}
}

func TestEngineSendAndVerify(t *testing.T) {
	ms := newMemStore()
	engine := NewEngine(ms, EngineConfig{
		CodeLength:             6,
		ExpiryMinutes:          5,
		MaxAttempts:            3,
		RateLimitPerPhonePerHr: 10,
	})
	ctx := context.Background()

	// Send OTP
	result, err := engine.Send(ctx, "+212600000000")
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}
	if !strings.HasPrefix(result.Token, "otp_tok_") {
		t.Errorf("token should have otp_tok_ prefix: %q", result.Token)
	}
	if len(result.Code) != 6 {
		t.Errorf("expected 6-digit code, got %q", result.Code)
	}

	// Verify with wrong code
	vr, err := engine.Verify(ctx, result.Token, "000000")
	if err != ErrInvalidCode {
		t.Fatalf("expected ErrInvalidCode, got: %v", err)
	}
	if vr.Verified {
		t.Error("expected verified=false for wrong code")
	}
	if vr.AttemptsRemaining != 2 {
		t.Errorf("expected 2 attempts remaining, got %d", vr.AttemptsRemaining)
	}

	// Verify with correct code
	vr, err = engine.Verify(ctx, result.Token, result.Code)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !vr.Verified {
		t.Error("expected verified=true")
	}
	if vr.Phone != "+212600000000" {
		t.Errorf("expected phone +212600000000, got %q", vr.Phone)
	}
}

func TestEngineRateLimit(t *testing.T) {
	ms := newMemStore()
	engine := NewEngine(ms, EngineConfig{
		CodeLength:             6,
		ExpiryMinutes:          5,
		MaxAttempts:            5,
		RateLimitPerPhonePerHr: 2,
	})
	ctx := context.Background()

	// Send 2 OTPs (should succeed)
	for i := 0; i < 2; i++ {
		_, err := engine.Send(ctx, "+212600000000")
		if err != nil {
			t.Fatalf("Send #%d error: %v", i+1, err)
		}
	}

	// 3rd should be rate limited
	_, err := engine.Send(ctx, "+212600000000")
	if err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}

	// Different phone should still work
	_, err = engine.Send(ctx, "+33600000000")
	if err != nil {
		t.Fatalf("Send to different phone error: %v", err)
	}
}

func TestEngineMaxAttempts(t *testing.T) {
	ms := newMemStore()
	engine := NewEngine(ms, EngineConfig{
		CodeLength:             6,
		ExpiryMinutes:          5,
		MaxAttempts:            2,
		RateLimitPerPhonePerHr: 10,
	})
	ctx := context.Background()

	result, _ := engine.Send(ctx, "+212600000000")

	// Use up all attempts with wrong codes
	for i := 0; i < 2; i++ {
		_, _ = engine.Verify(ctx, result.Token, "000000")
	}

	// Next attempt should fail with max attempts error
	_, err := engine.Verify(ctx, result.Token, result.Code)
	if err != ErrMaxAttempts {
		t.Fatalf("expected ErrMaxAttempts, got: %v", err)
	}
}

func TestEngineExpiry(t *testing.T) {
	ms := newMemStore()
	engine := NewEngine(ms, EngineConfig{
		CodeLength:             6,
		ExpiryMinutes:          0, // will set to 0 minutes = immediate expiry
		MaxAttempts:            5,
		RateLimitPerPhonePerHr: 10,
	})
	// Override ExpiryMinutes to something tiny for test
	engine.config.ExpiryMinutes = 1
	ctx := context.Background()

	result, _ := engine.Send(ctx, "+212600000000")

	// Manually set expiry to the past
	if req, ok := ms.otps[result.Token]; ok {
		req.ExpiresAt = time.Now().Add(-1 * time.Minute)
	}

	_, err := engine.Verify(ctx, result.Token, result.Code)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestEngineAlreadyVerified(t *testing.T) {
	ms := newMemStore()
	engine := NewEngine(ms, EngineConfig{
		CodeLength:             6,
		ExpiryMinutes:          5,
		MaxAttempts:            5,
		RateLimitPerPhonePerHr: 10,
	})
	ctx := context.Background()

	result, _ := engine.Send(ctx, "+212600000000")

	// Verify once (success)
	_, err := engine.Verify(ctx, result.Token, result.Code)
	if err != nil {
		t.Fatal(err)
	}

	// Try to verify again
	_, err = engine.Verify(ctx, result.Token, result.Code)
	if err != ErrAlreadyVerified {
		t.Fatalf("expected ErrAlreadyVerified, got: %v", err)
	}
}

func TestEngineTokenNotFound(t *testing.T) {
	ms := newMemStore()
	engine := NewEngine(ms, EngineConfig{
		CodeLength:             6,
		ExpiryMinutes:          5,
		MaxAttempts:            5,
		RateLimitPerPhonePerHr: 10,
	})
	ctx := context.Background()

	_, err := engine.Verify(ctx, "otp_tok_nonexistent", "123456")
	if err != ErrTokenNotFound {
		t.Fatalf("expected ErrTokenNotFound, got: %v", err)
	}
}
