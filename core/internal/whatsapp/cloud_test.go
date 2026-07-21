package whatsapp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wotp/core/internal/store"
)

func newTestCloudClient(t *testing.T, handler http.HandlerFunc) *CloudClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewCloudClient(CloudConfig{
		PhoneNumberID:       "123456",
		AccessToken:         "test-token",
		OTPTemplateName:     "otp_verification",
		OTPTemplateLanguage: "en_US",
		BaseURL:             srv.URL,
		HTTPClient:          srv.Client(),
	})
}

func newTestProjectStore(t *testing.T) *store.SQLiteProjectStore {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ps, err := store.NewSQLiteProjectStore(filepath.Join(t.TempDir(), "data.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteProjectStore: %v", err)
	}
	t.Cleanup(func() { ps.Close() })
	return ps
}

func TestCloudClient_ConnectVerifiesCredentials(t *testing.T) {
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/123456") {
			t.Fatalf("expected path to start with /123456, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		json.NewEncoder(w).Encode(map[string]string{
			"id":                   "123456",
			"display_phone_number": "+1 555 0100",
		})
	})

	if client.IsConnected() {
		t.Fatal("expected IsConnected to be false before Connect")
	}

	qrChan, err := client.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if qrChan != nil {
		t.Fatal("expected Connect to return a nil channel (Cloud API has no QR pairing)")
	}
	if !client.IsConnected() {
		t.Fatal("expected IsConnected to be true after a successful Connect")
	}
	if got := client.GetPhoneNumber(); got != "+1 555 0100" {
		t.Fatalf("GetPhoneNumber = %q, want %q", got, "+1 555 0100")
	}
}

func TestCloudClient_ConnectSurfacesMetaError(t *testing.T) {
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Invalid OAuth access token", "code": 190},
		})
	})

	if _, err := client.Connect(context.Background()); err == nil {
		t.Fatal("expected an error for an invalid access token")
	} else if !strings.Contains(err.Error(), "Invalid OAuth access token") {
		t.Fatalf("error = %v, want it to surface Meta's message", err)
	}
	if client.IsConnected() {
		t.Fatal("expected IsConnected to stay false after a failed Connect")
	}
}

func TestCloudClient_SendMessageBuildsTextPayload(t *testing.T) {
	var captured map[string]any
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/123456/messages" {
			t.Fatalf("path = %q, want /123456/messages", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.abc"}},
		})
	})

	result, err := client.SendMessage(context.Background(), "+1 (555) 010-0", "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if result.MessageID != "wamid.abc" {
		t.Fatalf("MessageID = %q, want wamid.abc", result.MessageID)
	}
	if captured["to"] != "15550100" {
		t.Fatalf("to = %v, want digits-only 15550100", captured["to"])
	}
	if captured["type"] != "text" {
		t.Fatalf("type = %v, want text", captured["type"])
	}
	text, _ := captured["text"].(map[string]any)
	if text["body"] != "hello" {
		t.Fatalf("text.body = %v, want hello", text["body"])
	}

	select {
	case evt := <-client.Events():
		if evt.Type != EventMessageSent || evt.MessageID != "wamid.abc" {
			t.Fatalf("unexpected event: %+v", evt)
		}
	default:
		t.Fatal("expected a message.sent event to be emitted")
	}
}

func TestCloudClient_SendMessageEmitsFailedEventOnError(t *testing.T) {
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Recipient not found", "code": 131030},
		})
	})

	if _, err := client.SendMessage(context.Background(), "15550100", "hello"); err == nil {
		t.Fatal("expected an error")
	}

	select {
	case evt := <-client.Events():
		if evt.Type != EventMessageFailed {
			t.Fatalf("expected message.failed event, got %+v", evt)
		}
	default:
		t.Fatal("expected a message.failed event to be emitted")
	}
}

func TestCloudClient_SendMediaRequiresURL(t *testing.T) {
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP call should be made when url and base64Data are both empty")
	})
	if _, err := client.SendMedia(context.Background(), "15550100", MediaSendOptions{Base64Data: "aGVsbG8="}); err == nil {
		t.Fatal("expected an error when only base64Data is provided (not supported yet)")
	}
}

func TestCloudClient_SendMediaBuildsImagePayload(t *testing.T) {
	var captured map[string]any
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.media"}},
		})
	})

	result, err := client.SendMedia(context.Background(), "15550100", MediaSendOptions{URL: "https://example.com/pic.jpg", Caption: "a caption"})
	if err != nil {
		t.Fatalf("SendMedia: %v", err)
	}
	if result.MessageID != "wamid.media" {
		t.Fatalf("MessageID = %q, want wamid.media", result.MessageID)
	}
	if captured["type"] != "image" {
		t.Fatalf("type = %v, want image", captured["type"])
	}
	image, _ := captured["image"].(map[string]any)
	if image["link"] != "https://example.com/pic.jpg" {
		t.Fatalf("image.link = %v", image["link"])
	}
	if image["caption"] != "a caption" {
		t.Fatalf("image.caption = %v", image["caption"])
	}
}

func TestCloudClient_SendLocationBuildsPayload(t *testing.T) {
	var captured map[string]any
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.location"}},
		})
	})

	result, err := client.SendLocation(context.Background(), "15550100", LocationSendOptions{
		Latitude: 33.5731, Longitude: -7.5898, Name: "Casablanca", Address: "Morocco",
	})
	if err != nil {
		t.Fatalf("SendLocation: %v", err)
	}
	if result.MessageID != "wamid.location" {
		t.Fatalf("MessageID = %q, want wamid.location", result.MessageID)
	}
	if captured["type"] != "location" {
		t.Fatalf("type = %v, want location", captured["type"])
	}
	loc, _ := captured["location"].(map[string]any)
	if loc["latitude"] != 33.5731 || loc["longitude"] != -7.5898 {
		t.Fatalf("location lat/long = %v/%v, want 33.5731/-7.5898", loc["latitude"], loc["longitude"])
	}
	if loc["name"] != "Casablanca" || loc["address"] != "Morocco" {
		t.Fatalf("location name/address = %v/%v, want Casablanca/Morocco", loc["name"], loc["address"])
	}
}

func TestCloudClient_SendLocationOmitsEmptyNameAndAddress(t *testing.T) {
	var captured map[string]any
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.location"}},
		})
	})

	if _, err := client.SendLocation(context.Background(), "15550100", LocationSendOptions{Latitude: 1, Longitude: 2}); err != nil {
		t.Fatalf("SendLocation: %v", err)
	}
	loc, _ := captured["location"].(map[string]any)
	if _, ok := loc["name"]; ok {
		t.Fatalf("expected no name field when unset, got %v", loc["name"])
	}
	if _, ok := loc["address"]; ok {
		t.Fatalf("expected no address field when unset, got %v", loc["address"])
	}
}

func TestCloudClient_SendOTPTemplateBuildsTemplatePayload(t *testing.T) {
	var captured map[string]any
	client := newTestCloudClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]string{{"id": "wamid.otp"}},
		})
	})

	result, err := client.SendOTPTemplate(context.Background(), "15550100", "123456", 5)
	if err != nil {
		t.Fatalf("SendOTPTemplate: %v", err)
	}
	if result.MessageID != "wamid.otp" {
		t.Fatalf("MessageID = %q, want wamid.otp", result.MessageID)
	}
	if captured["type"] != "template" {
		t.Fatalf("type = %v, want template", captured["type"])
	}
	tpl, _ := captured["template"].(map[string]any)
	if tpl["name"] != "otp_verification" {
		t.Fatalf("template.name = %v, want otp_verification", tpl["name"])
	}
	lang, _ := tpl["language"].(map[string]any)
	if lang["code"] != "en_US" {
		t.Fatalf("template.language.code = %v, want en_US", lang["code"])
	}
	components, _ := tpl["components"].([]any)
	if len(components) != 1 {
		t.Fatalf("expected exactly one component, got %d", len(components))
	}
	body, _ := components[0].(map[string]any)
	if body["type"] != "body" {
		t.Fatalf("component.type = %v, want body", body["type"])
	}
	params, _ := body["parameters"].([]any)
	if len(params) != 1 {
		t.Fatalf("expected exactly one body parameter, got %d", len(params))
	}
	param, _ := params[0].(map[string]any)
	if param["text"] != "123456" {
		t.Fatalf("parameter.text = %v, want 123456", param["text"])
	}
}

func TestCloudClient_SendOTPTemplateRequiresConfiguredTemplate(t *testing.T) {
	client := NewCloudClient(CloudConfig{
		PhoneNumberID: "123456",
		AccessToken:   "test-token",
		// OTPTemplateName intentionally left empty.
	})
	if _, err := client.SendOTPTemplate(context.Background(), "15550100", "123456", 5); err == nil {
		t.Fatal("expected an error when no OTP template is configured")
	}
}

func TestCloudClient_SetPresenceAndGetChatsRequireAStore(t *testing.T) {
	client := NewCloudClient(CloudConfig{PhoneNumberID: "123456", AccessToken: "t"})
	if err := client.SetPresence(context.Background(), "15550100", PresenceTyping); err == nil {
		t.Fatal("expected SetPresence to error when no store is configured")
	}
	if _, err := client.GetChats(context.Background()); err == nil {
		t.Fatal("expected GetChats to error when no store is configured")
	}
}

func TestCloudClient_SetPresenceErrorsWithoutAnInboundMessage(t *testing.T) {
	ps := newTestProjectStore(t)
	client := NewCloudClient(CloudConfig{PhoneNumberID: "123456", AccessToken: "t", Store: ps})
	if err := client.SetPresence(context.Background(), "15550100", PresenceTyping); err == nil {
		t.Fatal("expected SetPresence to error when this phone has never sent an inbound message")
	}
}

func TestCloudClient_SetPresenceSendsTypingIndicatorForLastInboundMessage(t *testing.T) {
	ps := newTestProjectStore(t)
	conv, err := ps.GetOrCreateConversation(context.Background(), "15550100")
	if err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}
	if err := ps.InsertInboundMessage(context.Background(), &store.InboundMessage{
		ConversationID: conv.ID, Phone: "15550100", Content: "hi", MessageID: "wamid.IN1", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("InsertInboundMessage: %v", err)
	}

	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte("{}"))
	}))
	t.Cleanup(srv.Close)
	client := NewCloudClient(CloudConfig{
		PhoneNumberID: "123456", AccessToken: "t", Store: ps,
		BaseURL: srv.URL, HTTPClient: srv.Client(),
	})

	if err := client.SetPresence(context.Background(), "15550100", PresenceTyping); err != nil {
		t.Fatalf("SetPresence: %v", err)
	}
	if captured["message_id"] != "wamid.IN1" {
		t.Fatalf("message_id = %v, want wamid.IN1 (the last inbound message)", captured["message_id"])
	}
	if captured["status"] != "read" {
		t.Fatalf("status = %v, want read", captured["status"])
	}
	typing, _ := captured["typing_indicator"].(map[string]any)
	if typing["type"] != "text" {
		t.Fatalf("typing_indicator.type = %v, want text", typing["type"])
	}

	// state=paused marks the message read without showing a typing
	// indicator — there's no explicit "stop typing" call on Cloud API,
	// it just naturally isn't shown when this field is omitted.
	captured = nil
	if err := client.SetPresence(context.Background(), "15550100", PresencePaused); err != nil {
		t.Fatalf("SetPresence (paused): %v", err)
	}
	if _, ok := captured["typing_indicator"]; ok {
		t.Fatalf("expected no typing_indicator field for state=paused, got %v", captured["typing_indicator"])
	}
}

func TestCloudClient_GetChatsListsConversations(t *testing.T) {
	ps := newTestProjectStore(t)
	if _, err := ps.GetOrCreateConversation(context.Background(), "212600000000"); err != nil {
		t.Fatalf("GetOrCreateConversation: %v", err)
	}

	client := NewCloudClient(CloudConfig{PhoneNumberID: "123456", AccessToken: "t", Store: ps})
	chats, err := client.GetChats(context.Background())
	if err != nil {
		t.Fatalf("GetChats: %v", err)
	}
	if len(chats) != 1 || chats[0].JID != "212600000000@s.whatsapp.net" {
		t.Fatalf("GetChats = %+v, want [212600000000@s.whatsapp.net]", chats)
	}
}

func TestCloudClient_DisconnectIsANoOp(t *testing.T) {
	client := NewCloudClient(CloudConfig{PhoneNumberID: "123456", AccessToken: "t"})
	client.Disconnect() // must not panic
}
