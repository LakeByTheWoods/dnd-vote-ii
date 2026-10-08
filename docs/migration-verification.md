# Migration verification — 8 October 2026

Session Vote is live at https://lakebythewoods.neocities.org/session-vote/.
The deployed source commit is `f6ce04d`. The active Worker version after the
persistence check is `4e581adc-b2cb-4d3a-b874-e71252bfeae5`.

## Checks passed

- TypeScript and JavaScript syntax checks.
- 20 Workers/D1 tests and two frontend contract tests. These include six exported
  Go tally fixtures, concurrent replacement, transaction rollback, cascading
  deletes, validation, permissions, CORS, Unicode, UTC boundaries, and large ballots.
- Local browser: poll creation, share links, ranking, unavailability, submission,
  editing, winner changes, host view, and vote deletion. The user accepted the
  existing native delete confirmation after it blocked the in-app browser.
- Local D1: poll and ballot data matched after the server process restarted.
- Production API: creation, voting, tally, host deletion, and exact data equality
  across Worker redeployment.
- Live Neocities browser: creation, voting, editing, winner changes, public results,
  and host controls. No console errors were reported. Live deletion was checked
  through the API to avoid another native browser dialog.
- All nine uploaded frontend files matched their source hashes.
- Database SQL export completed. Deployment credentials were excluded from Git.
- npm dependency audit reported zero vulnerabilities after the sharp override.

All production test polls were removed by exact recorded IDs. The production
poll list was empty after cleanup. No existing local SQLite files were imported
or deleted. No unrelated Neocities files were changed.

## Deployment recovery

The first attempt reached the temporary Worker before route propagation completed.
Rollback restored the prior Worker and preserved D1; no frontend files had been
uploaded. The deploy script now waits for health with bounded retries. A second
attempt exposed Cloudflare's restriction on secret edits after rollback; deploying
the current version before installing its secret resolved it. The final attempt
passed all deployment checks.

The private deployment manifest is under
`~/.config/session-vote/deployments/2026-10-08T09-20-42-330Z/manifest.json`.
Browser screenshots and the test record are saved locally under
`.scratch/session-vote/evidence/` and are excluded from Git.
Use docs/deployment.md for redeployment, export, and rollback commands.
