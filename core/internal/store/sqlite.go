package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteProjectStore is the SQLite implementation of ProjectStore — the
// single data.db file for this wotp-core instance (see project.Load).
type SQLiteProjectStore struct {
	db     *sql.DB
	logger *slog.Logger
}

var projectMigrations = []Migration{
	{
		Version: 1,
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS otp_requests (
				id         TEXT PRIMARY KEY,
				token      TEXT NOT NULL UNIQUE,
				phone      TEXT NOT NULL,
				code_hash  TEXT NOT NULL,
				status     TEXT NOT NULL DEFAULT 'pending',
				message_id TEXT NOT NULL DEFAULT '',
				attempts   INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME NOT NULL,
				expires_at DATETIME NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_otp_requests_token ON otp_requests(token)`,
			`CREATE INDEX IF NOT EXISTS idx_otp_requests_phone_created ON otp_requests(phone, created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_otp_requests_message_id ON otp_requests(message_id)`,
			`CREATE TABLE IF NOT EXISTS generic_messages (
				id           TEXT PRIMARY KEY,
				phone        TEXT NOT NULL,
				message_type TEXT NOT NULL,
				content      TEXT NOT NULL,
				status       TEXT NOT NULL DEFAULT 'pending',
				error        TEXT,
				created_at   DATETIME NOT NULL,
				updated_at   DATETIME NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS webhook_logs (
				id          TEXT PRIMARY KEY,
				event_type  TEXT NOT NULL,
				payload     TEXT NOT NULL,
				status_code INTEGER NOT NULL,
				error       TEXT,
				created_at  DATETIME NOT NULL
			)`,
		},
	},
	{
		Version: 2,
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS conversations (
				id         TEXT PRIMARY KEY,
				phone      TEXT NOT NULL UNIQUE,
				state      TEXT NOT NULL DEFAULT 'bot',
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS conversation_state_changes (
				id              TEXT PRIMARY KEY,
				conversation_id TEXT NOT NULL REFERENCES conversations(id),
				from_state      TEXT NOT NULL,
				to_state        TEXT NOT NULL,
				actor           TEXT NOT NULL DEFAULT '',
				reason          TEXT NOT NULL DEFAULT '',
				created_at      DATETIME NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_conversation_state_changes_conv ON conversation_state_changes(conversation_id, created_at)`,
			`CREATE TABLE IF NOT EXISTS inbound_messages (
				id              TEXT PRIMARY KEY,
				conversation_id TEXT NOT NULL REFERENCES conversations(id),
				phone           TEXT NOT NULL,
				content         TEXT NOT NULL DEFAULT '',
				push_name       TEXT NOT NULL DEFAULT '',
				message_id      TEXT NOT NULL DEFAULT '',
				created_at      DATETIME NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_inbound_messages_phone ON inbound_messages(phone, created_at)`,
		},
	},
	{
		Version: 3,
		SQL: []string{
			// Links an OTP send into the conversation for its phone number
			// (see GetOrCreateConversation) — NOT NULL DEFAULT '' matches
			// message_id's existing convention on this table rather than
			// NULL, so scans never need sql.NullString handling.
			`ALTER TABLE otp_requests ADD COLUMN conversation_id TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX IF NOT EXISTS idx_otp_requests_conversation_id ON otp_requests(conversation_id, created_at)`,
		},
	},
	{
		Version: 4,
		SQL: []string{
			// media_kind/media_mime_type record that an inbound message
			// carried an attachment wotp downloaded to MediaDir (see
			// project.Runtime.MediaDir, GET /v1/media/{message_id}) — both
			// empty for a plain text/location message.
			`ALTER TABLE inbound_messages ADD COLUMN media_kind TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE inbound_messages ADD COLUMN media_mime_type TEXT NOT NULL DEFAULT ''`,
			// Not unique: whatsmeow can redeliver the same message_id after
			// a reconnect (at-least-once delivery), and InsertInboundMessage
			// doesn't dedupe — a hard constraint here would turn a harmless
			// redelivery into an insert failure. GetInboundMessageByMessageID
			// picks the most recent match instead of relying on uniqueness.
			`CREATE INDEX IF NOT EXISTS idx_inbound_messages_message_id ON inbound_messages(message_id)`,
		},
	},
}

// NewSQLiteProjectStore opens (or creates) a SQLite database at the given
// path and runs migrations.
func NewSQLiteProjectStore(dbPath string, logger *slog.Logger) (*SQLiteProjectStore, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1) // SQLite works best with a single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := applyMigrations(db, projectMigrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate project store: %w", err)
	}

	logger.Info("project store initialized", "path", dbPath)
	return &SQLiteProjectStore{db: db, logger: logger}, nil
}

// CreateOTPRequest inserts a new OTP request into the database.
func (s *SQLiteProjectStore) CreateOTPRequest(ctx context.Context, req *OTPRequest) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO otp_requests (id, token, phone, code_hash, status, message_id, attempts, conversation_id, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Token, req.Phone, req.CodeHash, req.Status, req.MessageID,
		req.Attempts, req.ConversationID, req.CreatedAt.UTC().Format(time.RFC3339), req.ExpiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: create otp request: %w", err)
	}
	return nil
}

// GetOTPRequestByToken retrieves an OTP request by its opaque token.
func (s *SQLiteProjectStore) GetOTPRequestByToken(ctx context.Context, token string) (*OTPRequest, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, token, phone, code_hash, status, message_id, attempts, conversation_id, created_at, expires_at
		 FROM otp_requests WHERE token = ?`, token,
	)

	var req OTPRequest
	var createdAt, expiresAt string
	err := row.Scan(
		&req.ID, &req.Token, &req.Phone, &req.CodeHash,
		&req.Status, &req.MessageID, &req.Attempts, &req.ConversationID,
		&createdAt, &expiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get otp request: %w", err)
	}
	req.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	req.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	return &req, nil
}

// UpdateOTPStatus changes the status of an OTP request.
func (s *SQLiteProjectStore) UpdateOTPStatus(ctx context.Context, token string, status OTPStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE otp_requests SET status = ? WHERE token = ?`, status, token,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update otp status: %w", err)
	}
	return nil
}

// UpdateOTPStatusByMessageID changes the status of an OTP request using its WhatsApp message ID.
func (s *SQLiteProjectStore) UpdateOTPStatusByMessageID(ctx context.Context, messageID string, status OTPStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE otp_requests SET status = ? WHERE message_id = ?`,
		status, messageID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update otp status by message_id: %w", err)
	}
	return nil
}

// UpdateOTPMessageID sets the WhatsApp message ID for tracking delivery.
func (s *SQLiteProjectStore) UpdateOTPMessageID(ctx context.Context, token string, messageID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE otp_requests SET message_id = ? WHERE token = ?`, messageID, token,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update message id: %w", err)
	}
	return nil
}

// IncrementAttempts bumps the attempt counter and returns the new value.
func (s *SQLiteProjectStore) IncrementAttempts(ctx context.Context, token string) (int, error) {
	var attempts int
	err := s.db.QueryRowContext(ctx,
		`UPDATE otp_requests SET attempts = attempts + 1 WHERE token = ? RETURNING attempts`, token,
	).Scan(&attempts)
	if err != nil {
		return 0, fmt.Errorf("sqlite: increment attempts: %w", err)
	}
	return attempts, nil
}

// CountRecentOTPs counts how many OTP requests were created for a phone number
// since the given time. Used for rate limiting.
func (s *SQLiteProjectStore) CountRecentOTPs(ctx context.Context, phone string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM otp_requests WHERE phone = ? AND created_at >= ?`,
		phone, since.UTC().Format(time.RFC3339),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count recent otps: %w", err)
	}
	return count, nil
}

// GetRecentOTPs returns the most recent OTP requests.
func (s *SQLiteProjectStore) GetRecentOTPs(ctx context.Context, limit int) ([]OTPRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, phone, status, message_id, conversation_id, created_at, expires_at
		 FROM otp_requests ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get recent otps: %w", err)
	}
	defer rows.Close()

	var otps []OTPRequest
	for rows.Next() {
		var req OTPRequest
		var createdAt, expiresAt string
		if err := rows.Scan(&req.ID, &req.Token, &req.Phone, &req.Status, &req.MessageID, &req.ConversationID, &createdAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan recent otp: %w", err)
		}
		req.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		req.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		otps = append(otps, req)
	}
	return otps, rows.Err()
}

// GetOTPRequestsByConversationID returns the OTP requests linked to a
// conversation (see OTPRequest.ConversationID), most recent last — matching
// ListInboundMessagesByPhone's ordering so both slot into the same
// chronological merge in handleGetConversationMessages.
func (s *SQLiteProjectStore) GetOTPRequestsByConversationID(ctx context.Context, conversationID string, limit int) ([]OTPRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, phone, status, message_id, conversation_id, created_at, expires_at
		 FROM otp_requests WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?`,
		conversationID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get otp requests by conversation: %w", err)
	}
	defer rows.Close()

	var otps []OTPRequest
	for rows.Next() {
		var req OTPRequest
		var createdAt, expiresAt string
		if err := rows.Scan(&req.ID, &req.Token, &req.Phone, &req.Status, &req.MessageID, &req.ConversationID, &createdAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan otp request: %w", err)
		}
		req.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		req.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		otps = append(otps, req)
	}
	// Reverse to chronological (oldest first) — same reason as
	// ListInboundMessagesByPhone: the query orders DESC so LIMIT keeps the
	// most *recent* N, not the oldest N.
	for i, j := 0, len(otps)-1; i < j; i, j = i+1, j-1 {
		otps[i], otps[j] = otps[j], otps[i]
	}
	return otps, rows.Err()
}

// ExpireStaleOTPs marks pending/sent OTP requests as expired if their
// expiry time has passed. Returns the number of rows updated.
func (s *SQLiteProjectStore) ExpireStaleOTPs(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE otp_requests SET status = 'expired'
		 WHERE status IN ('pending', 'sent', 'delivered', 'read')
		 AND expires_at <= ?`, now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("sqlite: expire stale otps: %w", err)
	}
	return result.RowsAffected()
}

// SaveGenericMessage inserts a new generic message record.
func (s *SQLiteProjectStore) SaveGenericMessage(ctx context.Context, msg *GenericMessage) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO generic_messages (id, phone, message_type, content, status, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Phone, msg.MessageType, msg.Content, msg.Status, msg.Error,
		msg.CreatedAt.UTC().Format(time.RFC3339), msg.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: save generic message: %w", err)
	}
	return nil
}

// UpdateGenericMessageStatus updates the status and error of a generic message.
func (s *SQLiteProjectStore) UpdateGenericMessageStatus(ctx context.Context, id string, status string, errStr string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE generic_messages SET status = ?, error = ?, updated_at = ? WHERE id = ?`,
		status, errStr, time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update generic message status: %w", err)
	}
	return nil
}

// GetGenericMessages retrieves the most recent generic messages.
func (s *SQLiteProjectStore) GetGenericMessages(ctx context.Context, limit int) ([]GenericMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, phone, message_type, content, status, error, created_at, updated_at
		 FROM generic_messages ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get generic messages: %w", err)
	}
	defer rows.Close()

	var msgs []GenericMessage
	for rows.Next() {
		var msg GenericMessage
		var createdAt, updatedAt string
		var errStr sql.NullString
		if err := rows.Scan(&msg.ID, &msg.Phone, &msg.MessageType, &msg.Content, &msg.Status, &errStr, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan generic message: %w", err)
		}
		if errStr.Valid {
			msg.Error = errStr.String
		}
		msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		msg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// GetGenericMessagesByPhone retrieves the most recent generic (outbound)
// messages sent to a specific phone number — used to merge outbound history
// alongside inbound messages for a conversation. phone is matched after
// normalizing both sides (see NormalizePhone), since callers may have
// stored it with or without formatting like "+" or spaces.
func (s *SQLiteProjectStore) GetGenericMessagesByPhone(ctx context.Context, phone string, limit int) ([]GenericMessage, error) {
	target := NormalizePhone(phone)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, phone, message_type, content, status, error, created_at, updated_at
		 FROM generic_messages ORDER BY created_at DESC LIMIT ?`, limit*4, // over-fetch since we filter in Go
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get generic messages by phone: %w", err)
	}
	defer rows.Close()

	var msgs []GenericMessage
	for rows.Next() {
		var msg GenericMessage
		var createdAt, updatedAt string
		var errStr sql.NullString
		if err := rows.Scan(&msg.ID, &msg.Phone, &msg.MessageType, &msg.Content, &msg.Status, &errStr, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan generic message: %w", err)
		}
		if NormalizePhone(msg.Phone) != target {
			continue
		}
		if errStr.Valid {
			msg.Error = errStr.String
		}
		msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		msg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		msgs = append(msgs, msg)
		if len(msgs) >= limit {
			break
		}
	}
	return msgs, rows.Err()
}

// GetOrCreateConversation returns the conversation for phone (normalized —
// see NormalizePhone), creating one in the default (bot) state if this is
// the first time this contact has been seen.
func (s *SQLiteProjectStore) GetOrCreateConversation(ctx context.Context, phone string) (*Conversation, error) {
	normalized := NormalizePhone(phone)

	existing, err := s.getConversationByPhone(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := time.Now().UTC()
	conv := &Conversation{
		ID:        uuid.New().String(),
		Phone:     normalized,
		State:     ConversationStateBot,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO conversations (id, phone, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(phone) DO NOTHING`,
		conv.ID, conv.Phone, conv.State, conv.CreatedAt.Format(time.RFC3339), conv.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: create conversation: %w", err)
	}

	// Another goroutine may have won the race on the UNIQUE(phone)
	// constraint between our SELECT and INSERT — re-read to return
	// whichever row actually ended up in the table.
	created, err := s.getConversationByPhone(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, fmt.Errorf("sqlite: conversation for %q not found after insert", normalized)
	}
	return created, nil
}

func (s *SQLiteProjectStore) getConversationByPhone(ctx context.Context, phone string) (*Conversation, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, phone, state, created_at, updated_at FROM conversations WHERE phone = ?`, phone,
	)
	return scanConversation(row)
}

// GetConversationByID looks up a conversation by its ID — returns (nil, nil)
// if it doesn't exist.
func (s *SQLiteProjectStore) GetConversationByID(ctx context.Context, id string) (*Conversation, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, phone, state, created_at, updated_at FROM conversations WHERE id = ?`, id,
	)
	return scanConversation(row)
}

func scanConversation(row *sql.Row) (*Conversation, error) {
	var c Conversation
	var createdAt, updatedAt string
	err := row.Scan(&c.ID, &c.Phone, &c.State, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: scan conversation: %w", err)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &c, nil
}

// ListConversations returns every conversation for this project, most
// recently updated first.
func (s *SQLiteProjectStore) ListConversations(ctx context.Context) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, phone, state, created_at, updated_at FROM conversations ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list conversations: %w", err)
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.Phone, &c.State, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan conversation: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetConversationState updates a conversation's state and records the
// transition in its audit trail, in a single transaction — see the
// ProjectStore interface doc comment for why these two always happen
// together.
func (s *SQLiteProjectStore) SetConversationState(ctx context.Context, id, state, actor, reason string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin set conversation state: %w", err)
	}
	defer tx.Rollback()

	var fromState string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM conversations WHERE id = ?`, id).Scan(&fromState); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("sqlite: conversation %q not found", id)
		}
		return fmt.Errorf("sqlite: read conversation state: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`UPDATE conversations SET state = ?, updated_at = ? WHERE id = ?`, state, now, id,
	); err != nil {
		return fmt.Errorf("sqlite: update conversation state: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO conversation_state_changes (id, conversation_id, from_state, to_state, actor, reason, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), id, fromState, state, actor, reason, now,
	); err != nil {
		return fmt.Errorf("sqlite: insert conversation state change: %w", err)
	}

	return tx.Commit()
}

// ListConversationStateChanges returns a conversation's full audit trail,
// oldest first.
func (s *SQLiteProjectStore) ListConversationStateChanges(ctx context.Context, conversationID string) ([]ConversationStateChange, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, conversation_id, from_state, to_state, actor, reason, created_at
		 FROM conversation_state_changes WHERE conversation_id = ? ORDER BY created_at ASC`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list conversation state changes: %w", err)
	}
	defer rows.Close()

	var out []ConversationStateChange
	for rows.Next() {
		var c ConversationStateChange
		var createdAt string
		if err := rows.Scan(&c.ID, &c.ConversationID, &c.FromState, &c.ToState, &c.Actor, &c.Reason, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan conversation state change: %w", err)
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// InsertInboundMessage persists a message received from a counterpart.
func (s *SQLiteProjectStore) InsertInboundMessage(ctx context.Context, msg *InboundMessage) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO inbound_messages (id, conversation_id, phone, content, push_name, message_id, media_kind, media_mime_type, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, msg.Phone, msg.Content, msg.PushName, msg.MessageID,
		msg.MediaKind, msg.MediaMimeType, msg.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: insert inbound message: %w", err)
	}
	return nil
}

// ListInboundMessagesByPhone returns the most recent inbound messages for a
// phone number (normalized — see NormalizePhone), most recent last (so
// callers can render them straight into a chronological chat view).
func (s *SQLiteProjectStore) ListInboundMessagesByPhone(ctx context.Context, phone string, limit int) ([]InboundMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, conversation_id, phone, content, push_name, message_id, media_kind, media_mime_type, created_at
		 FROM inbound_messages WHERE phone = ? ORDER BY created_at DESC LIMIT ?`,
		NormalizePhone(phone), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list inbound messages: %w", err)
	}
	defer rows.Close()

	var out []InboundMessage
	for rows.Next() {
		var m InboundMessage
		var createdAt string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Phone, &m.Content, &m.PushName, &m.MessageID, &m.MediaKind, &m.MediaMimeType, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan inbound message: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, m)
	}
	// Reverse to chronological (oldest first) — the query above orders
	// DESC so LIMIT keeps the most *recent* N, not the oldest N.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// GetInboundMessageByMessageID looks up an inbound message by its WhatsApp
// message id — used by GET /v1/media/{message_id}. Not globally unique
// (see the migration v4 index comment: whatsmeow can redeliver a message_id
// after a reconnect), so this returns the most recent match rather than
// erroring on more than one.
func (s *SQLiteProjectStore) GetInboundMessageByMessageID(ctx context.Context, messageID string) (*InboundMessage, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, conversation_id, phone, content, push_name, message_id, media_kind, media_mime_type, created_at
		 FROM inbound_messages WHERE message_id = ? ORDER BY created_at DESC LIMIT 1`,
		messageID,
	)
	var m InboundMessage
	var createdAt string
	err := row.Scan(&m.ID, &m.ConversationID, &m.Phone, &m.Content, &m.PushName, &m.MessageID, &m.MediaKind, &m.MediaMimeType, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get inbound message by message id: %w", err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &m, nil
}

// SaveWebhookLog inserts a new webhook log record.
func (s *SQLiteProjectStore) SaveWebhookLog(ctx context.Context, log *WebhookLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_logs (id, event_type, payload, status_code, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		log.ID, log.EventType, log.Payload, log.StatusCode, log.Error,
		log.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: save webhook log: %w", err)
	}
	return nil
}

// GetWebhookLogs retrieves the most recent webhook logs.
func (s *SQLiteProjectStore) GetWebhookLogs(ctx context.Context, limit int) ([]WebhookLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, event_type, payload, status_code, error, created_at
		 FROM webhook_logs ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get webhook logs: %w", err)
	}
	defer rows.Close()

	var logs []WebhookLog
	for rows.Next() {
		var l WebhookLog
		var createdAt string
		var errStr sql.NullString
		if err := rows.Scan(&l.ID, &l.EventType, &l.Payload, &l.StatusCode, &errStr, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan webhook log: %w", err)
		}
		if errStr.Valid {
			l.Error = errStr.String
		}
		l.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// Close closes the underlying database connection.
func (s *SQLiteProjectStore) Close() error {
	return s.db.Close()
}
