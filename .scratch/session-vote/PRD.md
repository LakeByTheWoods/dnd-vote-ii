# Session Vote migration

Status: ready-for-human

Deploy the static frontend to https://lakebythewoods.neocities.org/session-vote/.
Replace Go and SQLite hosting with a TypeScript Worker and Cloudflare D1.
Start with an empty database. Preserve the poll API, ranked-pairs rules, ballot
replacement rules, and host permissions. Rename visible D&D branding to Session Vote.

The user authorised implementation, account resource creation, deployment, test data
cleanup, and Git commits. Credentials stay outside the repository under the user's
.config/session-vote directory. Use the existing Free Cloudflare plan.

Acceptance: API and tally tests pass; local and live browser workflows work;
data survives restarts and redeployment; only the app's Neocities folder changes;
operations documentation includes deployment, export, and rollback.

## Comments

2026-10-08: Implemented and deployed. The automated suite and live browser checks
passed. See docs/migration-verification.md for evidence and deployment details.
