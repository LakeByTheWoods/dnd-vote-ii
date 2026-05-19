---
title: Resolve and lock ranked-pairs tally
status: ready-for-agent
type: AFK
---

## Parent

.scratch/repo-hardening/PRD.md

## What to build

Make ranked-pairs/head-to-head behavior the explicit product rule for DnD Vote results. The app currently implements head-to-head ranked-pairs behavior while the README describes geometric scoring; this slice should remove that mismatch by documenting, labeling, and regression-testing the selected ranked-pairs rule.

Result calculation should be isolated enough to test without running the web server, while preserving the existing public behavior unless tests reveal a real ranked-pairs bug.

## Acceptance criteria

- [ ] Documentation describes ranked-pairs/head-to-head tallying instead of geometric scoring.
- [ ] Results UI labels use ranked-pairs/head-to-head language consistently.
- [ ] The tallying logic is covered by focused tests for a normal winner, ties, one eligible date, all-disqualified dates, and unavailable-date disqualification.
- [ ] Tests verify that repeated voter-name replacement affects the final tally as one current ballot, not multiple ballots.
- [ ] The results API still returns the poll, ordered tallied dates, and winner in a shape the frontend can render.
- [ ] The verification command from Issue 01 passes.

## Blocked by

- .scratch/repo-hardening/issues/01-establish-behavior-test-harness.md
