package whatsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if _, err := client.SendMedia(context.Background(), "15550100", "", "aGVsbG8=", ""); err == nil {
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

	result, err := client.SendMedia(context.Background(), "15550100", "https://example.com/pic.jpg", "", "a caption")
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

func TestCloudClient_SetPresenceAndGetChatsAreUnsupported(t *testing.T) {
	client := NewCloudClient(CloudConfig{PhoneNumberID: "123456", AccessToken: "t"})
	if err := client.SetPresence(context.Background(), "15550100", PresenceTyping); err == nil {
		t.Fatal("expected SetPresence to return an error on the cloud backend")
	}
	if _, err := client.GetChats(context.Background()); err == nil {
		t.Fatal("expected GetChats to return an error on the cloud backend")
	}
}

func TestCloudClient_DisconnectIsANoOp(t *testing.T) {
	client := NewCloudClient(CloudConfig{PhoneNumberID: "123456", AccessToken: "t"})
	client.Disconnect() // must not panic
}
