Status: completed

## Parent

.scratch/go-server-migration/PRD.md

## What to build

Port the ranked-pairs head-to-head tally into a pure Go module and expose poll results through the Go API. Results viewers should see the same user-facing outcome concepts as before: submitted ballots, disqualified unavailable dates, ordered tallied dates, ranks, head-to-head records, unavailable-by names, and the current winner.

This slice should make the tally behavior independently testable and keep result calculation separate from HTTP and SQLite details.

## Acceptance criteria

- [ ] A pure Go tally module calculates ranked-pairs results from an in-memory poll representation.
- [ ] Dates marked unavailable by any voter are disqualified before pairwise tallying.
- [ ] The tally handles ordinary winners, cycle-skipping, ties, one eligible date, all-disqualified polls, and unavailable-date disqualification.
- [ ] `GET /api/polls/{pollId}/results` returns the poll, ordered tallied dates, and winner.
- [ ] Tallied date payloads include rank, record, disqualification status, and unavailable-by names.
- [ ] The results API does not leak management tokens.
- [ ] The existing static results frontend can display results without a frontend framework migration.
- [ ] Tally behavior is covered by table-driven Go tests.
- [ ] Results API behavior is covered by Go HTTP tests using a temporary SQLite database.

## Blocked by

- .scratch/go-server-migration/issues/03-submit-and-replace-votes-end-to-end.md

## Comments

- Completed on branch `codex/go-server-migration/04-ranked-pairs-results` and merged into `codex/go-server-migration-prd`.
- Acceptance review: approved.
- Code-quality review: approved after adding intent comments around ranked-pairs algorithm decisions.
- Verification on PRD branch: `$env:PATH='C:\Program Files\Go\bin;' + $env:PATH; python scripts\verify.py` passed.
