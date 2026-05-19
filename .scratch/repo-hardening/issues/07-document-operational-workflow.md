---
title: Document operational workflow
status: completed
type: AFK
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Update project documentation so a non-web and non-Python developer can understand how the app works, how to verify it locally, how ranked-pairs results are calculated at a high level, and how Railway deployment should be configured.

The documentation should describe the final hardened behavior rather than transitional implementation details.

## Acceptance criteria

- [x] Documentation explains the app's host, voter, and results flows.
- [x] Documentation explains ranked-pairs/head-to-head results at a high level.
- [x] Documentation includes the local run command and the local verification command.
- [x] Documentation describes SQLite data storage and the Railway volume environment variable.
- [x] Documentation describes host management capability and destructive-action protection.
- [x] Documentation does not describe geometric scoring as the current tally rule.
- [x] The verification command from Issue 01 passes.

## Blocked by

- .scratch/repo-hardening/issues/02-resolve-and-lock-ranked-pairs-tally.md
- .scratch/repo-hardening/issues/03-harden-poll-creation-validation.md
- .scratch/repo-hardening/issues/04-harden-ballot-validation-and-replacement.md
- .scratch/repo-hardening/issues/05-protect-destructive-management-actions.md
- .scratch/repo-hardening/issues/06-add-non-drag-ranking-controls.md
