package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// NumberStatus represents the pairing/connection state of a WhatsApp number.
type NumberStatus string

const (
	NumberStatusPending      NumberStatus = "pending"
	NumberStatusConnected    NumberStatus = "connected"
	NumberStatusDisconnected NumberStatus = "disconnected"
)

// Number represents the WhatsApp number (whatsmeow device) currently paired
// with this instance — at most one, see whatsapp.Pool.
type Number struct {
	JID       string       `json:"jid"`
	Phone     string       `json:"phone"`
	Status    NumberStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
}

// ControlStore is the data access interface for instance-wide (control
// plane) data: API keys, the paired WhatsApp number, and settings. Unlike
// ProjectStore (otps/messages/webhooks/conversations), there is exactly one
// ControlStore per wotp-core instance, and it must be resolvable before the
// rest of the runtime is built (it's what auth uses to validate an apikey,
// and what holds the instance's settings blob).
type ControlStore interface {
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error)
	ListAPIKeys(ctx context.Context) ([]APIKey, error)
	DeleteAPIKeysByTier(ctx context.Context, tier string) error

	UpsertNumber(ctx context.Context, n *Number) error
	ListNumbers(ctx context.Context) ([]Number, error)
	UpdateNumberStatus(ctx context.Context, jid string, status NumberStatus) error

	// GetSettings returns the instance's settings JSON blob, or "" if none
	// has been saved yet (callers fall back to defaults — see
	// project.DefaultSettings).
	GetSettings(ctx context.Context) (string, error)
	UpdateSettings(ctx context.Context, settingsJSON string) error

	Close() error
}

// SQLiteControlStore is the SQLite implementation of ControlStore, backed by
// a single control.db file for the instance.
type SQLiteControlStore struct {
	db     *sql.DB
	logger *slog.Logger
}

var controlMigrations = []Migration{
	{
		Version: 1,
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS api_keys (
				id         TEXT PRIMARY KEY,
				key_hash   TEXT NOT NULL,
				key_prefix TEXT NOT NULL UNIQUE,
				tier       TEXT NOT NULL,
				created_at DATETIME NOT NULL
			)`,
			// One row at most — see whatsapp.Pool, which refuses to pair a
			// second device. No project_id: this instance has exactly one
			// data plane.
			`CREATE TABLE IF NOT EXISTS numbers (
				jid          TEXT PRIMARY KEY,
				phone        TEXT NOT NULL,
				status       TEXT NOT NULL DEFAULT 'pending',
				created_at   DATETIME NOT NULL
			)`,
			// Singleton row (id always 1) holding the instance's
			// OTP/messaging/whatsapp/webhooks/templates/cloud settings — see
			// project.Settings.
			`CREATE TABLE IF NOT EXISTS instance_settings (
				id            INTEGER PRIMARY KEY CHECK (id = 1),
				settings_json TEXT NOT NULL
			)`,
		},
	},
}

// NewSQLiteControlStore opens (or creates) the control.db database at the
// given path and runs migrations.
func NewSQLiteControlStore(dbPath string, logger *slog.Logger) (*SQLiteControlStore, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1) // SQLite works best with a single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := applyMigrations(db, controlMigrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate control store: %w", err)
	}

	logger.Info("control store initialized", "path", dbPath)
	return &SQLiteControlStore{db: db, logger: logger}, nil
}

// --- API keys ---

func (s *SQLiteControlStore) CreateAPIKey(ctx context.Context, key *APIKey) error {
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

func (s *SQLiteControlStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error) {
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

func (s *SQLiteControlStore) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
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

func (s *SQLiteControlStore) DeleteAPIKeysByTier(ctx context.Context, tier string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE tier = ?`, tier)
	if err != nil {
		return fmt.Errorf("sqlite: delete api keys by tier: %w", err)
	}
	return nil
}

// --- Numbers ---

func (s *SQLiteControlStore) UpsertNumber(ctx context.Context, n *Number) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO numbers (jid, phone, status, created_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(jid) DO UPDATE SET
			phone = excluded.phone,
			status = excluded.status`,
		n.JID, n.Phone, n.Status, n.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert number: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) ListNumbers(ctx context.Context) ([]Number, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT jid, phone, status, created_at FROM numbers ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list numbers: %w", err)
	}
	defer rows.Close()

	var numbers []Number
	for rows.Next() {
		var n Number
		var createdAt string
		if err := rows.Scan(&n.JID, &n.Phone, &n.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan number: %w", err)
		}
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		numbers = append(numbers, n)
	}
	return numbers, rows.Err()
}

func (s *SQLiteControlStore) UpdateNumberStatus(ctx context.Context, jid string, status NumberStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE numbers SET status = ? WHERE jid = ?`, status, jid,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update number status: %w", err)
	}
	return nil
}

// --- Settings ---

func (s *SQLiteControlStore) GetSettings(ctx context.Context) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT settings_json FROM instance_settings WHERE id = 1`)
	var settingsJSON string
	err := row.Scan(&settingsJSON)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sqlite: get settings: %w", err)
	}
	return settingsJSON, nil
}

func (s *SQLiteControlStore) UpdateSettings(ctx context.Context, settingsJSON string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO instance_settings (id, settings_json) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET settings_json = excluded.settings_json`,
		settingsJSON,
	)
	if err != nil {
		return fmt.Errorf("sqlite: update settings: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteControlStore) Close() error {
	return s.db.Close()
}
