# Session Vote operations

## Services

- Site: https://lakebythewoods.neocities.org/session-vote/
- API: https://session-vote-api.elbeaston.workers.dev
- Cloudflare Worker: `session-vote-api`
- D1 database: `session-vote`, bound as `DB`

Cloudflare hosts the API and database. Neocities hosts only the nine files listed
in `scripts/static.mjs`. The deployment script checks the Neocities identity and
Supporter status before writing. It does not change the site's home page.

## Local development and checks

Use Node.js 24 LTS and npm. Run:

```text
npm ci
npm run dev
```

Open http://localhost:8000/session-vote/. The API listens on port 8787.
Local D1 data persists in `.wrangler/state`. Local migrations run before startup.
Production permits only the Neocities origin; local development permits localhost
and 127.0.0.1 on port 8000. Origin checks do not replace host-token checks.

Run `npm run verify` or `python scripts/verify.py` before deployment. This runs
TypeScript checks, frontend syntax checks, Workers/D1 tests, and frontend contract
tests. Tests use isolated local D1 data; they do not use the remote database.
The tally fixtures were exported from the original Go tests before Go was removed.
The sharp override fixes a transitive development dependency advisory; the Worker
has no production npm dependencies.

## Credentials and deployment

Store `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`, and `NEOCITIES_API_KEY` in
`~/.config/session-vote/deploy-secrets.json`. The Cloudflare token needs Workers
Scripts Edit and D1 Edit on the configured account. Use the Neocities API key for
lakebythewoods. Never put these values in frontend files or Git.

Commit changes, then run:

```text
npm run deploy
```

The script rejects an uncommitted source tree. It saves existing target files and
the prior Worker version, applies D1 migrations, installs the admin secret,
deploys the Worker, tests a poll and ballot, redeploys to check persistence, and
deletes only its recorded test poll. It then uploads changed frontend files and
checks their remote hashes. A manifest records the commit, file hashes, and result.
Backups and manifests live under `~/.config/session-vote/deployments/`.

The admin secret is generated once and kept in
`~/.config/session-vote/admin-secret.txt` and the Worker secret
`DND_VOTE_ADMIN_SECRET`. Without that Worker secret, bulk deletion is disabled.
Host links contain a separate per-poll token. Keep those links private.

If a request outcome is unknown, inspect the saved manifest and remote state
before retrying. A failed deployment attempts to restore the prior frontend and
Worker version, while preserving D1 data. Failed test poll IDs remain recorded
in the manifest. Clean them up by exact ID, never by clearing the whole database.

## Backup and rollback

Export the production database with:

```text
npm run db:export
```

The SQL export is saved outside Git in the private deployment directory. It
contains host tokens and ballot data. Keep it private. Export before future schema
changes. Apply future migrations as new numbered SQL files; do not edit a migration
that has already run.

To restore the frontend and Worker from a saved deployment manifest:

```text
node scripts/deploy.mjs rollback "C:/Users/elbea/.config/session-vote/deployments/TIMESTAMP/manifest.json"
```

Rollback restores overwritten app files and removes newly uploaded app files.
It restores the prior Worker version where one was recorded. It does not reverse
database migrations or delete user polls. Before rolling back future releases,
check that the old Worker can read the current schema.

## Live checks

Open the Neocities URL, create a disposable poll, and save its ID. Use the voting
link to change date order, mark one date unavailable, and submit. Use Edit Vote to
replace the ballot and confirm only one ballot remains. Confirm that the public
results view has no delete button and that the host link can delete a test vote.
Remove only the recorded test poll through D1 after the check.

Use the Workers dashboard for invocation errors and CPU limits, and the D1
dashboard for query and storage use. The API returns no-store responses and logs
only an operation label and error type, never request bodies or tokens. Free-plan
limits can cause request failures; the deployment script does not upgrade plans.
The health route checks the Worker; fetching the poll list also checks D1.
