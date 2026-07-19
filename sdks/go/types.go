// Package wotp provides request and response types for the Wotp API.
package wotp

import "time"

// ─── Requests ─────────────────────────────────────────────────────

// SendOTPRequest is the payload for POST /otp/send.
type SendOTPRequest struct {
	Phone string `json:"phone"`
}

// VerifyOTPRequest is the payload for POST /otp/verify.
type VerifyOTPRequest struct {
	Token string `json:"token"`
	Code  string `json:"code"`
}

// ─── Responses ────────────────────────────────────────────────────

// SendOTPResponse is the result of a successful OTP send.
type SendOTPResponse struct {
	// Token is the opaque token to use for verification.
	Token string `json:"token"`
	// ExpiresAt is the timestamp when this OTP expires.
	ExpiresAt time.Time `json:"expires_at"`
	// Warning is set to "message_send_failed" when the OTP was created but
	// the WhatsApp send itself failed (e.g. no number is connected yet).
	// The token is still valid — only delivery failed.
	Warning string `json:"warning,omitempty"`
}

// VerifyOTPResponse is the result of an OTP verification attempt.
type VerifyOTPResponse struct {
	// Verified indicates whether the code was correct.
	Verified bool `json:"verified"`
	// Phone is the verified phone number (only set when Verified is true).
	Phone string `json:"phone,omitempty"`
	// AttemptsRemaining is the number of remaining attempts (only set when Verified is false).
	AttemptsRemaining *int `json:"attempts_remaining,omitempty"`
}

// HealthResponse is the result of a health check. This is an instance-wide
// liveness check — it has no notion of a single connected phone number,
// since one instance can host many projects each with their own numbers.
// See GetChats or the dashboard for per-project connection state.
type HealthResponse struct {
	// Status is "ok" when the instance is up.
	Status string `json:"status"`
	// UptimeSeconds is the uptime of the Wotp instance in seconds.
	UptimeSeconds int64 `json:"uptime_seconds"`
}

// ─── Internal API shapes ──────────────────────────────────────────

// apiErrorResponse is the raw error body returned by the Wotp API.
type apiErrorResponse struct {
	Verified          *bool  `json:"verified,omitempty"`
	Error             string `json:"error,omitempty"`
	AttemptsRemaining *int   `json:"attempts_remaining,omitempty"`
	Message           string `json:"message,omitempty"`
}

// ─── Messages & Chats ─────────────────────────────────────────────

type SendTextRequest struct {
	Phone string `json:"phone"`
	Type  string `json:"type"`
	Text  string `json:"text"`
}

type SendMediaRequest struct {
	Phone   string `json:"phone"`
	Type    string `json:"type"`
	URL     string `json:"url,omitempty"`
	Base64  string `json:"base64,omitempty"`
	Caption string `json:"caption,omitempty"`
}

// MessageResponse is the result of a successful POST /v1/messages/send.
// There is no "success" field — a failed send comes back as a non-2xx
// status and doRequest returns it as an error instead.
type MessageResponse struct {
	MessageID string `json:"message_id,omitempty"`
}

// Chat is a WhatsApp contact visible to one of the project's connected numbers.
type Chat struct {
	JID  string `json:"jid"`
	Name string `json:"name,omitempty"`
}

// Presence states accepted by SetPresence.
const (
	PresenceTyping = "typing"
	PresencePaused = "paused"
)

// SetPresenceRequest is the payload for POST /v1/messages/presence.
type SetPresenceRequest struct {
	Phone string `json:"phone"`
	State string `json:"state"`
}
