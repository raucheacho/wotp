package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Project represents a tenant in a multi-project wotp instance.
type Project struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	// SettingsJSON holds the project's OTP/messaging/whatsapp/webhooks/templates
	// settings as a JSON blob (see project.Settings). Stored as a blob rather
	// than dedicated columns since these fields are read-mostly and never
	// filtered on in SQL.
	SettingsJSON string    `json:"settings_json"`
	CreatedAt    time.Time `json:"created_at"`
}

// NumberStatus represents the pairing/connection state of a WhatsApp number.
type NumberStatus string

const (
	NumberStatusPending      NumberStatus = "pending"
	NumberStatusConnected    NumberStatus = "connected"
	NumberStatusDisconnected NumberStatus = "disconnected"
)

// Number represents the WhatsApp number (whatsmeow device) assigned to a
// project — at most one per project, see whatsapp.Pool.
type Number struct {
	JID       string       `json:"jid"`
	ProjectID string       `json:"project_id"`
	Phone     string       `json:"phone"`
	Status    NumberStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
}

// ControlStore is the data access interface for instance-wide (control plane)
// data: projects, API keys, and the WhatsApp number registry. Unlike
// ProjectStore (otps/messages/webhooks), there is exactly one ControlStore
// per wotp-core instance, and it must be resolvable before a project is known
// (it's what auth uses to map an apikey to a project).
type ControlStore interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProjectByID(ctx context.Context, id string) (*Project, error)
	GetProjectBySlug(ctx context.Context, slug string) (*Project, error)
	ListProjects(ctx context.Context) ([]Project, error)
	UpdateProjectSettings(ctx context.Context, id, settingsJSON string) error
	DeleteProject(ctx context.Context, id string) error

	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error)
	ListAPIKeysByProject(ctx context.Context, projectID string) ([]APIKey, error)
	DeleteAPIKeysByProjectAndTier(ctx context.Context, projectID, tier string) error

	UpsertNumber(ctx context.Context, n *Number) error
	ListNumbersByProject(ctx context.Context, projectID string) ([]Number, error)
	UpdateNumberStatus(ctx context.Context, jid string, status NumberStatus) error

	Close() error
}

// SQLiteControlStore is the SQLite implementation of ControlStore, backed by
// a single control.db file shared by every project on the instance.
type SQLiteControlStore struct {
	db     *sql.DB
	logger *slog.Logger
}

var controlMigrations = []Migration{
	{
		Version: 1,
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS projects (
				id            TEXT PRIMARY KEY,
				slug          TEXT NOT NULL UNIQUE,
				name          TEXT NOT NULL,
				settings_json TEXT NOT NULL DEFAULT '{}',
				created_at    DATETIME NOT NULL
			)`,
			// project_id has no FK constraint (unlike numbers.project_id below):
			// root-tier keys use the keys.RootProjectID sentinel, which never
			// corresponds to a real row in projects.
			`CREATE TABLE IF NOT EXISTS api_keys (
				id         TEXT PRIMARY KEY,
				project_id TEXT NOT NULL,
				key_hash   TEXT NOT NULL,
				key_prefix TEXT NOT NULL UNIQUE,
				tier       TEXT NOT NULL,
				created_at DATETIME NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_api_keys_project ON api_keys(project_id)`,
			// One row per project at most — see whatsapp.Pool, which now
			// refuses to pair a second device. No ordering column: with at
			// most one number, there's nothing to order.
			`CREATE TABLE IF NOT EXISTS numbers (
				jid          TEXT PRIMARY KEY,
				project_id   TEXT NOT NULL REFERENCES projects(id),
				phone        TEXT NOT NULL,
				status       TEXT NOT NULL DEFAULT 'pending',
				created_at   DATETIME NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_numbers_project ON numbers(project_id)`,
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

// --- Projects ---

func (s *SQLiteControlStore) CreateProject(ctx context.Context, p *Project) error {
	settingsJSON := p.SettingsJSON
	if settingsJSON == "" {
		settingsJSON = "{}"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, slug, name, settings_json, created_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Name, settingsJSON, p.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: create project: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) GetProjectByID(ctx context.Context, id string) (*Project, error) {
	return s.scanProject(s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, settings_json, created_at FROM projects WHERE id = ?`, id,
	))
}

func (s *SQLiteControlStore) GetProjectBySlug(ctx context.Context, slug string) (*Project, error) {
	return s.scanProject(s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, settings_json, created_at FROM projects WHERE slug = ?`, slug,
	))
}

func (s *SQLiteControlStore) scanProject(row *sql.Row) (*Project, error) {
	var p Project
	var createdAt string
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.SettingsJSON, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get project: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &p, nil
}

func (s *SQLiteControlStore) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, name, settings_json, created_at FROM projects ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.SettingsJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan project: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *SQLiteControlStore) UpdateProjectSettings(ctx context.Context, id, settingsJSON string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE projects SET settings_json = ? WHERE id = ?`, settingsJSON, id)
	if err != nil {
		return fmt.Errorf("sqlite: update project settings: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) DeleteProject(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete project: %w", err)
	}
	return nil
}

// --- API keys ---

func (s *SQLiteControlStore) CreateAPIKey(ctx context.Context, key *APIKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, project_id, key_hash, key_prefix, tier, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		key.ID, key.ProjectID, key.KeyHash, key.KeyPrefix, key.Tier, key.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: create api key: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, key_hash, key_prefix, tier, created_at FROM api_keys WHERE key_prefix = ?`, prefix,
	)

	var key APIKey
	var createdAt string
	err := row.Scan(&key.ID, &key.ProjectID, &key.KeyHash, &key.KeyPrefix, &key.Tier, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get api key: %w", err)
	}
	key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &key, nil
}

func (s *SQLiteControlStore) ListAPIKeysByProject(ctx context.Context, projectID string) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, key_hash, key_prefix, tier, created_at
		 FROM api_keys WHERE project_id = ? ORDER BY created_at`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list api keys by project: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var key APIKey
		var createdAt string
		if err := rows.Scan(&key.ID, &key.ProjectID, &key.KeyHash, &key.KeyPrefix, &key.Tier, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan api key: %w", err)
		}
		key.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *SQLiteControlStore) DeleteAPIKeysByProjectAndTier(ctx context.Context, projectID, tier string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE project_id = ? AND tier = ?`, projectID, tier,
	)
	if err != nil {
		return fmt.Errorf("sqlite: delete api keys by project and tier: %w", err)
	}
	return nil
}

// --- Numbers ---

func (s *SQLiteControlStore) UpsertNumber(ctx context.Context, n *Number) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO numbers (jid, project_id, phone, status, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(jid) DO UPDATE SET
			phone = excluded.phone,
			status = excluded.status`,
		n.JID, n.ProjectID, n.Phone, n.Status, n.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert number: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) ListNumbersByProject(ctx context.Context, projectID string) ([]Number, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT jid, project_id, phone, status, created_at
		 FROM numbers WHERE project_id = ? ORDER BY created_at`, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list numbers by project: %w", err)
	}
	defer rows.Close()

	var numbers []Number
	for rows.Next() {
		var n Number
		var createdAt string
		if err := rows.Scan(&n.JID, &n.ProjectID, &n.Phone, &n.Status, &createdAt); err != nil {
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

// Close closes the underlying database connection.
func (s *SQLiteControlStore) Close() error {
	return s.db.Close()
}
