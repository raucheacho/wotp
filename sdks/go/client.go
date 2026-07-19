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

// SendMedia sends a media message.
func (c *Client) SendMedia(ctx context.Context, phone string, media SendMediaRequest) (*MessageResponse, error) {
	media.Phone = phone
	media.Type = "media"
	var resp MessageResponse

	if err := c.doRequest(ctx, http.MethodPost, "/v1/messages/send", media, &resp); err != nil {
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
