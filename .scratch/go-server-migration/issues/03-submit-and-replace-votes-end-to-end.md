Status: ready-for-agent

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Implement the Go-backed voter ballot path end to end. A voter should be able to open the existing voter page, submit ranked available dates and unavailable dates, and edit a previous ballot by submitting again with the same voter name.

This slice should preserve the current user-facing ballot model and server validation rules while using Go types and transactional SQLite persistence.

## Acceptance criteria

- [ ] `POST /api/polls/{pollId}/votes` creates a vote for an existing poll.
- [ ] Submitting a vote again with the same voter name replaces the previous vote for that poll.
- [ ] Voter-name replacement is case-insensitive within a poll.
- [ ] Vote replacement removes old rankings and unavailability before saving the new ballot.
- [ ] Empty voter names, non-string voter names, oversized voter names, duplicate rankings, duplicate unavailable dates, contradictory ranked/unavailable dates, unknown ranked dates, and unknown unavailable dates return predictable JSON errors.
- [ ] A successful vote response returns the updated poll with the current vote list.
- [ ] Vote creation and replacement happen in a transaction.
- [ ] The existing static voter frontend can submit ballots without a frontend framework migration.
- [ ] API behavior is covered by Go HTTP tests using a temporary SQLite database.

## Blocked by

- .scratch/go-server-migration/issues/02-create-and-fetch-polls-end-to-end.md
