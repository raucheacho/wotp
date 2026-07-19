package store

import (
	"database/sql"
	"fmt"
)

// Migration is a single forward-only schema change, applied once and tracked
// via SQLite's PRAGMA user_version. Versions must be sequential starting at 1.
type Migration struct {
	Version int
	SQL     []string
}

// applyMigrations brings db up to the highest version present in migrations.
// Each migration runs inside its own transaction; the version is only
// advanced if every statement in that migration succeeds.
func applyMigrations(db *sql.DB, migrations []Migration) error {
	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("store: read schema version: %w", err)
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("store: begin migration %d: %w", m.Version, err)
		}

		for _, stmt := range m.SQL {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("store: migration %d: %w\nSQL: %s", m.Version, err, stmt)
			}
		}

		// PRAGMA doesn't accept bound parameters in the sqlite3 driver;
		// m.Version is a compile-time constant from our own migration list, not user input.
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.Version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: set schema version %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %d: %w", m.Version, err)
		}
		current = m.Version
	}

	return nil
}
