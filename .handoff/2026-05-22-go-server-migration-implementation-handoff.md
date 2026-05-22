# Go Server Migration Implementation Handoff

## Next session focus

Run `implement-prd-issues` for the DnD Vote Go server migration.

Recommended skill:

- `implement-prd-issues`

## Current state

The repo has been prepared for local markdown issue tracking and the Go server migration has been specified.

Key artifacts:

- Agent skill configuration: `AGENTS.md`
- Local issue tracker docs: `docs/agents/issue-tracker.md`
- Triage label mapping: `docs/agents/triage-labels.md`
- Domain doc guidance: `docs/agents/domain.md`
- Parent PRD: `.scratch/go-server-migration/PRD.md`
- Implementation issues: `.scratch/go-server-migration/issues/`

The PRD is marked `ready-for-agent`. All seven implementation issues are marked `Status: ready-for-agent`.

## User decisions already confirmed

- Migrate the server from Python to Go.
- Preserve user-facing behavior and user stories.
- Existing SQLite database files do not need compatibility.
- Keep SQLite as the datastore for this migration.
- Keep the current plain HTML/CSS/JS frontend.
- Small Go dependencies are acceptable.
- Python may stay temporarily as a reference during implementation, but the final repo should have only the Go server.
- The migration should be behavior-preserving, not a feature expansion.

## Issue order

Implement in dependency order:

1. `.scratch/go-server-migration/issues/01-bootstrap-go-server-with-static-app-shell.md`
2. `.scratch/go-server-migration/issues/02-create-and-fetch-polls-end-to-end.md`
3. `.scratch/go-server-migration/issues/03-submit-and-replace-votes-end-to-end.md`
4. `.scratch/go-server-migration/issues/04-expose-ranked-pairs-results-end-to-end.md`
5. `.scratch/go-server-migration/issues/05-protect-and-delete-individual-votes.md`
6. `.scratch/go-server-migration/issues/06-protect-admin-bulk-clear.md`
7. `.scratch/go-server-migration/issues/07-cut-over-deployment-and-remove-python-server.md`

Issues 5 and 6 can proceed after their blockers are complete and do not depend on each other. Issue 7 should be last.

## Repo facts

Current backend:

- `main.py` imports and runs `server.py`.
- `server.py` is a Python standard-library HTTP server using SQLite.
- Existing static frontend files live at repo root: `index.html`, `vote.html`, `results.html`, `styles.css`, `shared.js`, `host.js`, `vote.js`, `results.js`.
- Existing verification script: `scripts/verify.py`.
- Deployment config: `railway.toml`.
- Existing Python behavior tests: `tests/test_server_behavior.py`.

Useful current behavior references:

- `README.txt` describes the app flows and operations.
- `tests/test_server_behavior.py` covers tally behavior, API behavior, validation, token secrecy, management delete, admin bulk clear, and oversized body behavior.
- The existing tests are useful as behavioral reference material, but the Go implementation does not need to preserve Python internals or old DB compatibility.

## Expected final state

- `go run ./cmd/dnd-vote-server` or equivalent runs the app.
- Go tests cover tally, API behavior, validation, auth, and store behavior.
- Existing frontend works against the Go API.
- Verification runs Go tests plus JavaScript syntax checks.
- Railway or equivalent deployment starts the Go server.
- Python runtime files and Python-specific tests/dependencies are removed after parity.

## Notes for the next agent

- Do not redesign the frontend.
- Do not add new product features while implementing the migration.
- Keep HTTP handlers thin and put durable behavior in typed Go modules.
- Keep ranked-pairs tally pure and well covered with table-driven tests.
- Keep SQLite access behind a small store boundary and use transactions for multi-step writes.
- Enforce destructive action authorization server-side; browser UI is not the security boundary.
