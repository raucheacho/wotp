package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteProjectStore is the SQLite implementation of ProjectStore. Each
// project on a wotp-core instance gets its own file (see project.Registry).
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
func (s *SQLiteProjectStore) GetOTPRequestByToken(ctx context.Context, token string) (*OTPRequest, error) {
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
