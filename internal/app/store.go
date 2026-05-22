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
	_, err := s.db.ExecContext(ctx, `
		PRAGMA foreign_keys = ON;

		CREATE TABLE IF NOT EXISTS polls (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			created_at TEXT NOT NULL,
			management_token TEXT NOT NULL
		);

		CREATE UNIQUE INDEX IF NOT EXISTS polls_management_token
		ON polls (management_token);

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
	`)
	return err
}
