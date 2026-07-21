package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// metaWebhookPayload is the shape of Meta's inbound webhook POST body.
// See https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/payload-examples
type metaWebhookPayload struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Text      struct {
						Body string `json:"body"`
					} `json:"text"`
					Location struct {
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
						Name      string  `json:"name"`
					} `json:"location"`
					// Image/Video/Document carry an optional Caption;
					// Audio doesn't — WhatsApp's protocol has no caption
					// concept for voice notes, same fact as whatsmeow's
					// AudioMessage (see extractInboundMedia).
					Image struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						Caption  string `json:"caption"`
					} `json:"image"`
					Video struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						Caption  string `json:"caption"`
					} `json:"video"`
					Audio struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
					} `json:"audio"`
					Document struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						Caption  string `json:"caption"`
					} `json:"document"`
				} `json:"messages"`
				Statuses []struct {
					ID          string `json:"id"`
					Status      string `json:"status"`
					Timestamp   string `json:"timestamp"`
					RecipientID string `json:"recipient_id"`
				} `json:"statuses"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// metaStatusEventType maps Meta's status webhook values onto the same
// Event.Type constants whatsmeow's Pool already emits for delivery
// receipts — so StartEventForwarder's status-update switch (keyed on
// these strings) needs no backend-specific branching.
var metaStatusEventType = map[string]string{
	"sent":      EventMessageSent,
	"delivered": EventMessageDelivered,
	"read":      EventMessageRead,
	"failed":    EventMessageFailed,
}

// ParseMetaWebhook translates a Meta Cloud API webhook POST body into
// wotp's backend-agnostic Event shape — deliberately the same shape
// whatsmeow's Pool emits (same Data map keys for inbound messages, same
// Type constants for status updates), so routeInboundMessage and
// StartEventForwarder need no backend-specific handling. See
// conversations.go's evt.Data.(map[string]interface{}) type assertion for
// the contract this must satisfy.
func ParseMetaWebhook(body []byte) ([]Event, error) {
	var payload metaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("whatsapp/cloud: decode webhook payload: %w", err)
	}

	var events []Event
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			names := make(map[string]string, len(change.Value.Contacts))
			for _, c := range change.Value.Contacts {
				names[c.WaID] = c.Profile.Name
			}

			for _, m := range change.Value.Messages {
				text := m.Text.Body
				var mediaKind, mediaID, mediaMimeType string
				switch {
				case m.Location.Latitude != 0 || m.Location.Longitude != 0:
					text = FormatLocationText(m.Location.Name, m.Location.Latitude, m.Location.Longitude)
				case m.Image.ID != "":
					text, mediaKind, mediaID, mediaMimeType = m.Image.Caption, string(MediaKindImage), m.Image.ID, m.Image.MimeType
				case m.Video.ID != "":
					text, mediaKind, mediaID, mediaMimeType = m.Video.Caption, string(MediaKindVideo), m.Video.ID, m.Video.MimeType
				case m.Audio.ID != "":
					mediaKind, mediaID, mediaMimeType = string(MediaKindAudio), m.Audio.ID, m.Audio.MimeType
				case m.Document.ID != "":
					text, mediaKind, mediaID, mediaMimeType = m.Document.Caption, string(MediaKindDocument), m.Document.ID, m.Document.MimeType
				}

				data := map[string]interface{}{
					"text":     text,
					"pushName": names[m.From],
				}
				if mediaKind != "" {
					// mediaID (Meta's reference, not bytes — ParseMetaWebhook
					// does no network I/O) is resolved to mediaBytes by
					// handleMetaWebhookEvents, which has rt.Cloud in scope.
					data["mediaKind"] = mediaKind
					data["mediaMimeType"] = mediaMimeType
					data["mediaID"] = mediaID
				}

				events = append(events, Event{
					Type:      EventMessageReceived,
					Phone:     m.From,
					MessageID: m.ID,
					At:        parseMetaTimestamp(m.Timestamp),
					Data:      data,
				})
			}

			for _, st := range change.Value.Statuses {
				evtType, ok := metaStatusEventType[st.Status]
				if !ok {
					continue
				}
				events = append(events, Event{
					Type:      evtType,
					Phone:     st.RecipientID,
					MessageID: st.ID,
					At:        parseMetaTimestamp(st.Timestamp),
				})
			}
		}
	}
	return events, nil
}

func parseMetaTimestamp(raw string) time.Time {
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	return time.Unix(sec, 0).UTC()
}

// VerifyMetaSignature checks the X-Hub-Signature-256 header Meta sends on
// every webhook POST — an HMAC-SHA256 of the raw request body keyed by the
// App Secret, prefixed "sha256=". Mirrors the exact primitive
// core/internal/webhooks/webhooks.go already uses to *sign* wotp's own
// outbound webhooks (crypto/hmac + crypto/sha256 + encoding/hex), just
// verifying instead of producing — hmac.Equal for constant-time
// comparison, since this is authenticating an inbound request.
func VerifyMetaSignature(body []byte, header, appSecret string) bool {
	const prefix = "sha256="
	if appSecret == "" || !strings.HasPrefix(header, prefix) {
		return false
	}
	want, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}
	h := hmac.New(sha256.New, []byte(appSecret))
	h.Write(body)
	return hmac.Equal(h.Sum(nil), want)
}
