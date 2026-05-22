Status: ready-for-agent

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Implement the Go-backed host poll creation and poll retrieval path end to end. A host should be able to create a poll from the existing frontend, receive the same user-facing link information, and then fetch that poll through the API. Poll summaries should also be listable for existing app workflows.

This slice should include the domain types, validation, SQLite persistence, management-token generation, and HTTP behavior needed for polls, while keeping the API close enough that the current frontend needs little or no change.

## Acceptance criteria

- [ ] `POST /api/polls` creates a poll with a title and at least two unique future candidate dates.
- [ ] Poll creation returns the poll plus a private management token.
- [ ] The management token is generated with cryptographically secure randomness.
- [ ] Fetched poll and results-facing poll payloads do not leak the management token.
- [ ] `GET /api/polls/{pollId}` returns the created poll with dates and no votes.
- [ ] `GET /api/polls` returns poll summaries with date and vote counts.
- [ ] Blank titles, oversized titles, invalid date values, past dates, invalid date payload types, and too few unique dates return predictable JSON errors.
- [ ] Poll creation and date inserts happen in a transaction.
- [ ] API behavior is covered by Go HTTP tests using a temporary SQLite database.

## Blocked by

- .scratch/go-server-migration/issues/01-bootstrap-go-server-with-static-app-shell.md
