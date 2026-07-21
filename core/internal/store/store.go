// Package store defines the data access interface for wotp-core
// and provides a SQLite implementation.
package store

import (
	"context"
	"time"
)

// OTPStatus represents the lifecycle state of an OTP request.
type OTPStatus string

const (
	StatusPending   OTPStatus = "pending"
	StatusSent      OTPStatus = "sent"
	StatusDelivered OTPStatus = "delivered"
	StatusRead      OTPStatus = "read"
	StatusVerified  OTPStatus = "verified"
	StatusExpired   OTPStatus = "expired"
	StatusFailed    OTPStatus = "failed"
)

// OTPRequest represents a single OTP verification request stored in the database.
type OTPRequest struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Phone     string    `json:"phone"`
	CodeHash  string    `json:"-"`
	Status    OTPStatus `json:"status"`
	MessageID string    `json:"message_id,omitempty"`
	Attempts  int       `json:"attempts"`
	// ConversationID links this OTP send into the conversation for its
	// phone number (see GetOrCreateConversation) — set at Send time so
	// GET /v1/conversations/{id}/messages can show it alongside inbound
	// and outbound messages. Empty for OTPs sent before this existed.
	ConversationID string    `json:"conversation_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// APIKey represents a stored API key.
type APIKey struct {
	ID        string    `json:"id"`
	KeyHash   string    `json:"-"`
	KeyPrefix string    `json:"key_prefix"`
	Tier      string    `json:"tier"` // "anon" or "service"
	CreatedAt time.Time `json:"created_at"`
}

// ProjectStore is the data access interface for the instance's data plane
// (otps, generic messages, webhook logs, conversations). wotp-core is
// mono-tenant — one ProjectStore per instance, backed by one SQLite file —
// see ControlStore (control.go) for the smaller set of instance-wide data
// (api_keys, numbers, settings) that lives alongside it in control.db.
// Implementations must be safe for concurrent use.
type ProjectStore interface {
	// OTP operations
	CreateOTPRequest(ctx context.Context, req *OTPRequest) error
	GetOTPRequestByToken(ctx context.Context, token string) (*OTPRequest, error)
	UpdateOTPStatus(ctx context.Context, token string, status OTPStatus) error
	UpdateOTPStatusByMessageID(ctx context.Context, messageID string, status OTPStatus) error
	UpdateOTPMessageID(ctx context.Context, token string, messageID string) error
	IncrementAttempts(ctx context.Context, token string) (int, error)
	CountRecentOTPs(ctx context.Context, phone string, since time.Time) (int, error)
	GetRecentOTPs(ctx context.Context, limit int) ([]OTPRequest, error)
	ExpireStaleOTPs(ctx context.Context, now time.Time) (int64, error)
	GetOTPRequestsByConversationID(ctx context.Context, conversationID string, limit int) ([]OTPRequest, error)

	// Generic Messages
	SaveGenericMessage(ctx context.Context, msg *GenericMessage) error
	UpdateGenericMessageStatus(ctx context.Context, id string, status string, errStr string) error
	GetGenericMessages(ctx context.Context, limit int) ([]GenericMessage, error)
	GetGenericMessagesByPhone(ctx context.Context, phone string, limit int) ([]GenericMessage, error)

	// Webhooks
	SaveWebhookLog(ctx context.Context, log *WebhookLog) error
	GetWebhookLogs(ctx context.Context, limit int) ([]WebhookLog, error)

	// Conversations — one per counterpart phone number, tracking whether a
	// bot or a human is currently handling it. See ConversationState* and
	// core/internal/api/conversations.go for the takeover/resume API.
	GetOrCreateConversation(ctx context.Context, phone string) (*Conversation, error)
	ListConversations(ctx context.Context) ([]Conversation, error)
	GetConversationByID(ctx context.Context, id string) (*Conversation, error)
	// SetConversationState updates a conversation's state and records the
	// change in its audit trail (actor/reason) in the same call — the two
	// always happen together, so there's no API path that can update one
	// without the other.
	SetConversationState(ctx context.Context, id, state, actor, reason string) error
	ListConversationStateChanges(ctx context.Context, conversationID string) ([]ConversationStateChange, error)
	InsertInboundMessage(ctx context.Context, msg *InboundMessage) error
	ListInboundMessagesByPhone(ctx context.Context, phone string, limit int) ([]InboundMessage, error)
	GetInboundMessageByMessageID(ctx context.Context, messageID string) (*InboundMessage, error)

	// Lifecycle
	Close() error
}

// GenericMessage represents a generic WhatsApp message stored in the database.
type GenericMessage struct {
	ID          string    `json:"id"`
	Phone       string    `json:"phone"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookLog represents a dispatched webhook stored in the database.
type WebhookLog struct {
	ID         string    `json:"id"`
	EventType  string    `json:"event_type"`
	Payload    string    `json:"payload"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Conversation states.
const (
	// ConversationStateBot is the default — inbound messages for this
	// phone still get forwarded to the project's webhook (its bot).
	ConversationStateBot = "bot"
	// ConversationStateHuman means an operator has taken over. Inbound
	// messages are still recorded AND still forwarded to the webhook —
	// wotp doesn't decide whether the app's bot should act on them, it
	// just includes this state in the forwarded payload (see
	// api.routeInboundMessage) so the app's own logic can skip replying.
	ConversationStateHuman = "human"
	// ConversationStateClosed marks a conversation as done — purely
	// informational for now (no routing difference from Bot), for
	// projects that want to track resolved threads.
	ConversationStateClosed = "closed"
)

// Conversation tracks the bot/human handoff state for one counterpart phone
// number within a project. A project has at most one WhatsApp number (see
// whatsapp.Pool), so a conversation is keyed purely by phone — no need to
// track which number it's on.
type Conversation struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationStateChange is one audit-trail row for a conversation's
// bot/human transitions — who changed it and why, so a takeover/resume
// action is never silent.
type ConversationStateChange struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	FromState      string    `json:"from_state"`
	ToState        string    `json:"to_state"`
	Actor          string    `json:"actor,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// InboundMessage is a message received from a counterpart, tied to their
// conversation. Outbound history for the same phone lives in
// GenericMessage (see GetGenericMessagesByPhone) — this table exists
// because, before conversations, inbound messages were only ever forwarded
// live (webhook/WS), never durably stored.
type InboundMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Phone          string    `json:"phone"`
	Content        string    `json:"content"`
	PushName       string    `json:"push_name,omitempty"`
	MessageID      string    `json:"message_id,omitempty"`
	// MediaKind is "image"|"video"|"audio"|"document" when this message
	// carried an attachment wotp downloaded — see GET /v1/media/{message_id}
	// — empty for a plain text/location message.
	MediaKind string `json:"media_kind,omitempty"`
	// MediaMimeType is set whenever MediaKind is, regardless of whether the
	// download itself succeeded (GET /v1/media/{id} 404s if it didn't).
	MediaMimeType string    `json:"media_mime_type,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// NormalizePhone strips everything but digits, so the same contact always
// maps to the same conversation regardless of whether a caller wrote
// "+212 600-000000" or "212600000000". Mirrors
// core/internal/whatsapp.normalizePhoneDigits — duplicated rather than
// imported to keep store dependency-free of the whatsapp package.
func NormalizePhone(phone string) string {
	digits := make([]byte, 0, len(phone))
	for i := 0; i < len(phone); i++ {
		if phone[i] >= '0' && phone[i] <= '9' {
			digits = append(digits, phone[i])
		}
	}
	return string(digits)
}
