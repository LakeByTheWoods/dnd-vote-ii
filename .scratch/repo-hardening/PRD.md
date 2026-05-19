---
title: DnD Vote Repository Hardening
status: ready-for-agent
source: review
---

## Problem Statement

The DnD Vote app works as a small cross-device scheduling tool, but the repo is hard to trust and hard to maintain for someone who is not already comfortable with web or Python development.

The main risks are practical rather than cosmetic. A public deployment exposes unauthenticated destructive actions, the implemented vote tally does not match the scoring rule documented in the README, server-side validation is thin, mobile voting depends on drag-and-drop, and there are no automated tests around the behavior that decides the final date.

As the app owner, I need the repo to make its actual behavior explicit, protect shared poll data from casual deletion, and have enough tests and structure that future changes are understandable and low-risk.

## Solution

Harden the app around the behaviors that matter to a DnD date poll:

- Poll creation should accept only valid candidate dates and bounded text input.
- Voting should preserve the intended preferential ballot model and reject malformed ballots.
- Result calculation should use one clearly documented tallying rule.
- Destructive actions should be protected so public voters cannot clear polls or delete other people's votes by accident or by direct API call.
- Mobile and keyboard users should be able to rank dates without relying only on native drag-and-drop.
- The core poll, vote, and result behavior should be covered by automated tests that can run locally before deployment.

The implementation should keep the app simple: Python standard library server, SQLite persistence, and static HTML/CSS/JavaScript. Any new structure should extract deep, testable modules rather than introduce a large framework.

## User Stories

1. As the app owner, I want the README to describe the same voting rule the app actually uses, so that I can explain results without reading code.
2. As the app owner, I want the result page labels to match the tallying rule, so that voters understand why a date won.
3. As a host, I want a public voter to be unable to clear every poll, so that shared poll data is not lost.
4. As a host, I want a public voter to be unable to delete someone else's vote without authorization, so that results remain trustworthy.
5. As a host, I want any intentionally destructive action to require host-level authority, so that management controls are separate from voter controls.
6. As a host, I want clear error messages when poll creation fails, so that I can fix bad input quickly.
7. As a host, I want the server to reject invalid date strings, so that results pages do not show broken dates.
8. As a host, I want the server to reject past candidate dates, so that stale sample data or direct API calls cannot create unusable polls.
9. As a host, I want title length limits, so that an abusive request cannot create oversized database records or distorted UI.
10. As a host, I want each poll to have at least two unique valid dates, so that every poll can produce a meaningful choice.
11. As a host, I want the sample poll dates to stay current or be generated relative to today, so that the sample is always usable.
12. As a voter, I want to submit my ranked available dates, so that my preferences are counted.
13. As a voter, I want to mark dates as unavailable, so that dates I cannot attend are disqualified.
14. As a voter, I want the server to reject a ballot that ranks and marks the same date unavailable, so that my ballot cannot contradict itself.
15. As a voter, I want the server to reject duplicate ranked dates, so that my ballot cannot overcount a date.
16. As a voter, I want the server to reject unknown date ids, so that ballots cannot reference dates outside the poll.
17. As a voter, I want to edit my existing vote by using the same name, so that I can correct my ballot without creating a duplicate.
18. As a voter, I want mobile-friendly ranking controls, so that I can vote from my phone.
19. As a keyboard user, I want non-drag ranking controls, so that I can reorder dates without a mouse.
20. As a voter, I want confirmation before replacing my existing ballot, so that I understand when I am updating a previous vote.
21. As a voter, I want the results page to show unavailable dates and who marked them unavailable, so that disqualifications are transparent.
22. As a voter, I want the results page to show the winning eligible date clearly, so that I can quickly see the likely session date.
23. As a voter, I want the app to handle no-vote and all-disqualified states clearly, so that empty or impossible polls do not look broken.
24. As a developer, I want the tallying algorithm isolated behind a small interface, so that it can be tested without running the web server.
25. As a developer, I want input validation isolated behind a small interface, so that API handlers stay readable.
26. As a developer, I want database access grouped behind clear functions, so that persistence behavior can be tested with temporary SQLite files.
27. As a developer, I want HTTP handlers to be thin, so that route behavior is easier to inspect and less likely to hide business rules.
28. As a developer, I want tests for poll creation, vote replacement, unavailability disqualification, and winner calculation, so that core behavior does not regress.
29. As a developer, I want tests for rejected malformed requests, so that direct API calls cannot bypass browser checks.
30. As a developer, I want a documented local verification command, so that a non-web developer can run checks before deploying.
31. As a Railway deployer, I want the app to keep using the mounted SQLite volume, so that production data persists across deploys.
32. As a Railway deployer, I want destructive management actions to remain possible for the host, so that bad polls can still be cleaned up deliberately.

## Implementation Decisions

- Keep the app on the current Python standard library server, SQLite database, and static frontend stack.
- Resolve the tallying-rule mismatch before changing result behavior. The product must choose one rule: either implement the README's geometric scoring or update the docs and UI to explicitly endorse the current head-to-head ranked-pairs behavior.
- Prefer a `tally` deep module for result calculation. Its input should be a plain poll data structure and its output should be ordered date results plus the winner.
- Prefer a `validation` deep module for poll and ballot validation. It should enforce date format, date range, uniqueness, unknown ids, duplicate rankings, title length, voter name length, and request-size constraints.
- Keep SQLite persistence in a small storage boundary. It should continue to support temporary database paths for tests and `DND_VOTE_DATA_DIR` for Railway.
- Keep HTTP route handlers focused on parsing, calling validation/storage/tally code, and returning JSON responses.
- Protect destructive actions. The minimum acceptable design is a host-only secret or per-poll management token checked server-side before clearing polls or deleting votes.
- Do not rely on hiding buttons in the browser for security. Server routes must enforce the destructive-action checks.
- Keep voter-facing routes public by default. Creating polls may remain public if this is intended for a private friend-group link, but deleting data should not.
- Add non-drag ranking controls to the ballot, such as up/down buttons for each date. Drag-and-drop may remain as an enhancement.
- Generate sample dates relative to the current date rather than hard-coding calendar dates.
- Keep API error responses as JSON with clear user-facing messages.
- Avoid introducing a web framework unless the standard library server becomes a real blocker.

## Testing Decisions

- Tests should focus on external behavior: API responses, persisted poll/vote state, and tally results. Do not test private implementation details such as local variable names or exact helper decomposition.
- Add Python tests for the tally module using small in-memory poll fixtures.
- Add Python tests for validation using direct function calls and representative invalid payloads.
- Add Python tests for storage behavior using a temporary SQLite database directory.
- Add HTTP-level tests for create poll, fetch poll, submit vote, replace vote, fetch results, delete vote authorization, and clear-polls authorization.
- Add regression tests for the selected tally rule, including ties, all-disqualified polls, one eligible date, and repeated voter-name replacement.
- Add tests proving direct API calls cannot create malformed dates or oversized text fields.
- Add a lightweight JavaScript syntax check command to the documented verification path.
- Prior art in this repo is currently manual smoke testing only, so the first implementation should establish the test harness.

## Out of Scope

- User accounts and full authentication.
- A large frontend framework migration.
- A large Python web framework migration.
- Real-time live result updates.
- Email, SMS, or calendar integrations.
- Multi-host organizations or permission roles.
- Complex audit logs.
- Database migration tooling beyond what is needed for the current SQLite schema.

## Further Notes

- The current local database lives under `data/dnd_vote.db` and is ignored by git.
- The deployment documentation expects Railway to mount a volume at `/data` and set `DND_VOTE_DATA_DIR=/data`.
- The current app can be smoke-tested locally with `python main.py` and opened at `http://localhost:8000`.
- The current syntax checks that passed during review were `python -m py_compile server.py main.py` and `node --check` for the JavaScript files.
- The review confirmed over HTTP that unauthenticated `DELETE /api/polls` currently clears all polls.
