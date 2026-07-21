// Package wotp provides the official Go SDK for Wotp — WhatsApp OTP, self-hosted.
//
// Usage:
//
//	client := wotp.NewClient("http://localhost:54321", wotp.WithApiKey("wotp_anon_xxx"))
//
//	resp, err := client.SendOTP(ctx, "+212600000000")
//	result, err := client.VerifyOTP(ctx, resp.Token, "483920")
package wotp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ─── Default Configuration ────────────────────────────────────────

const (
	defaultMaxRetries = 3
	defaultRetryDelay = 500 * time.Millisecond
	defaultTimeout    = 10 * time.Second
)

// ─── Client ───────────────────────────────────────────────────────

// Client is the Wotp API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// ─── Functional Options ──────────────────────────────────────────

// Option configures the Wotp client.
type Option func(*Client)

// WithApiKey sets the API key for authentication.
func WithApiKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithMaxRetries sets the maximum number of retries for transient errors.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithRetryDelay sets the base delay between retries (exponential backoff).
func WithRetryDelay(d time.Duration) Option {
	return func(c *Client) {
		c.retryDelay = d
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// ─── Constructor ──────────────────────────────────────────────────

// NewClient creates a new Wotp client with the given base URL and options.
//
//	client := wotp.NewClient("http://localhost:54321", wotp.WithApiKey("wotp_anon_xxx"))
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		maxRetries: defaultMaxRetries,
		retryDelay: defaultRetryDelay,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ─── Public API ───────────────────────────────────────────────────

// SendOTP sends an OTP to the given phone number.
//
// The phone number must be in E.164 format (e.g. "+212600000000").
// Returns a token and expiration timestamp on success.
func (c *Client) SendOTP(ctx context.Context, phone string) (*SendOTPResponse, error) {
	body := SendOTPRequest{Phone: phone}
	var raw struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
		Warning   string `json:"warning,omitempty"`
	}

	if err := c.doRequest(ctx, http.MethodPost, "/v1/otp/send", body, &raw); err != nil {
		return nil, err
	}

	expiresAt, err := time.Parse(time.RFC3339, raw.ExpiresAt)
	if err != nil {
		return nil, &WotpError{Message: fmt.Sprintf("failed to parse expires_at: %v", err)}
	}

	return &SendOTPResponse{
		Token:     raw.Token,
		ExpiresAt: expiresAt,
		Warning:   raw.Warning,
	}, nil
}

// VerifyOTP verifies an OTP code against a previously issued token.
//
// Returns the verification result. If the code is invalid, returns an
// InvalidCodeError. If the token has expired, returns an ExpiredTokenError.
func (c *Client) VerifyOTP(ctx context.Context, token, code string) (*VerifyOTPResponse, error) {
	body := VerifyOTPRequest{Token: token, Code: code}
	var raw struct {
		Verified          bool   `json:"verified"`
		Phone             string `json:"phone,omitempty"`
		AttemptsRemaining *int   `json:"attempts_remaining,omitempty"`
	}

	if err := c.doRequest(ctx, http.MethodPost, "/v1/otp/verify", body, &raw); err != nil {
		return nil, err
	}

	return &VerifyOTPResponse{
		Verified:          raw.Verified,
		Phone:             raw.Phone,
		AttemptsRemaining: raw.AttemptsRemaining,
	}, nil
}

// Health checks the health of the Wotp instance.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v1/health", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ─── Internal HTTP Logic ──────────────────────────────────────────

func (c *Client) doRequest(ctx context.Context, method, path string, body any, result any) error {
	url := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		var reqBody io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return &WotpError{Message: fmt.Sprintf("failed to marshal request: %v", err)}
			}
			reqBody = bytes.NewReader(b)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return &WotpError{Message: fmt.Sprintf("failed to create request: %v", err)}
		}

		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("apikey", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error — retry
			lastErr = &WotpError{Message: fmt.Sprintf("network error: %v", err)}
			if attempt < c.maxRetries {
				c.sleep(attempt)
				continue
			}
			return lastErr
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return &WotpError{Message: fmt.Sprintf("failed to read response: %v", err)}
		}

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return &WotpError{Message: fmt.Sprintf("failed to parse response: %v", err)}
			}
			return nil
		}

		// Parse error body
		var apiErr apiErrorResponse
		_ = json.Unmarshal(respBody, &apiErr)

		// Business errors — never retry
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			retryAfter := 0
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				retryAfter, _ = strconv.Atoi(ra)
			}
			return &RateLimitError{
				WotpError:  WotpError{StatusCode: 429, Message: "rate limit exceeded"},
				RetryAfter: retryAfter,
			}

		case http.StatusBadRequest:
			if apiErr.Error == "token_expired" {
				return &ExpiredTokenError{
					WotpError: WotpError{StatusCode: 400, Message: "OTP token has expired"},
				}
			}
			if apiErr.Error == "invalid_code" {
				remaining := 0
				if apiErr.AttemptsRemaining != nil {
					remaining = *apiErr.AttemptsRemaining
				}
				return &InvalidCodeError{
					WotpError:         WotpError{StatusCode: 400, Message: "invalid OTP code"},
					AttemptsRemaining: remaining,
				}
			}
			msg := apiErr.Message
			if msg == "" {
				msg = string(respBody)
			}
			return &WotpError{StatusCode: resp.StatusCode, Message: msg}

		case http.StatusGone:
			return &ExpiredTokenError{
				WotpError: WotpError{StatusCode: 410, Message: "OTP token has expired"},
			}
		}

		// Transient server errors — retry
		if isTransientStatus(resp.StatusCode) {
			lastErr = &WotpError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("server error: %s", string(respBody))}
			if attempt < c.maxRetries {
				c.sleep(attempt)
				continue
			}
			return lastErr
		}

		// Unknown error
		msg := apiErr.Message
		if msg == "" {
			msg = string(respBody)
		}
		return &WotpError{StatusCode: resp.StatusCode, Message: msg}
	}

	if lastErr != nil {
		return lastErr
	}
	return &WotpError{Message: "request failed after retries"}
}

func isTransientStatus(status int) bool {
	return status == 502 || status == 503 || status == 504
}

func (c *Client) sleep(attempt int) {
	delay := c.retryDelay * (1 << attempt)
	time.Sleep(delay)
}

// SendText sends a text message.
func (c *Client) SendText(ctx context.Context, phone, text string) (*MessageResponse, error) {
	body := SendTextRequest{Phone: phone, Type: "text", Text: text}
	var resp MessageResponse

	if err := c.doRequest(ctx, http.MethodPost, "/v1/messages/send", body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SendMedia sends a media message (image, video, audio, or document — see
// MediaKind). Kind defaults to MediaKindImage when left unset, matching the
// API's legacy "media" alias.
func (c *Client) SendMedia(ctx context.Context, phone string, media SendMediaRequest) (*MessageResponse, error) {
	media.Phone = phone
	if media.Kind == "" {
		media.Kind = MediaKindImage
	}
	var resp MessageResponse

	if err := c.doRequest(ctx, http.MethodPost, "/v1/messages/send", media, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SendLocation sends a WhatsApp location message. opts may be nil if
// neither a name nor an address label is needed.
func (c *Client) SendLocation(ctx context.Context, phone string, latitude, longitude float64, opts *LocationOptions) (*MessageResponse, error) {
	body := SendLocationRequest{Phone: phone, Type: "location", Latitude: latitude, Longitude: longitude}
	if opts != nil {
		body.Name = opts.Name
		body.Address = opts.Address
	}
	var resp MessageResponse

	if err := c.doRequest(ctx, http.MethodPost, "/v1/messages/send", body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetChats lists all chats.
func (c *Client) GetChats(ctx context.Context) ([]Chat, error) {
	var resp []Chat
	if err := c.doRequest(ctx, http.MethodGet, "/v1/chats", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SetPresence sets the typing indicator for a chat without sending a
// message. state must be PresenceTyping or PresencePaused.
func (c *Client) SetPresence(ctx context.Context, phone, state string) error {
	body := SetPresenceRequest{Phone: phone, State: state}
	var resp struct {
		OK bool `json:"ok"`
	}
	return c.doRequest(ctx, http.MethodPost, "/v1/messages/presence", body, &resp)
}

// ─── Conversations & takeover ─────────────────────────────────────

// ListConversations lists every tracked conversation (one per contact that
// has ever messaged in).
func (c *Client) ListConversations(ctx context.Context) ([]Conversation, error) {
	var resp []Conversation
	if err := c.doRequest(ctx, http.MethodGet, "/v1/conversations", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetConversation fetches a single conversation by id.
func (c *Client) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	var resp Conversation
	if err := c.doRequest(ctx, http.MethodGet, "/v1/conversations/"+url.PathEscape(id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetConversationMessages returns the full chronological thread for a
// conversation — inbound replies, outbound sends, and OTP sends merged
// together.
func (c *Client) GetConversationMessages(ctx context.Context, id string) ([]ConversationMessage, error) {
	var resp []ConversationMessage
	if err := c.doRequest(ctx, http.MethodGet, "/v1/conversations/"+url.PathEscape(id)+"/messages", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// TakeoverConversation marks a conversation as human-owned. wotp keeps
// forwarding message.received either way — it's up to your own bot logic
// to read conversation_state from the webhook payload and stay quiet.
// opts may be nil.
func (c *Client) TakeoverConversation(ctx context.Context, id string, opts *ConversationStateChangeRequest) error {
	return c.setConversationState(ctx, id, "takeover", opts)
}

// ResumeConversation hands a conversation back to the bot. opts may be nil.
func (c *Client) ResumeConversation(ctx context.Context, id string, opts *ConversationStateChangeRequest) error {
	return c.setConversationState(ctx, id, "resume", opts)
}

func (c *Client) setConversationState(ctx context.Context, id, action string, opts *ConversationStateChangeRequest) error {
	var body ConversationStateChangeRequest
	if opts != nil {
		body = *opts
	}
	var resp struct {
		State string `json:"state"`
	}
	return c.doRequest(ctx, http.MethodPost, "/v1/conversations/"+url.PathEscape(id)+"/"+action, body, &resp)
}

// ─── Inbound media ─────────────────────────────────────────────────

// GetMedia downloads the raw bytes of an inbound media message wotp
// captured at receive time (see ConversationMessage.Kind /
// SendMedia/MediaKind). Returns a WotpError with StatusCode 404 if the
// message wasn't a media message, or if the download itself failed when
// the message arrived.
func (c *Client) GetMedia(ctx context.Context, messageID string) (*MediaFile, error) {
	reqURL := c.baseURL + "/v1/media/" + url.PathEscape(messageID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &WotpError{Message: fmt.Sprintf("failed to create request: %v", err)}
	}
	if c.apiKey != "" {
		req.Header.Set("apikey", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &WotpError{Message: fmt.Sprintf("network error: %v", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &WotpError{Message: fmt.Sprintf("failed to read response: %v", err)}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &MediaFile{Data: body, ContentType: resp.Header.Get("Content-Type")}, nil
	}

	var apiErr apiErrorResponse
	_ = json.Unmarshal(body, &apiErr)
	msg := apiErr.Message
	if msg == "" {
		msg = string(body)
	}
	return nil, &WotpError{StatusCode: resp.StatusCode, Message: msg}
}
