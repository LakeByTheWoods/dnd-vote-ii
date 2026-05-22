Status: ready-for-agent

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Implement the operator bulk-clear path in the Go API. Clearing all polls should remain possible for deployment maintenance, but it must require the configured admin secret and reject public callers predictably.

This slice should keep the admin-secret boundary isolated from ordinary host and voter behavior.

## Acceptance criteria

- [ ] `DELETE /api/polls` clears all polls only when the correct admin secret is provided.
- [ ] Missing admin secrets return a predictable forbidden JSON error.
- [ ] Invalid admin secrets return a predictable forbidden JSON error.
- [ ] A successful bulk clear returns a simple success payload.
- [ ] Bulk clear removes polls and dependent poll data.
- [ ] Admin-secret comparison uses constant-time comparison.
- [ ] The admin secret is read from server configuration, not hard-coded.
- [ ] Bulk clear behavior is covered by Go HTTP tests using a temporary SQLite database.

## Blocked by

- .scratch/go-server-migration/issues/02-create-and-fetch-polls-end-to-end.md
