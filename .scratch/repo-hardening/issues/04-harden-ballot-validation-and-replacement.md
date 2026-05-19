---
title: Harden ballot validation and replacement
status: completed
type: AFK
completed_by: codex/repo-hardening/04-ballot-validation
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Make ballot submission robust when requests come directly to the API, not only from the vote page. Voters should be able to submit ranked available dates, mark unavailable dates, and replace their existing same-name ballot, while malformed ballots are rejected with clear errors.

The slice should keep same-name replacement as an intentional app behavior and make it clear to voters when they are editing an existing ballot.

## Acceptance criteria

- [ ] Ballot submission rejects duplicate ranked dates.
- [ ] Ballot submission rejects duplicate unavailable dates.
- [ ] Ballot submission rejects unknown date ids.
- [ ] Ballot submission rejects a date that is both ranked and marked unavailable.
- [ ] Ballot submission rejects empty voter names and oversized voter names.
- [ ] A same-name voter replacement leaves only one current ballot for that poll.
- [ ] The vote page clearly indicates when an existing ballot has been loaded for editing.
- [ ] The verification command from Issue 01 passes.

## Blocked by

- .scratch/repo-hardening/issues/01-establish-behavior-test-harness.md
