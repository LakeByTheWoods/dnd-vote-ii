CREATE TABLE polls (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  created_at TEXT NOT NULL,
  management_token TEXT NOT NULL
);
CREATE UNIQUE INDEX polls_management_token ON polls(management_token);
CREATE TABLE poll_dates (
  id TEXT PRIMARY KEY,
  poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  date_value TEXT NOT NULL
);
CREATE INDEX poll_dates_poll ON poll_dates(poll_id);
CREATE TABLE votes (
  id TEXT PRIMARY KEY,
  poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
  voter_name TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX votes_poll_voter_name ON votes(poll_id, voter_name COLLATE NOCASE);
CREATE TABLE vote_rankings (
  vote_id TEXT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
  date_id TEXT NOT NULL REFERENCES poll_dates(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  PRIMARY KEY(vote_id, position)
);
CREATE TABLE vote_unavailability (
  vote_id TEXT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
  date_id TEXT NOT NULL REFERENCES poll_dates(id) ON DELETE CASCADE,
  PRIMARY KEY(vote_id, date_id)
);
