package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// defaultCloudAPIBase is the Meta Graph API base URL used when
// CloudConfig.BaseURL is empty.
const defaultCloudAPIBase = "https://graph.facebook.com/v21.0"

// CloudConfig configures a Meta WhatsApp Cloud API-backed Client.
type CloudConfig struct {
	PhoneNumberID string
	AccessToken   string

	// OTPTemplateName/OTPTemplateLanguage identify the pre-approved
	// Authentication-category template SendOTPTemplate sends. See
	// SendOTPTemplate for why this can't be plain text.
	OTPTemplateName     string
	OTPTemplateLanguage string // e.g. "en_US"; defaults to "en_US" if empty.

	// BaseURL overrides the Graph API base URL. Empty uses the real Meta
	// endpoint — only set this in tests, against an httptest.Server.
	BaseURL string

	// HTTPClient overrides the client used for requests. Empty uses
	// http.DefaultClient.
	HTTPClient *http.Client
}

// CloudClient is a whatsapp.Client backed by the official Meta WhatsApp
// Cloud API instead of whatsmeow. Unlike Pool, it has no pairing/QR/session
// state to manage — every operation is a stateless authenticated HTTP call,
// because Meta hosts the connection, not us. It represents exactly one
// number (the one behind PhoneNumberID); there's no equivalent of Pool's
// multi-device round-robin, since a Cloud-backed number has no ban-driven
// need to spread traffic across several numbers.
type CloudClient struct {
	cfg  CloudConfig
	http *http.Client
	base string

	mu           sync.RWMutex
	displayPhone string

	events chan Event
}

// NewCloudClient builds a CloudClient. It makes no network calls — call
// Connect to verify the configured credentials against the Graph API.
func NewCloudClient(cfg CloudConfig) *CloudClient {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	base := cfg.BaseURL
	if base == "" {
		base = defaultCloudAPIBase
	}
	return &CloudClient{
		cfg:    cfg,
		http:   httpClient,
		base:   base,
		events: make(chan Event, 256),
	}
}

// metaErrorEnvelope is the error shape Meta returns on non-2xx responses.
type metaErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (c *CloudClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("whatsapp/cloud: encode request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reqBody)
	if err != nil {
		return fmt.Errorf("whatsapp/cloud: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp/cloud: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("whatsapp/cloud: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var envelope metaErrorEnvelope
		if json.Unmarshal(respBody, &envelope) == nil && envelope.Error.Message != "" {
			return fmt.Errorf("whatsapp/cloud: %s (code %d)", envelope.Error.Message, envelope.Error.Code)
		}
		return fmt.Errorf("whatsapp/cloud: meta api error: status %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("whatsapp/cloud: decode response: %w", err)
		}
	}
	return nil
}

// Connect verifies the configured phone number ID and access token against
// the Graph API. Unlike Pool.Pair, there's no QR to display — Cloud API
// numbers are registered through Meta's own console/API, not linked by
// scanning a code — so the returned channel is always nil.
func (c *CloudClient) Connect(ctx context.Context) (<-chan string, error) {
	var info struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/"+c.cfg.PhoneNumberID+"?fields=id,display_phone_number", nil, &info); err != nil {
		return nil, fmt.Errorf("whatsapp/cloud: verify phone number: %w", err)
	}
	c.mu.Lock()
	c.displayPhone = info.DisplayPhoneNumber
	c.mu.Unlock()
	return nil, nil
}

type cloudSendResponse struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

func (c *CloudClient) send(ctx context.Context, body any) (*SendResult, error) {
	var resp cloudSendResponse
	if err := c.doJSON(ctx, http.MethodPost, "/"+c.cfg.PhoneNumberID+"/messages", body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Messages) == 0 {
		return nil, fmt.Errorf("whatsapp/cloud: meta accepted the send but returned no message id")
	}
	return &SendResult{MessageID: resp.Messages[0].ID}, nil
}

// SendMessage sends a free-form text message. Per Meta's rules this only
// succeeds inside the 24-hour customer service window (the recipient must
// have messaged this number within the last 24h) — it is NOT a valid path
// for OTP, which is always the first message in a conversation. Use
// SendOTPTemplate for that.
func (c *CloudClient) SendMessage(ctx context.Context, phone, message string) (*SendResult, error) {
	result, err := c.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                normalizePhoneDigits(phone),
		"type":              "text",
		"text":              map[string]string{"body": message},
	})
	return c.finish(phone, result, err)
}

// SendMedia sends an image message via a public URL. Cloud API does not
// accept raw bytes in the send call itself — only a link Meta fetches at
// send time, or a previously-uploaded media id. Inline base64Data isn't
// supported yet since nothing in wotp needs it today (the OTP flow this
// client was built for is text-only); add a resumable-upload path if a
// Cloud-backed project ever needs to send local files.
func (c *CloudClient) SendMedia(ctx context.Context, phone, url, base64Data, caption string) (*SendResult, error) {
	if url == "" {
		return nil, fmt.Errorf("whatsapp/cloud: media send requires a public url (base64 payloads aren't supported on the cloud backend yet)")
	}
	media := map[string]any{"link": url}
	if caption != "" {
		media["caption"] = caption
	}
	result, err := c.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                normalizePhoneDigits(phone),
		"type":              "image",
		"image":             media,
	})
	return c.finish(phone, result, err)
}

// SendOTPTemplate sends code through the pre-approved Authentication
// template configured on this client. Meta requires an approved template
// for any message outside the 24h customer-service window, and an OTP is
// structurally always outside it — there is no way to send a plain-text
// OTP on the Cloud API. Implements TemplateOTPSender.
//
// expiryMinutes is accepted for interface symmetry with the whatsmeow path
// (whose rendered template text includes it) but isn't sent to Meta here —
// if the approved template itself displays an expiry, encode that as a
// static string in the template at creation time; Meta's own
// code-expiration warning is a template-creation option, not a per-send
// parameter.
func (c *CloudClient) SendOTPTemplate(ctx context.Context, phone, code string, expiryMinutes int) (*SendResult, error) {
	if c.cfg.OTPTemplateName == "" {
		return nil, fmt.Errorf("whatsapp/cloud: no OTP template configured (set otp_template_name in this project's cloud settings)")
	}
	language := c.cfg.OTPTemplateLanguage
	if language == "" {
		language = "en_US"
	}
	result, err := c.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                normalizePhoneDigits(phone),
		"type":              "template",
		"template": map[string]any{
			"name":     c.cfg.OTPTemplateName,
			"language": map[string]string{"code": language},
			"components": []map[string]any{
				{
					"type": "body",
					"parameters": []map[string]string{
						{"type": "text", "text": code},
					},
				},
			},
		},
	})
	return c.finish(phone, result, err)
}

func (c *CloudClient) finish(phone string, result *SendResult, err error) (*SendResult, error) {
	if err != nil {
		c.emitEvent(Event{Type: EventMessageFailed, Phone: phone, Error: err.Error(), At: time.Now().UTC()})
		return nil, err
	}
	c.emitEvent(Event{Type: EventMessageSent, Phone: phone, MessageID: result.MessageID, At: time.Now().UTC()})
	return result, nil
}

// SetPresence is not supported: Cloud API only exposes a typing indicator
// tied to marking one specific inbound message as read, not an independent
// "start/stop typing for this phone number" call like whatsmeow's chat
// presence — so it can't be implemented with this method's signature.
func (c *CloudClient) SetPresence(ctx context.Context, phone, state string) error {
	return fmt.Errorf("whatsapp/cloud: presence is not supported on the cloud backend")
}

// GetChats is not supported: Cloud API is stateless per-message and has no
// concept of a contact/chat list the way a whatsmeow session does.
func (c *CloudClient) GetChats(ctx context.Context) ([]Chat, error) {
	return nil, fmt.Errorf("whatsapp/cloud: chat listing is not supported on the cloud backend")
}

// IsConnected reports whether Connect has successfully verified credentials.
func (c *CloudClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.displayPhone != ""
}

// GetPhoneNumber returns the verified display phone number, or "" before
// Connect succeeds.
func (c *CloudClient) GetPhoneNumber() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.displayPhone
}

// Disconnect is a no-op: there's no persistent connection to tear down.
func (c *CloudClient) Disconnect() {}

// Events returns delivery/failure events for sends made through this
// client. Inbound message events are not populated here — Cloud API
// delivers those via a Meta-configured webhook endpoint, not a socket, and
// wiring that endpoint is separate, future work not needed for the
// OTP-only send path this client covers today.
func (c *CloudClient) Events() <-chan Event {
	return c.events
}

func (c *CloudClient) emitEvent(evt Event) {
	select {
	case c.events <- evt:
	default:
	}
}

var (
	_ Client            = (*CloudClient)(nil)
	_ TemplateOTPSender = (*CloudClient)(nil)
)
