package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(ctx context.Context, databasePath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.initializeSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initializeSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		PRAGMA foreign_keys = ON;

		CREATE TABLE IF NOT EXISTS polls (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			created_at TEXT NOT NULL,
			management_token TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS poll_dates (
			id TEXT PRIMARY KEY,
			poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
			date_value TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS votes (
			id TEXT PRIMARY KEY,
			poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
			voter_name TEXT NOT NULL,
			created_at TEXT NOT NULL
		);

		CREATE UNIQUE INDEX IF NOT EXISTS votes_poll_voter_name
		ON votes (poll_id, voter_name COLLATE NOCASE);

		CREATE TABLE IF NOT EXISTS vote_rankings (
			vote_id TEXT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
			date_id TEXT NOT NULL REFERENCES poll_dates(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			PRIMARY KEY (vote_id, position)
		);

		CREATE TABLE IF NOT EXISTS vote_unavailability (
			vote_id TEXT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
			date_id TEXT NOT NULL REFERENCES poll_dates(id) ON DELETE CASCADE,
			PRIMARY KEY (vote_id, date_id)
		);
	`); err != nil {
		return err
	}

	if err := s.ensurePollManagementTokens(ctx); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS polls_management_token
		ON polls (management_token);
	`)
	return err
}

func (s *Store) ensurePollManagementTokens(ctx context.Context) error {
	hasTokenColumn, err := s.pollsHasColumn(ctx, "management_token")
	if err != nil {
		return err
	}
	if !hasTokenColumn {
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE polls ADD COLUMN management_token TEXT"); err != nil {
			return err
		}
	}

	rows, err := s.db.QueryContext(ctx, "SELECT id FROM polls WHERE management_token IS NULL OR management_token = ''")
	if err != nil {
		return err
	}
	defer rows.Close()

	var pollIDs []string
	for rows.Next() {
		var pollID string
		if err := rows.Scan(&pollID); err != nil {
			return err
		}
		pollIDs = append(pollIDs, pollID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, pollID := range pollIDs {
		token, err := generateManagementToken()
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, "UPDATE polls SET management_token = ? WHERE id = ?", token, pollID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) pollsHasColumn(ctx context.Context, columnName string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info(polls)")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}
