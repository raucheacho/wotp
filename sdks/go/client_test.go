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
		if r.URL.Path != "/v1/otp/send" {
			t.Errorf("expected path /v1/otp/send, got %s", r.URL.Path)
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
		if r.URL.Path != "/v1/otp/verify" {
			t.Errorf("expected path /v1/otp/verify, got %s", r.URL.Path)
		}
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
		if r.URL.Path != "/v1/health" {
			t.Errorf("expected path /v1/health, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"uptime_seconds": 12345,
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
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
			"status":         "ok",
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
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
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
			"message_id": "msg_123",
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.SendText(context.Background(), "+212600000000", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.MessageID != "msg_123" {
		t.Errorf("expected msg_123, got '%s'", resp.MessageID)
	}
}

func TestSendText_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to send message"})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	_, err := client.SendText(context.Background(), "+212600000000", "hello")
	if err == nil {
		t.Fatal("expected an error when the send fails server-side")
	}
}

func TestGetChats_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chats" {
			t.Errorf("expected /v1/chats, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{
			{"jid": "212600000000@s.whatsapp.net", "name": "Alice"},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	chats, err := client.GetChats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chats) != 1 || chats[0].JID != "212600000000@s.whatsapp.net" || chats[0].Name != "Alice" {
		t.Fatalf("unexpected chats: %+v", chats)
	}
}

func TestSendMedia_DefaultsToImageKind(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body SendMediaRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Kind != MediaKindImage {
			t.Errorf("expected Kind %q (default), got %q", MediaKindImage, body.Kind)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"message_id": "msg_media"})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.SendMedia(context.Background(), "+212600000000", SendMediaRequest{URL: "https://example.com/a.jpg"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID != "msg_media" {
		t.Errorf("expected msg_media, got %q", resp.MessageID)
	}
}

func TestSendMedia_ExplicitDocumentKindAndFilename(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body SendMediaRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Kind != MediaKindDocument {
			t.Errorf("expected Kind %q, got %q", MediaKindDocument, body.Kind)
		}
		if body.Filename != "invoice.pdf" {
			t.Errorf("expected filename invoice.pdf, got %q", body.Filename)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"message_id": "msg_doc"})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	_, err := client.SendMedia(context.Background(), "+212600000000", SendMediaRequest{
		Kind: MediaKindDocument, URL: "https://example.com/invoice.pdf", Filename: "invoice.pdf",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendLocation_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages/send" {
			t.Errorf("expected /v1/messages/send, got %s", r.URL.Path)
		}
		var body SendLocationRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.Type != "location" {
			t.Errorf("expected type location, got %q", body.Type)
		}
		if body.Latitude != 33.5731 || body.Longitude != -7.5898 {
			t.Errorf("unexpected coordinates: %+v", body)
		}
		if body.Name != "Casablanca" {
			t.Errorf("expected name Casablanca, got %q", body.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"message_id": "msg_loc"})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	resp, err := client.SendLocation(context.Background(), "+212600000000", 33.5731, -7.5898, &LocationOptions{Name: "Casablanca"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID != "msg_loc" {
		t.Errorf("expected msg_loc, got %q", resp.MessageID)
	}
}

func TestSendLocation_NilOptions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"message_id": "msg_loc"})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	if _, err := client.SendLocation(context.Background(), "+212600000000", 33.5731, -7.5898, nil); err != nil {
		t.Fatalf("unexpected error with nil opts: %v", err)
	}
}

func TestConversations_ListGetMessagesTakeoverResume(t *testing.T) {
	var lastTakeoverBody ConversationStateChangeRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/conversations":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "conv-1", "phone": "212600000000", "state": "bot", "created_at": "2026-07-13T10:00:00Z", "updated_at": "2026-07-13T10:00:00Z"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/conversations/conv-1":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "conv-1", "phone": "212600000000", "state": "bot", "created_at": "2026-07-13T10:00:00Z", "updated_at": "2026-07-13T10:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/conversations/conv-1/messages":
			json.NewEncoder(w).Encode([]map[string]any{
				{"direction": "inbound", "content": "hello", "at": "2026-07-13T10:00:00Z"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/conversations/conv-1/takeover":
			_ = json.NewDecoder(r.Body).Decode(&lastTakeoverBody)
			json.NewEncoder(w).Encode(map[string]string{"state": "human"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/conversations/conv-1/resume":
			json.NewEncoder(w).Encode(map[string]string{"state": "bot"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	ctx := context.Background()

	convs, err := client.ListConversations(ctx)
	if err != nil || len(convs) != 1 || convs[0].ID != "conv-1" {
		t.Fatalf("ListConversations: %v, %+v", err, convs)
	}

	conv, err := client.GetConversation(ctx, "conv-1")
	if err != nil || conv.State != "bot" {
		t.Fatalf("GetConversation: %v, %+v", err, conv)
	}

	msgs, err := client.GetConversationMessages(ctx, "conv-1")
	if err != nil || len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Fatalf("GetConversationMessages: %v, %+v", err, msgs)
	}

	if err := client.TakeoverConversation(ctx, "conv-1", &ConversationStateChangeRequest{Actor: "agent-1", Reason: "escalation"}); err != nil {
		t.Fatalf("TakeoverConversation: %v", err)
	}
	if lastTakeoverBody.Actor != "agent-1" || lastTakeoverBody.Reason != "escalation" {
		t.Fatalf("takeover body not forwarded: %+v", lastTakeoverBody)
	}

	if err := client.ResumeConversation(ctx, "conv-1", nil); err != nil {
		t.Fatalf("ResumeConversation with nil opts: %v", err)
	}
}

func TestGetMedia_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/media/wamid.IMG1" {
			t.Errorf("expected /v1/media/wamid.IMG1, got %s", r.URL.Path)
		}
		if r.Header.Get("apikey") != "test-key" {
			t.Errorf("expected apikey header, got %q", r.Header.Get("apikey"))
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	media, err := client.GetMedia(context.Background(), "wamid.IMG1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(media.Data) != "fake-jpeg-bytes" {
		t.Errorf("unexpected data: %q", media.Data)
	}
	if media.ContentType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %q", media.ContentType)
	}
}

func TestGetMedia_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "media not found"})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	_, err := client.GetMedia(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for a missing media file")
	}
	wotpErr, ok := err.(*WotpError)
	if !ok || wotpErr.StatusCode != 404 {
		t.Fatalf("expected a WotpError with StatusCode 404, got %T: %v", err, err)
	}
}

func TestSetPresence_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages/presence" {
			t.Errorf("expected /v1/messages/presence, got %s", r.URL.Path)
		}
		var body SetPresenceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body.State != PresenceTyping {
			t.Errorf("expected state %q, got %q", PresenceTyping, body.State)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, WithApiKey("test-key"))
	if err := client.SetPresence(context.Background(), "+212600000000", PresenceTyping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
