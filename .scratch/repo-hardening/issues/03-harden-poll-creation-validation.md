---
title: Harden poll creation validation
status: completed
type: AFK
completed_by: codex/repo-hardening/03-poll-validation
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Harden poll creation so browser checks are not the only thing protecting the app. Direct API calls should only be able to create polls with a bounded title and at least two unique valid future candidate dates.

The host page should continue to feel simple, including a sample-poll action that always produces usable upcoming dates.

## Acceptance criteria

- [ ] Poll creation rejects invalid date strings with a clear JSON error.
- [ ] Poll creation rejects past candidate dates with a clear JSON error.
- [ ] Poll creation rejects oversized titles and oversized request bodies with clear JSON errors.
- [ ] Poll creation still accepts at least two unique valid upcoming dates.
- [ ] Duplicate candidate dates are normalized or rejected consistently and covered by tests.
- [ ] The sample-poll action generates current upcoming dates rather than fixed stale dates.
- [ ] Host-facing error messages remain understandable.
- [ ] The verification command from Issue 01 passes.

## Blocked by

- .scratch/repo-hardening/issues/01-establish-behavior-test-harness.md
