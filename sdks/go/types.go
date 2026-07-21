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
// liveness check — see GetChats or the dashboard for the connected number's
// own status (an instance is mono-tenant: exactly one WhatsApp number).
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

// MediaKind identifies the kind of attachment for SendMedia — wotp supports
// the same four kinds on both its whatsmeow and Cloud API backends.
type MediaKind string

const (
	MediaKindImage    MediaKind = "image"
	MediaKindVideo    MediaKind = "video"
	MediaKindAudio    MediaKind = "audio"
	MediaKindDocument MediaKind = "document"
)

type SendMediaRequest struct {
	Phone   string    `json:"phone"`
	Kind    MediaKind `json:"type"`
	URL     string    `json:"url,omitempty"`
	Base64  string    `json:"base64,omitempty"`
	Caption string    `json:"caption,omitempty"`
	// Filename is shown as the file name in the recipient's chat. Only
	// meaningful when Kind is MediaKindDocument.
	Filename string `json:"filename,omitempty"`
}

// SendLocationRequest is the payload for a location-type POST
// /v1/messages/send. Name/Address are optional; Latitude/Longitude are
// required.
type SendLocationRequest struct {
	Phone     string  `json:"phone"`
	Type      string  `json:"type"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// LocationOptions are the optional fields for SendLocation — grouped in a
// struct (rather than two more trailing string params) so a caller sending
// just coordinates isn't stuck passing "", "" for name/address.
type LocationOptions struct {
	Name    string
	Address string
}

// MessageResponse is the result of a successful POST /v1/messages/send.
// There is no "success" field — a failed send comes back as a non-2xx
// status and doRequest returns it as an error instead.
type MessageResponse struct {
	MessageID string `json:"message_id,omitempty"`
}

// Chat is a WhatsApp contact visible to the connected number.
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

// ─── Conversations & takeover ─────────────────────────────────────

// Conversation states.
const (
	ConversationStateBot   = "bot"
	ConversationStateHuman = "human"
)

// Conversation is a contact's WhatsApp conversation thread — one per phone
// number, created automatically on first inbound contact. State is "bot"
// (default) until a human takes it over.
type Conversation struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationMessage is one entry in GetConversationMessages' merged,
// chronological thread — inbound replies, outbound sends, and OTP sends all
// show up here. Kind is "otp"/"text"/"media" for outbound entries, or an
// inbound media message's kind ("image"/"video"/"audio"/"document"); empty
// for a plain inbound text/location message.
type ConversationMessage struct {
	Direction string `json:"direction"`
	Kind      string `json:"kind,omitempty"`
	Content   string `json:"content"`
	PushName  string `json:"push_name,omitempty"`
	// MediaMimeType is set alongside Kind for an inbound media message —
	// see GetMedia to fetch the actual bytes.
	MediaMimeType string    `json:"media_mime_type,omitempty"`
	MessageID     string    `json:"message_id,omitempty"`
	Status        string    `json:"status,omitempty"`
	At            time.Time `json:"at"`
}

// ConversationStateChangeRequest is the optional payload for
// TakeoverConversation/ResumeConversation — both fields are freeform and
// optional, but recording them is what makes a takeover auditable instead
// of a silent state flip.
type ConversationStateChangeRequest struct {
	Actor  string `json:"actor,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ─── Inbound media ─────────────────────────────────────────────────

// MediaFile is the raw bytes of a downloaded inbound media message — an
// image, video, voice note, or document a contact sent in, ready to feed to
// OCR, Whisper, or wherever else your bot needs it.
type MediaFile struct {
	Data        []byte
	ContentType string
}
