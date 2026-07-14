package wotp

import "fmt"

// ─── Base Error ───────────────────────────────────────────────────

// WotpError is the base error type for all Wotp SDK errors.
type WotpError struct {
	// StatusCode is the HTTP status code returned by the API (0 for network errors).
	StatusCode int
	// Message is a human-readable error message.
	Message string
}

func (e *WotpError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("wotp: %s (HTTP %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("wotp: %s", e.Message)
}

// ─── Typed Business Errors ────────────────────────────────────────

// RateLimitError is returned when the API responds with HTTP 429.
// The phone number or IP has exceeded the configured rate limit.
type RateLimitError struct {
	WotpError
	// RetryAfter is the number of seconds to wait before retrying (if provided).
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("wotp: rate limit exceeded, retry after %ds", e.RetryAfter)
	}
	return "wotp: rate limit exceeded"
}

// ExpiredTokenError is returned when verification is attempted with an expired token.
type ExpiredTokenError struct {
	WotpError
}

func (e *ExpiredTokenError) Error() string {
	return "wotp: OTP token has expired"
}

// InvalidCodeError is returned when the OTP code is incorrect.
type InvalidCodeError struct {
	WotpError
	// AttemptsRemaining is the number of remaining verification attempts.
	AttemptsRemaining int
}

func (e *InvalidCodeError) Error() string {
	if e.AttemptsRemaining > 0 {
		return fmt.Sprintf("wotp: invalid OTP code, %d attempts remaining", e.AttemptsRemaining)
	}
	return "wotp: invalid OTP code"
}

// ─── Error Type Checks ───────────────────────────────────────────

// IsRateLimitError checks if err is a RateLimitError.
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// IsExpiredTokenError checks if err is an ExpiredTokenError.
func IsExpiredTokenError(err error) bool {
	_, ok := err.(*ExpiredTokenError)
	return ok
}

// IsInvalidCodeError checks if err is an InvalidCodeError.
func IsInvalidCodeError(err error) bool {
	_, ok := err.(*InvalidCodeError)
	return ok
}
