Status: ready-for-agent

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Create the initial Go server foundation for DnD Vote. The app should be runnable with the Go server, serve the existing static pages, expose a minimal API surface suitable for subsequent slices, initialize a fresh SQLite database, and provide production-oriented server basics such as configuration, request limits, and HTTP timeouts.

This slice should establish the backend structure that later poll, vote, results, management, and admin behavior will build on. It should not port all existing behavior yet; it should make the Go runtime real, verifiable, and ready for vertical feature slices.

## Acceptance criteria

- [ ] A Go module exists with a server entrypoint that can be run locally.
- [ ] The Go server serves the existing host, vote, results, CSS, and JavaScript static files.
- [ ] The server reads host, port, data directory or database location, and admin-secret configuration from environment variables with sensible local defaults.
- [ ] The server opens or creates a fresh SQLite database and initializes the schema needed by upcoming slices.
- [ ] The HTTP server is configured with header, read, write, and idle timeouts.
- [ ] JSON API requests enforce a maximum request body size.
- [ ] A lightweight router and SQLite driver are chosen and documented through normal dependency metadata.
- [ ] The local verification command runs Go tests and JavaScript syntax checks.
- [ ] Existing frontend files remain plain HTML, CSS, and JavaScript.

## Blocked by

None - can start immediately
