---
title: Protect destructive management actions
status: ready-for-agent
type: AFK
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Protect destructive actions so public voters cannot clear shared poll data or delete other people's votes. Use per-poll management tokens for poll-specific management actions. Voter and public results links should not contain management authority.

Bulk clearing all polls should either be removed from the normal public UI or guarded by an explicit deployment-level admin secret. Server-side checks are required; hiding buttons in the browser is not sufficient.

## Acceptance criteria

- [ ] Creating a poll gives the host a private management capability separate from public vote and results links.
- [ ] Public vote and results links do not include the per-poll management token.
- [ ] Deleting an individual vote requires the correct management token for that poll.
- [ ] Unauthenticated vote deletion fails with an appropriate JSON error.
- [ ] Clearing all polls is removed from the ordinary public host workflow or requires a deployment-level admin secret.
- [ ] Unauthenticated bulk clear requests fail.
- [ ] Tests prove destructive routes reject missing, invalid, and wrong-poll management tokens.
- [ ] Host management remains usable without adding user accounts.
- [ ] The verification command from Issue 01 passes.

## Blocked by

- .scratch/repo-hardening/issues/01-establish-behavior-test-harness.md
