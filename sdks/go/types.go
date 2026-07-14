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

// HealthResponse is the result of a health check.
type HealthResponse struct {
	// Status is the WhatsApp connection status (e.g. "connected").
	Status string `json:"status"`
	// Phone is the connected phone number.
	Phone string `json:"phone"`
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
