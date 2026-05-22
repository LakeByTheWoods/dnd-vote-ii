---
title: Migrate DnD Vote Server to Go
status: ready-for-agent
source: conversation
---

## Problem Statement

The DnD Vote app currently works as a small Python standard-library web server with a plain HTML, CSS, and JavaScript frontend. That has been enough for the current app, but the server is expected to become easier to maintain, extend, and harden over time.

The current server concentrates HTTP handling, validation, persistence, token checks, static file serving, and ranked-pairs tallying in one Python module. This makes the behavior inspectable, but it also leaves important server concerns depending on dynamic data shapes and a minimal HTTP server foundation.

As the app owner, I want the server migrated to Go so that the backend has stronger type boundaries, clearer package seams, better production HTTP primitives, and a more maintainable foundation for future poll-management and security work, while the user-facing DnD scheduling experience stays the same.

## Solution

Replace the Python backend with a Go backend that preserves the existing user-facing app behavior.

The Go server should continue to serve the current plain static frontend and expose API behavior that supports the same host, voter, results, management, and admin flows. The migration should be behavior-preserving from a user-story perspective, not a product redesign.

The implementation may redesign the SQLite schema because existing database files do not need to remain compatible. SQLite should remain the datastore for this slice. Small production-oriented Go dependencies are acceptable when they improve maintainability, especially for routing and SQLite access.

The final repository should have one supported server implementation: Go. The Python implementation may stay temporarily as a reference while parity is being built, but it should be removed once the Go server and verification path cover the intended behavior.

## User Stories

1. As a host, I want to open the host page and create a poll with a title and candidate dates, so that I can organize a DnD session date.
2. As a host, I want poll creation to return a voter link, results link, and private management link, so that I can share the right links with the right people.
3. As a host, I want the management link to include a private per-poll management token, so that management actions are not available from the public voter or results links.
4. As a host, I want the app to reject a blank poll title, so that every poll has a meaningful name.
5. As a host, I want the app to reject overly long poll titles, so that abusive or accidental input cannot distort storage or UI behavior.
6. As a host, I want the app to reject invalid date strings, so that candidate dates are always usable calendar dates.
7. As a host, I want the app to reject past candidate dates, so that newly created polls are useful for future scheduling.
8. As a host, I want duplicate candidate dates to be normalized or rejected predictably, so that a poll cannot count the same date as multiple choices.
9. As a host, I want each poll to require at least two unique candidate dates, so that the poll represents a real choice.
10. As a voter, I want to open a public voter link, so that I can submit my availability without needing an account.
11. As a voter, I want to enter my name, so that my ballot can be identified in the poll results.
12. As a voter, I want to rank the dates I can attend, so that my preferences are counted.
13. As a voter, I want to mark dates I cannot attend as unavailable, so that impossible dates are disqualified.
14. As a voter, I want to change ranking order with the existing frontend controls, so that the migration does not change how voting feels.
15. As a voter, I want to submit a ballot with the same name again, so that I can edit my previous vote instead of creating a duplicate.
16. As a voter, I want replacement by same voter name to remain case-insensitive, so that small casing changes do not create multiple ballots.
17. As a voter, I want the server to reject an empty voter name, so that anonymous duplicate-prone ballots are not accepted.
18. As a voter, I want the server to reject overly long voter names, so that abusive or accidental input cannot distort storage or UI behavior.
19. As a voter, I want the server to reject duplicate ranked dates, so that one ballot cannot overcount a date.
20. As a voter, I want the server to reject duplicate unavailable dates, so that one ballot cannot overstate unavailability.
21. As a voter, I want the server to reject a ballot that both ranks and marks the same date unavailable, so that my ballot cannot contradict itself.
22. As a voter, I want the server to reject unknown date identifiers, so that a ballot cannot reference dates outside its poll.
23. As a voter, I want clear JSON error messages when a ballot is invalid, so that the frontend can show predictable failures.
24. As a results viewer, I want to open the public results link, so that I can see submitted ballots and the current winning date.
25. As a results viewer, I want results to continue using the ranked-pairs head-to-head tally, so that the winner is selected by the same rule as before.
26. As a results viewer, I want unavailable dates to be disqualified before tallying, so that impossible dates cannot win.
27. As a results viewer, I want to see who marked a date unavailable, so that disqualifications are transparent.
28. As a results viewer, I want ties and all-disqualified states to be handled clearly, so that edge cases do not appear broken.
29. As a results viewer, I want each eligible date's head-to-head record to remain available, so that the outcome can be explained.
30. As a host, I want to delete an individual vote from the management view, so that I can clean up bad or duplicate submissions.
31. As a host, I want individual vote deletion to require the correct management token, so that public users cannot delete other people's votes.
32. As a host, I want a missing management token to be rejected, so that the security boundary is enforced server-side.
33. As a host, I want an invalid or wrong-poll management token to be rejected, so that management authority cannot leak across polls.
34. As an operator, I want bulk clearing all polls to require the configured admin secret, so that public users cannot erase deployment data.
35. As an operator, I want a missing or invalid admin secret to be rejected, so that destructive operations have a clear server-side check.
36. As an operator, I want deployment to start the Go server directly, so that the production runtime no longer depends on Python.
37. As an operator, I want the database location to remain configurable for mounted persistent storage, so that Railway or similar deployments can persist poll data.
38. As an operator, I want HTTP server timeouts and request body limits, so that the app has better baseline resistance to slow or oversized requests.
39. As a developer, I want poll, vote, date, tally, and API response data to be represented by Go types, so that shape drift is caught earlier.
40. As a developer, I want HTTP handlers to stay thin, so that route behavior is easy to review and business rules live in testable modules.
41. As a developer, I want SQLite access behind a small store interface, so that persistence behavior can be tested with temporary databases.
42. As a developer, I want ranked-pairs tallying isolated in a pure module, so that result behavior can be tested without running the server.
43. As a developer, I want token checks isolated from route handlers, so that management and admin authorization are easy to audit.
44. As a developer, I want the verification command to run Go tests and JavaScript syntax checks, so that future changes can be checked locally.
45. As a developer, I want the Python server removed after Go parity is verified, so that the repo has one supported backend.
46. As a developer, I want the migration to avoid frontend framework changes, so that backend behavior remains the clear completion target.

## Implementation Decisions

- Replace the Python server with a Go server as the only supported backend by the end of this PRD.
- Preserve user-facing behavior and user stories rather than preserving the exact Python implementation or existing database files.
- Keep the current plain HTML, CSS, and JavaScript frontend for this migration slice.
- Keep SQLite as the datastore.
- Existing SQLite database files do not need to be supported. The Go implementation may create a cleaner schema from scratch.
- Allow small production-oriented Go dependencies when they improve maintainability. A lightweight router and a SQLite driver are acceptable.
- Prefer a router that keeps route declarations explicit and supports middleware cleanly.
- Prefer a SQLite driver choice that is documented with its build tradeoffs. A no-CGO driver is attractive for cross-platform simplicity; a CGO driver is acceptable if the implementation documents the deployment expectation.
- Configure the HTTP server with production-oriented basics, including header, read, write, and idle timeouts.
- Enforce a maximum request body size for JSON API requests.
- Keep static file serving in the Go server so the app remains a single deployable web service.
- Keep API routes close enough to the current frontend contract that little or no frontend JavaScript has to change.
- Preserve JSON error responses with clear user-facing messages for validation and authorization failures.
- Generate per-poll management tokens with cryptographically secure randomness.
- Compare management and admin secrets using constant-time comparison.
- Read the admin secret from the deployment environment.
- Read the data directory or database location from the deployment environment so mounted storage remains possible.
- Keep poll creation public unless a later PRD introduces host accounts or broader authentication.
- Keep voter and results links public.
- Keep destructive management actions protected by server-side checks. Browser UI visibility must not be the security boundary.
- Build a configuration module responsible for environment variables, defaults, and startup validation.
- Build an HTTP API module responsible for routing, JSON decoding, response encoding, request limits, and mapping domain errors to HTTP statuses.
- Build an auth module responsible for management-token and admin-secret checks.
- Build a store module responsible for SQLite schema creation, transactions, and persistence queries.
- Build a tally module responsible for ranked-pairs calculation from an in-memory poll representation.
- Keep the tally module pure: it should not know about HTTP, SQLite, environment variables, or static files.
- Keep store methods oriented around domain operations such as creating polls, fetching polls, replacing votes, deleting votes, listing poll summaries, and clearing polls.
- Use explicit transactions for multi-step writes such as poll creation, vote replacement, vote deletion, and bulk clearing.
- Preserve case-insensitive voter-name replacement within a poll.
- Preserve the result rule that any date marked unavailable by any voter is disqualified before ranked-pairs tallying.
- Preserve the result output concepts: ordered tallied dates, rank, record, disqualification status, unavailable-by names, and winner.
- Update deployment configuration to start the Go server once parity is complete.
- Update documentation so local run, verification, deployment, data storage, and management-token behavior describe the Go server.
- Remove Python runtime files, Python tests, and Python-specific deployment assumptions after the Go verification path passes.

## Testing Decisions

- Tests should focus on external behavior and stable domain behavior, not private implementation details.
- The ranked-pairs tally module should have table-driven tests covering ordinary head-to-head winners, cycle-skipping, ties, one eligible date, all dates disqualified, and unavailability disqualification.
- API behavior tests should cover poll creation, fetching a poll, listing poll summaries, submitting a vote, replacing a vote by voter name, fetching results, deleting one vote, and clearing all polls.
- Validation tests should cover missing title, oversized title, invalid date type, invalid date format, past dates, blank dates, too few unique dates, non-string voter name, missing voter name, oversized voter name, duplicate rankings, duplicate unavailable dates, contradictory ranked/unavailable dates, unknown ranked dates, and unknown unavailable dates.
- Authorization tests should prove that individual vote deletion rejects missing, invalid, and wrong-poll management tokens.
- Authorization tests should prove that bulk clearing rejects missing and invalid admin secrets.
- Security-oriented tests should prove management tokens are only returned to poll creation responses and are not leaked through fetch-poll or results responses.
- Request-limit tests should prove oversized JSON bodies are rejected predictably.
- Store tests should use temporary SQLite databases and should verify persisted data through public store operations rather than by coupling every test to raw table layout.
- HTTP tests should exercise the real Go router and handlers with an in-process test server.
- JavaScript syntax checks should remain part of the verification path while the frontend stays plain JavaScript.
- The verification command should run Go tests and JavaScript checks in one documented local command.
- Prior art in this repo includes the existing Python behavior tests, which define the current important scenarios and should guide equivalent Go tests.

## Out of Scope

- Supporting existing SQLite database files created by the Python server.
- Changing the plain static frontend to a framework.
- Redesigning the host, vote, or results pages.
- Adding user accounts.
- Adding full authentication or authorization beyond the existing management-token and admin-secret model.
- Real-time results updates.
- Email, SMS, Discord, calendar, or notification integrations.
- Multi-host organizations or role-based permissions.
- Complex audit logging.
- Changing the voting rule away from ranked-pairs head-to-head tallying.
- Adding new poll types, recurring sessions, comments, or attendance notes.
- Supporting multiple databases.
- Building a separate API service and static asset host.

## Further Notes

- This PRD is a migration PRD, not a feature expansion PRD.
- The frontend should remain usable throughout the migration, but Python and Go do not need to coexist as supported runtimes after parity is complete.
- Keeping the API contract close to the current frontend contract is a means to reduce migration scope, not a requirement to preserve every internal implementation detail.
- The most important deep modules are tally and store: tally should make result behavior easy to trust, and store should make persistence and transactions easy to audit.
- Security improvement comes from clearer boundaries, stronger server defaults, typed request handling, request limits, token isolation, and explicit authorization checks. Go does not make the app secure automatically.
