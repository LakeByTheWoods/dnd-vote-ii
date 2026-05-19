---
title: Establish behavior test harness
status: completed
type: AFK
completed_by: codex/repo-hardening/01-test-harness
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Create the first repeatable verification path for the DnD Vote app. The test harness should exercise the server's externally visible behavior using isolated temporary data, so future changes to poll creation, voting, persistence, and results can be made with confidence.

The slice should leave the app runnable exactly as before while adding a documented command that checks Python syntax, JavaScript syntax, and core Python behavior tests.

## Acceptance criteria

- [ ] A local test command runs without requiring a production database or Railway environment.
- [ ] Tests use temporary SQLite storage and do not read, mutate, or depend on the developer's local poll database.
- [ ] Tests cover creating a poll, fetching a poll, submitting a vote, replacing a same-name vote, fetching results, and basic disqualification behavior.
- [ ] Tests prove direct API-style validation failures return predictable errors where the current behavior already supports them.
- [ ] The documented verification path includes Python behavior tests plus JavaScript syntax checks.
- [ ] Existing local run behavior remains `python main.py`.

## Blocked by

None - can start immediately
