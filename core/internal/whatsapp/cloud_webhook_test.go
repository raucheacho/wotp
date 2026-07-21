package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(body []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func TestVerifyMetaSignature(t *testing.T) {
	body := []byte(`{"entry":[]}`)
	secret := "app-secret"

	if !VerifyMetaSignature(body, sign(body, secret), secret) {
		t.Error("expected a correctly signed body to verify")
	}
	if VerifyMetaSignature(body, sign(body, "wrong-secret"), secret) {
		t.Error("expected a body signed with the wrong secret to fail verification")
	}
	if VerifyMetaSignature([]byte(`{"entry":[],"tampered":true}`), sign(body, secret), secret) {
		t.Error("expected a tampered body to fail verification even with a validly-formed signature")
	}
	if VerifyMetaSignature(body, "not-even-hex", secret) {
		t.Error("expected a malformed signature header to fail verification, not panic")
	}
	if VerifyMetaSignature(body, sign(body, secret), "") {
		t.Error("expected verification to fail outright when no App Secret is configured")
	}
	if VerifyMetaSignature(body, "", secret) {
		t.Error("expected a missing signature header to fail verification")
	}
}

func TestParseMetaWebhook_InboundMessage(t *testing.T) {
	body := []byte(`{
		"entry": [{
			"changes": [{
				"value": {
					"contacts": [{"profile": {"name": "Jane Doe"}, "wa_id": "212600000000"}],
					"messages": [{
						"from": "212600000000",
						"id": "wamid.ABC123",
						"timestamp": "1700000000",
						"type": "text",
						"text": {"body": "where is my order?"}
					}]
				}
			}]
		}]
	}`)

	events, err := ParseMetaWebhook(body)
	if err != nil {
		t.Fatalf("ParseMetaWebhook: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	evt := events[0]
	if evt.Type != EventMessageReceived {
		t.Errorf("Type = %q, want %q", evt.Type, EventMessageReceived)
	}
	if evt.Phone != "212600000000" {
		t.Errorf("Phone = %q, want 212600000000", evt.Phone)
	}
	if evt.MessageID != "wamid.ABC123" {
		t.Errorf("MessageID = %q, want wamid.ABC123", evt.MessageID)
	}
	data, ok := evt.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Data to be a map, got %T", evt.Data)
	}
	if data["text"] != "where is my order?" {
		t.Errorf(`data["text"] = %v, want "where is my order?"`, data["text"])
	}
	if data["pushName"] != "Jane Doe" {
		t.Errorf(`data["pushName"] = %v, want "Jane Doe"`, data["pushName"])
	}
	if evt.At.Unix() != 1700000000 {
		t.Errorf("At = %v, want unix 1700000000", evt.At)
	}
}

// TestParseMetaWebhook_InboundLocation is a regression test for the same
// gap as whatsmeow's: an inbound location message must produce a readable
// Data["text"] (name if present, else raw coordinates), not an empty
// string just because Meta's payload has no "text" field for this type.
func TestParseMetaWebhook_InboundLocation(t *testing.T) {
	body := []byte(`{
		"entry": [{
			"changes": [{
				"value": {
					"contacts": [{"profile": {"name": "Jane"}, "wa_id": "212600000000"}],
					"messages": [{
						"from": "212600000000",
						"id": "wamid.LOC1",
						"timestamp": "1700000000",
						"type": "location",
						"location": {"latitude": 33.5731, "longitude": -7.5898, "name": "Casablanca"}
					}]
				}
			}]
		}]
	}`)

	events, err := ParseMetaWebhook(body)
	if err != nil {
		t.Fatalf("ParseMetaWebhook: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	data, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Data to be a map, got %T", events[0].Data)
	}
	if data["text"] != "Casablanca" {
		t.Fatalf(`data["text"] = %v, want "Casablanca"`, data["text"])
	}
}

// TestParseMetaWebhook_InboundMedia is a regression test: ParseMetaWebhook
// must surface Meta's media reference (id/kind/mimetype) on Data even
// though it does no network I/O itself — actual bytes are resolved later by
// handleMetaWebhookEvents, which has rt.Cloud in scope.
func TestParseMetaWebhook_InboundMedia(t *testing.T) {
	body := []byte(`{
		"entry": [{
			"changes": [{
				"value": {
					"contacts": [{"profile": {"name": "Jane"}, "wa_id": "212600000000"}],
					"messages": [{
						"from": "212600000000",
						"id": "wamid.IMG1",
						"timestamp": "1700000000",
						"type": "image",
						"image": {"id": "media-id-123", "mime_type": "image/jpeg", "caption": "look at this"}
					}]
				}
			}]
		}]
	}`)

	events, err := ParseMetaWebhook(body)
	if err != nil {
		t.Fatalf("ParseMetaWebhook: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	data, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Data to be a map, got %T", events[0].Data)
	}
	if data["text"] != "look at this" {
		t.Fatalf(`data["text"] = %v, want the caption`, data["text"])
	}
	if data["mediaKind"] != string(MediaKindImage) {
		t.Fatalf(`data["mediaKind"] = %v, want %q`, data["mediaKind"], MediaKindImage)
	}
	if data["mediaMimeType"] != "image/jpeg" {
		t.Fatalf(`data["mediaMimeType"] = %v, want "image/jpeg"`, data["mediaMimeType"])
	}
	if data["mediaID"] != "media-id-123" {
		t.Fatalf(`data["mediaID"] = %v, want "media-id-123"`, data["mediaID"])
	}
	if _, present := data["mediaBytes"]; present {
		t.Fatalf(`data["mediaBytes"] = %v, want absent — ParseMetaWebhook does no network I/O`, data["mediaBytes"])
	}
}

// TestParseMetaWebhook_InboundAudioHasNoCaption mirrors extractInboundMedia
// (whatsmeow.go's pool.go equivalent): WhatsApp's protocol carries no
// caption field for voice notes, on Cloud API's side either.
func TestParseMetaWebhook_InboundAudioHasNoCaption(t *testing.T) {
	body := []byte(`{
		"entry": [{
			"changes": [{
				"value": {
					"contacts": [{"profile": {"name": "Jane"}, "wa_id": "212600000000"}],
					"messages": [{
						"from": "212600000000",
						"id": "wamid.AUD1",
						"timestamp": "1700000000",
						"type": "audio",
						"audio": {"id": "media-id-456", "mime_type": "audio/ogg"}
					}]
				}
			}]
		}]
	}`)

	events, err := ParseMetaWebhook(body)
	if err != nil {
		t.Fatalf("ParseMetaWebhook: %v", err)
	}
	data, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Data to be a map, got %T", events[0].Data)
	}
	if data["text"] != "" {
		t.Fatalf(`data["text"] = %v, want "" (audio has no caption)`, data["text"])
	}
	if data["mediaKind"] != string(MediaKindAudio) {
		t.Fatalf(`data["mediaKind"] = %v, want %q`, data["mediaKind"], MediaKindAudio)
	}
	if data["mediaID"] != "media-id-456" {
		t.Fatalf(`data["mediaID"] = %v, want "media-id-456"`, data["mediaID"])
	}
}

func TestParseMetaWebhook_StatusUpdates(t *testing.T) {
	body := []byte(`{
		"entry": [{
			"changes": [{
				"value": {
					"statuses": [
						{"id": "wamid.OUT1", "status": "delivered", "timestamp": "1700000001", "recipient_id": "212600000000"},
						{"id": "wamid.OUT2", "status": "read", "timestamp": "1700000002", "recipient_id": "212600000000"},
						{"id": "wamid.OUT3", "status": "unknown_future_status", "timestamp": "1700000003", "recipient_id": "212600000000"}
					]
				}
			}]
		}]
	}`)

	events, err := ParseMetaWebhook(body)
	if err != nil {
		t.Fatalf("ParseMetaWebhook: %v", err)
	}
	// The unrecognized "unknown_future_status" must be skipped, not error
	// out the whole payload — Meta adds new status values over time.
	if len(events) != 2 {
		t.Fatalf("expected 2 recognized status events, got %d: %+v", len(events), events)
	}
	if events[0].Type != EventMessageDelivered || events[0].MessageID != "wamid.OUT1" {
		t.Errorf("events[0] = %+v, want delivered/wamid.OUT1", events[0])
	}
	if events[1].Type != EventMessageRead || events[1].MessageID != "wamid.OUT2" {
		t.Errorf("events[1] = %+v, want read/wamid.OUT2", events[1])
	}
}

func TestParseMetaWebhook_InvalidJSON(t *testing.T) {
	if _, err := ParseMetaWebhook([]byte("not json")); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
