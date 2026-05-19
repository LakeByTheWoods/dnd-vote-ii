---
title: Add non-drag ranking controls
status: completed
type: AFK
completed_by: codex/repo-hardening/06-ranking-controls
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Make voting usable without native drag-and-drop. The ballot should support mobile and keyboard-friendly ranking controls while keeping drag-and-drop as an enhancement for browsers where it works well.

This slice should preserve the same submitted ballot shape: ordered ranked date ids plus unavailable date ids.

## Acceptance criteria

- [ ] Each ranked date can be moved up or down without drag-and-drop.
- [ ] Ranking controls work on narrow mobile viewports.
- [ ] Ranking controls are keyboard accessible.
- [ ] Marking a date unavailable still removes it from the submitted ranked list.
- [ ] Drag-and-drop may remain available, but it is no longer the only way to rank dates.
- [ ] Submitted ballots preserve the intended ranking order.
- [ ] The verification command from Issue 01 passes.

## Blocked by

- .scratch/repo-hardening/issues/04-harden-ballot-validation-and-replacement.md
