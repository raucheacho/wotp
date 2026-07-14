package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore is the SQLite implementation of the Store interface.
type SQLiteStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLiteStore opens (or creates) a SQLite database at the given path
// and runs migrations.
func NewSQLiteStore(dbPath string, logger *slog.Logger) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1) // SQLite works best with a single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &SQLiteStore{db: db, logger: logger}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}

	logger.Info("sqlite store initialized", "path", dbPath)
	return s, nil
}

func (s *SQLiteStore) migrate() error {
	migrations := []string{
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
		`CREATE TABLE IF NOT EXISTS api_keys (
			id         TEXT PRIMARY KEY,
			key_hash   TEXT NOT NULL,
			key_prefix TEXT NOT NULL UNIQUE,
			tier       TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix)`,
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
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w\nSQL: %s", err, m)
		}
	}
	return nil
}

// CreateOTPRequest inserts a new OTP request into the database.
func (s *SQLiteStore) CreateOTPRequest(ctx context.Context, req *OTPRequest) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO otp_requests (id, token, phone, code_hash, status, message_id, attempts, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Token, req.Phone, req.CodeHash, req.Status, req.MessageID,
		req.Attempts, req.CreatedAt.UTC().Format(time.RFC3339), req.ExpiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: create otp request: %w", err)
	}
	return nil
}

// GetOTPRequestByToken retrieves an OTP request by its opaque token.
func (s *SQLiteStore) GetOTPRequestByToken(ctx context.Context, token string) (*OTPRequest, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, token, phone, code_hash, status, message_id, attempts, created_at, expires_at
		 FROM otp_requests WHERE token = ?`, token,
	)

	var req OTPRequest
	var createdAt, expiresAt string
	err := row.Scan(
		&req.ID, &req.Token, &req.Phone, &req.CodeHash,
		&req.Status, &req.MessageID, &req.Attempts,
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
func (s *SQLiteStore) UpdateOTPStatus(ctx context.Context, token string, status OTPStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE otp_requests SET status = ? WHERE token = ?`, status, token,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update otp status: %w", err)
	}
	return nil
}

// UpdateOTPStatusByMessageID changes the status of an OTP request using its WhatsApp message ID.
func (s *SQLiteStore) UpdateOTPStatusByMessageID(ctx context.Context, messageID string, status OTPStatus) error {
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
func (s *SQLiteStore) UpdateOTPMessageID(ctx context.Context, token string, messageID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE otp_requests SET message_id = ? WHERE token = ?`, messageID, token,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update message id: %w", err)
	}
	return nil
}

// IncrementAttempts bumps the attempt counter and returns the new value.
func (s *SQLiteStore) IncrementAttempts(ctx context.Context, token string) (int, error) {
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
func (s *SQLiteStore) CountRecentOTPs(ctx context.Context, phone string, since time.Time) (int, error) {
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
func (s *SQLiteStore) GetRecentOTPs(ctx context.Context, limit int) ([]OTPRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, phone, status, message_id, created_at, expires_at
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
		if err := rows.Scan(&req.ID, &req.Token, &req.Phone, &req.Status, &req.MessageID, &createdAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan recent otp: %w", err)
		}
		req.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		req.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		otps = append(otps, req)
	}
	return otps, rows.Err()
}

// ExpireStaleOTPs marks pending/sent OTP requests as expired if their
// expiry time has passed. Returns the number of rows updated.
func (s *SQLiteStore) ExpireStaleOTPs(ctx context.Context, now time.Time) (int64, error) {
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

// CreateAPIKey inserts a new API key record.
func (s *SQLiteStore) CreateAPIKey(ctx context.Context, key *APIKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, key_hash, key_prefix, tier, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		key.ID, key.KeyHash, key.KeyPrefix, key.Tier, key.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: create api key: %w", err)
	}
	return nil
}

// GetAPIKeyByPrefix retrieves an API key by its unique prefix.
func (s *SQLiteStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, tier, created_at FROM api_keys WHERE key_prefix = ?`, prefix,
	)

	var key APIKey
	var createdAt string
	err := row.Scan(&key.ID, &key.KeyHash, &key.KeyPrefix, &key.Tier, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get api key: %w", err)
	}
	key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &key, nil
}

// ListAPIKeys returns all stored API keys.
func (s *SQLiteStore) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, key_hash, key_prefix, tier, created_at FROM api_keys ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		var createdAt string
		if err := rows.Scan(&key.ID, &key.KeyHash, &key.KeyPrefix, &key.Tier, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan api key: %w", err)
		}
		key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// DeleteAPIKeysByTier removes all API keys of a given tier.
func (s *SQLiteStore) DeleteAPIKeysByTier(ctx context.Context, tier string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE tier = ?`, tier,
	)
	if err != nil {
		return fmt.Errorf("sqlite: delete api keys by tier: %w", err)
	}
	return nil
}

// SaveGenericMessage inserts a new generic message record.
func (s *SQLiteStore) SaveGenericMessage(ctx context.Context, msg *GenericMessage) error {
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
func (s *SQLiteStore) UpdateGenericMessageStatus(ctx context.Context, id string, status string, errStr string) error {
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
func (s *SQLiteStore) GetGenericMessages(ctx context.Context, limit int) ([]GenericMessage, error) {
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

// SaveWebhookLog inserts a new webhook log record.
func (s *SQLiteStore) SaveWebhookLog(ctx context.Context, log *WebhookLog) error {
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
func (s *SQLiteStore) GetWebhookLogs(ctx context.Context, limit int) ([]WebhookLog, error) {
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
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
