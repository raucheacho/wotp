package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/wotp/core/internal/store"
)

// alreadyRegisteredPattern matches the error message Meta returns from
// POST /{phone_number_id}/register when this number is already subscribed
// to this exact app — the steady-state outcome of calling register() on
// every load, not a failure.
var alreadyRegisteredPattern = regexp.MustCompile(`(?i)already.*registered`)

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

	// WabaID is the WhatsApp Business Account ID that owns PhoneNumberID.
	// Only needed for inbound registration (SubscribeWabaToApp) — sending
	// (OTP, text, media) never touches it.
	WabaID string

	// AppSecret is the Meta App Secret used to verify the
	// X-Hub-Signature-256 header on inbound webhook POSTs. Required for
	// the inbound webhook receiver to accept anything; sending doesn't
	// need it.
	AppSecret string

	// VerifyToken is the arbitrary string configured on both sides (here
	// and in the Meta App dashboard's webhook subscription) to authorize
	// the GET verification handshake. Only needed for inbound.
	VerifyToken string

	// Pin is the 6-digit two-step-verification PIN set in Meta WhatsApp
	// Manager for this number — required by RegisterPhoneNumber. Left
	// empty, Connect skips registration entirely: an OTP/send-only setup
	// never needs it.
	Pin string

	// BaseURL overrides the Graph API base URL. Empty uses the real Meta
	// endpoint — only set this in tests, against an httptest.Server.
	BaseURL string

	// HTTPClient overrides the client used for requests. Empty uses
	// http.DefaultClient.
	HTTPClient *http.Client

	// Store gives SetPresence/GetChats read access to this instance's
	// conversation/inbound-message history — the one place CloudClient
	// isn't purely a stateless HTTP wrapper (see the doc comment on
	// CloudClient). Left nil, SetPresence/GetChats return a clear error
	// instead of a panic; every other method works without it.
	Store store.ProjectStore
}

// CloudClient is a whatsapp.Client backed by the official Meta WhatsApp
// Cloud API instead of whatsmeow. Unlike Pool, it has no pairing/QR/session
// state to manage — nearly every operation is a stateless authenticated
// HTTP call, because Meta hosts the connection, not us. It represents
// exactly one number (the one behind PhoneNumberID); there's no equivalent
// of Pool's multi-device round-robin, since a Cloud-backed number has no
// ban-driven need to spread traffic across several numbers.
//
// The one exception to "stateless": SetPresence and GetChats need
// conversation context Meta's API itself doesn't expose (a typing
// indicator is scoped to a specific inbound message id, not a bare phone
// number; there's no contact-list endpoint at all) — see CloudConfig.Store.
type CloudClient struct {
	cfg  CloudConfig
	http *http.Client
	base string
	// store mirrors cfg.Store — pulled out for brevity at call sites.
	store store.ProjectStore

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
		store:  cfg.Store,
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

// RegisterPhoneNumber subscribes PhoneNumberID for inbound webhook events
// on this app — POST /{phone_number_id}/register. Requires the 2FA Pin set
// in Meta WhatsApp Manager → Two-step verification; without it Meta
// rejects the call outright. Without this call, inbound events for this
// number are routed to whichever app last claimed it (often the one that
// did Embedded Signup), so a wotp instance would silently receive nothing.
func (c *CloudClient) RegisterPhoneNumber(ctx context.Context) error {
	body, err := json.Marshal(map[string]any{"messaging_product": "whatsapp", "pin": c.cfg.Pin})
	if err != nil {
		return fmt.Errorf("whatsapp/cloud: encode register request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+c.cfg.PhoneNumberID+"/register", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("whatsapp/cloud: build register request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp/cloud: register request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	var envelope metaErrorEnvelope
	_ = json.Unmarshal(respBody, &envelope)
	if envelope.Error.Message == "" {
		return fmt.Errorf("whatsapp/cloud: register: meta api error: status %d", resp.StatusCode)
	}
	// Same outcome as a fresh registration from the caller's POV — not an
	// error condition, this is what every call after the first looks like.
	if alreadyRegisteredPattern.MatchString(envelope.Error.Message) {
		return nil
	}
	return fmt.Errorf("whatsapp/cloud: register: %s (code %d)", envelope.Error.Message, envelope.Error.Code)
}

// SubscribeWabaToApp subscribes the WhatsApp Business Account (WabaID) to
// this app's webhook — POST /{waba_id}/subscribed_apps. Idempotent on
// Meta's side: safe (and necessary) to call on every load, required
// exactly once per WABA but harmless if already done.
func (c *CloudClient) SubscribeWabaToApp(ctx context.Context) error {
	if err := c.doJSON(ctx, http.MethodPost, "/"+c.cfg.WabaID+"/subscribed_apps", nil, nil); err != nil {
		return fmt.Errorf("whatsapp/cloud: subscribe waba to app: %w", err)
	}
	return nil
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

// SendLocation sends a location pin. opts.Name/Address are optional — a
// bare pin with no label is a valid send. Cloud API's location message
// shape (type: "location", location: {latitude, longitude, name?,
// address?}) is a stable, long-standing part of the API — same fields as
// whatsmeow's LocationMessage, just JSON instead of a proto struct.
func (c *CloudClient) SendLocation(ctx context.Context, phone string, opts LocationSendOptions) (*SendResult, error) {
	location := map[string]any{
		"latitude":  opts.Latitude,
		"longitude": opts.Longitude,
	}
	if opts.Name != "" {
		location["name"] = opts.Name
	}
	if opts.Address != "" {
		location["address"] = opts.Address
	}
	result, err := c.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                normalizePhoneDigits(phone),
		"type":              "location",
		"location":          location,
	})
	return c.finish(phone, result, err)
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

// SendMedia sends an image, video, audio, or document message via a public
// URL. Cloud API does not accept raw bytes in the send call itself — only
// a link Meta fetches at send time, or a previously-uploaded media id.
// Inline base64Data isn't supported yet since nothing in wotp needs it
// today (the OTP flow this client was built for is text-only); add a
// resumable-upload path if a Cloud-backed instance ever needs to send
// local files. Kind picks the Cloud "type" — audio accepts neither caption
// nor filename per Meta's spec (a caption on an audio send yields a 400),
// only document accepts a filename; both are silently dropped for kinds
// that don't support them, mirroring the whatsmeow side of this same
// parity (see mediaKindHandlers in pool.go).
func (c *CloudClient) SendMedia(ctx context.Context, phone string, opts MediaSendOptions) (*SendResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("whatsapp/cloud: media send requires a public url (base64 payloads aren't supported on the cloud backend yet)")
	}
	kind := string(opts.Kind)
	if kind == "" {
		kind = string(MediaKindImage)
	}

	media := map[string]any{"link": opts.URL}
	if opts.Caption != "" && kind != string(MediaKindAudio) {
		media["caption"] = opts.Caption
	}
	if opts.Filename != "" && kind == string(MediaKindDocument) {
		media["filename"] = opts.Filename
	}

	result, err := c.send(ctx, map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                normalizePhoneDigits(phone),
		"type":              kind,
		kind:                media,
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

// SetPresence shows (state=PresenceTyping) or clears (state=PresencePaused)
// a typing indicator in response to the most recent inbound message from
// phone — Cloud API's typing indicator is a side effect of marking a
// specific inbound message read (POST .../messages
// {status:"read", message_id, typing_indicator?}), not an independent
// "start/stop typing for this phone number" call the way whatsmeow's chat
// presence is. To keep this method's signature identical across backends,
// the last inbound message id is looked up here via the store rather than
// taken as a parameter — see CloudConfig.Store.
func (c *CloudClient) SetPresence(ctx context.Context, phone, state string) error {
	if state != PresenceTyping && state != PresencePaused {
		return fmt.Errorf("whatsapp/cloud: invalid presence state %q, must be %q or %q", state, PresenceTyping, PresencePaused)
	}
	if c.store == nil {
		return fmt.Errorf("whatsapp/cloud: presence requires a store, none configured")
	}
	last, err := c.lastInboundMessage(ctx, phone)
	if err != nil {
		return err
	}

	body := map[string]any{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        last.MessageID,
	}
	if state == PresenceTyping {
		body["typing_indicator"] = map[string]string{"type": "text"}
	}
	if err := c.doJSON(ctx, http.MethodPost, "/"+c.cfg.PhoneNumberID+"/messages", body, nil); err != nil {
		return fmt.Errorf("whatsapp/cloud: set presence: %w", err)
	}
	return nil
}

func (c *CloudClient) lastInboundMessage(ctx context.Context, phone string) (*store.InboundMessage, error) {
	msgs, err := c.store.ListInboundMessagesByPhone(ctx, phone, 1)
	if err != nil {
		return nil, fmt.Errorf("whatsapp/cloud: look up last inbound message: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("whatsapp/cloud: no inbound message from %s to respond to yet — Cloud API's typing indicator/read receipt requires an existing message", phone)
	}
	return &msgs[0], nil
}

// GetChats has no direct Cloud API equivalent — Meta exposes no
// contact/chat-list endpoint at all, stateless-per-message by design. The
// honest analog is wotp's own conversation history: every phone number
// that has exchanged a message with this instance, per the store — see
// CloudConfig.Store. (Pool.GetChats, by contrast, reads whatsmeow's live
// device contact store — deliberately not unified with this, since that
// already works and returns a different, also-valid notion of "chats".)
func (c *CloudClient) GetChats(ctx context.Context) ([]Chat, error) {
	if c.store == nil {
		return nil, fmt.Errorf("whatsapp/cloud: chat listing requires a store, none configured")
	}
	convs, err := c.store.ListConversations(ctx)
	if err != nil {
		return nil, fmt.Errorf("whatsapp/cloud: list conversations: %w", err)
	}
	chats := make([]Chat, 0, len(convs))
	for _, conv := range convs {
		chats = append(chats, Chat{JID: conv.Phone + "@s.whatsapp.net"})
	}
	return chats, nil
}

// DownloadMedia resolves a Meta media id to its CDN URL, then downloads the
// raw bytes — Meta's documented two-step flow (a webhook payload only ever
// carries the id, never the bytes or a direct URL — see ParseMetaWebhook's
// mediaID). Both requests use the same Bearer access token; the URL from
// step one is short-lived, so this should run promptly once the id arrives
// (see handleMetaWebhookEvents) rather than being cached for later.
func (c *CloudClient) DownloadMedia(ctx context.Context, mediaID string) (data []byte, mimeType string, err error) {
	var info struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/"+mediaID, nil, &info); err != nil {
		return nil, "", fmt.Errorf("whatsapp/cloud: resolve media url: %w", err)
	}
	if info.URL == "" {
		return nil, "", fmt.Errorf("whatsapp/cloud: meta returned no url for media %s", mediaID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("whatsapp/cloud: build media download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("whatsapp/cloud: download media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("whatsapp/cloud: download media: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("whatsapp/cloud: read media: %w", err)
	}
	return body, info.MimeType, nil
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
// client (OTP and generic text/media — see api.handleOTPSend/
// handleMessagesSend), plus inbound messages and delivery/read receipts
// once the Meta webhook receiver (api.handleMetaWebhookEvents, which calls
// PushEvent) is wired up and receiving traffic. Requires
// StartEventForwarder to actually be reading this channel — see
// server.go's fan-in over both rt.WA.Events() and rt.Cloud.Events().
func (c *CloudClient) Events() <-chan Event {
	return c.events
}

func (c *CloudClient) emitEvent(evt Event) {
	select {
	case c.events <- evt:
	default:
	}
}

// PushEvent injects an event onto this client's event channel — used by
// the Meta webhook receiver (ParseMetaWebhook's output) to feed inbound
// messages and status updates into the same pipeline sends already use via
// emitEvent, since neither the receiver nor callers of Events() need to
// know the difference.
func (c *CloudClient) PushEvent(evt Event) {
	c.emitEvent(evt)
}

var (
	_ Client            = (*CloudClient)(nil)
	_ TemplateOTPSender = (*CloudClient)(nil)
)
