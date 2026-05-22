Status: completed

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Implement host management for deleting an individual vote through the Go API. The management action must be enforced server-side with the correct per-poll management token, not by relying on the browser to hide controls.

This slice should preserve the current management-link behavior while isolating token checks so destructive route authorization is easy to audit.

## Acceptance criteria

- [ ] `DELETE /api/polls/{pollId}/votes/{voteId}` deletes the target vote only when the correct management token is provided.
- [ ] Missing management tokens return a predictable forbidden JSON error.
- [ ] Invalid management tokens return a predictable forbidden JSON error.
- [ ] A valid management token for a different poll is rejected.
- [ ] A successful delete returns the updated poll.
- [ ] Deleting a vote does not affect other votes for the poll.
- [ ] Vote deletion happens in a transaction.
- [ ] Management-token comparison uses constant-time comparison.
- [ ] Authorization and deletion behavior are covered by Go HTTP tests using a temporary SQLite database.

## Blocked by

- .scratch/go-server-migration/issues/03-submit-and-replace-votes-end-to-end.md

## Comments

- Completed on branch `codex/go-server-migration/05-delete-individual-votes` and merged into `codex/go-server-migration-prd`.
- Acceptance review: approved.
- Code-quality review: approved.
- Verification on PRD branch: `$env:PATH='C:\Program Files\Go\bin;' + $env:PATH; python scripts\verify.py` passed.
