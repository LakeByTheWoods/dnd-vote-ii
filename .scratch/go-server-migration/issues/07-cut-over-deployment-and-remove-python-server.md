Status: ready-for-agent

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Cut the project over to the Go server as the only supported backend. Update local run instructions, verification, and deployment configuration so the app no longer depends on Python, then remove Python runtime files and tests that are replaced by the Go implementation.

This slice should happen only after the Go server covers poll creation, voting, results, management deletion, and admin bulk clear behavior.

## Acceptance criteria

- [ ] Deployment configuration starts the Go server directly.
- [ ] README instructions describe installing, running, verifying, and deploying the Go server.
- [ ] Data storage documentation describes the Go server's SQLite location and persistent-volume configuration.
- [ ] Management-token and admin-secret documentation matches the Go implementation.
- [ ] The verification command runs Go tests and JavaScript syntax checks.
- [ ] Python server runtime files are removed.
- [ ] Python-specific tests and dependency files are removed or replaced by Go equivalents.
- [ ] The repo has one supported backend implementation: Go.
- [ ] The final verification command passes.

## Blocked by

- .scratch/go-server-migration/issues/01-bootstrap-go-server-with-static-app-shell.md
- .scratch/go-server-migration/issues/02-create-and-fetch-polls-end-to-end.md
- .scratch/go-server-migration/issues/03-submit-and-replace-votes-end-to-end.md
- .scratch/go-server-migration/issues/04-expose-ranked-pairs-results-end-to-end.md
- .scratch/go-server-migration/issues/05-protect-and-delete-individual-votes.md
- .scratch/go-server-migration/issues/06-protect-admin-bulk-clear.md
