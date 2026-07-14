package wotp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSendOTP_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/otp/send" {
			t.Errorf("expected /otp/send, got %s", r.URL.Path)
		}
		if r.Header.Get("apikey") != "test-key" {
			t.Errorf("expected apikey header 'test-key', got '%s'", r.Header.Get("apikey"))
		}

		var body SendOTPRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Phone != "+212600000000" {
			t.Errorf("expected phone '+212600000000', got '%s'", body.Phone)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token":      "otp_tok_abc123",
			"expires_at": "2026-07-13T10:35:00Z",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.SendOTP(context.Background(), "+212600000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Token != "otp_tok_abc123" {
		t.Errorf("expected token 'otp_tok_abc123', got '%s'", resp.Token)
	}

	expected := time.Date(2026, 7, 13, 10, 35, 0, 0, time.UTC)
	if !resp.ExpiresAt.Equal(expected) {
		t.Errorf("expected ExpiresAt %v, got %v", expected, resp.ExpiresAt)
	}
}

func TestVerifyOTP_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body VerifyOTPRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"verified": true,
			"phone":    "+212600000000",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.VerifyOTP(context.Background(), "otp_tok_abc123", "483920")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Verified {
		t.Error("expected verified to be true")
	}
	if resp.Phone != "+212600000000" {
		t.Errorf("expected phone '+212600000000', got '%s'", resp.Phone)
	}
}

func TestVerifyOTP_InvalidCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		remaining := 3
		json.NewEncoder(w).Encode(map[string]any{
			"verified":           false,
			"error":              "invalid_code",
			"attempts_remaining": remaining,
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	_, err := client.VerifyOTP(context.Background(), "otp_tok_abc123", "000000")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsInvalidCodeError(err) {
		t.Fatalf("expected InvalidCodeError, got %T: %v", err, err)
	}

	codeErr := err.(*InvalidCodeError)
	if codeErr.AttemptsRemaining != 3 {
		t.Errorf("expected 3 attempts remaining, got %d", codeErr.AttemptsRemaining)
	}
}

func TestVerifyOTP_ExpiredToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]any{
			"error":   "expired_token",
			"message": "OTP token has expired",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	_, err := client.VerifyOTP(context.Background(), "otp_tok_expired", "483920")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsExpiredTokenError(err) {
		t.Fatalf("expected ExpiredTokenError, got %T: %v", err, err)
	}
}

func TestSendOTP_RateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Rate limit exceeded",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	_, err := client.SendOTP(context.Background(), "+212600000000")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !IsRateLimitError(err) {
		t.Fatalf("expected RateLimitError, got %T: %v", err, err)
	}

	rlErr := err.(*RateLimitError)
	if rlErr.RetryAfter != 60 {
		t.Errorf("expected RetryAfter 60, got %d", rlErr.RetryAfter)
	}
}

func TestHealth_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/health" {
			t.Errorf("expected /health, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":         "connected",
			"phone":          "+212600000000",
			"uptime_seconds": 12345,
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "connected" {
		t.Errorf("expected status 'connected', got '%s'", resp.Status)
	}
	if resp.Phone != "+212600000000" {
		t.Errorf("expected phone '+212600000000', got '%s'", resp.Phone)
	}
	if resp.UptimeSeconds != 12345 {
		t.Errorf("expected uptime 12345, got %d", resp.UptimeSeconds)
	}
}

func TestRetryOnTransientError(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":         "connected",
			"phone":          "+212600000000",
			"uptime_seconds": 100,
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL,
		WithApiKey("test-key"),
		WithMaxRetries(3),
		WithRetryDelay(10*time.Millisecond),
	)
	resp, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if resp.Status != "connected" {
		t.Errorf("expected status 'connected', got '%s'", resp.Status)
	}
}

func TestNoRetryOnBusinessError(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Rate limit exceeded",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL,
		WithApiKey("test-key"),
		WithMaxRetries(3),
	)
	_, err := client.SendOTP(context.Background(), "+212600000000")
	if err == nil {
		t.Fatal("expected error")
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on business error), got %d", attempts)
	}
}

func TestFunctionalOptions(t *testing.T) {
	client := NewClient("http://localhost:54321",
		WithApiKey("my-key"),
		WithMaxRetries(5),
		WithRetryDelay(1*time.Second),
	)

	if client.apiKey != "my-key" {
		t.Errorf("expected apiKey 'my-key', got '%s'", client.apiKey)
	}
	if client.maxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", client.maxRetries)
	}
	if client.retryDelay != 1*time.Second {
		t.Errorf("expected retryDelay 1s, got %v", client.retryDelay)
	}
}

func TestSendText_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/messages/send" {
			t.Errorf("expected /v1/messages/send, got %s", r.URL.Path)
		}

		var body SendTextRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Phone != "+212600000000" {
			t.Errorf("expected phone '+212600000000', got '%s'", body.Phone)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success":   true,
			"messageId": "msg_123",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.SendText(context.Background(), "+212600000000", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.MessageID != "msg_123" {
		t.Errorf("expected msg_123, got '%s'", resp.MessageID)
	}
}
