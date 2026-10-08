Session Vote helps a group choose a session date.

Live site: https://lakebythewoods.neocities.org/session-vote/

The static HTML, CSS, and JavaScript files are hosted on Neocities. A TypeScript
Cloudflare Worker handles the API, and Cloudflare D1 stores polls and ballots.
Go and Railway are no longer supported deployment paths. Existing local SQLite
files are preserved, but are not imported into the new database.

Run locally:
1. Install Node.js 24 LTS.
2. Run npm ci.
3. Run npm run dev.
4. Open http://localhost:8000/session-vote/.

Verify: npm run verify (or python scripts/verify.py).
Deploy: commit the source, then run npm run deploy.
Export the database: npm run db:export.
See docs/deployment.md for credentials, backups, rollback, and live checks.

How it works:
- Create a poll with at least two unique candidate dates, today or later.
- Share the voter and results links. Keep the host management link private.
- Voters enter a name, rank available dates, and mark dates they cannot attend.
- Submitting the same voter name again replaces that ballot. SQLite NOCASE rules
  apply to the stored name comparison; there are no user accounts.
- A date marked unavailable by any voter is disqualified before tallying.
- Remaining dates use ranked-pairs head-to-head comparisons. Stronger victories
  are locked first, except when an edge would create a cycle. Stable tie rules
  preserve the original app's result order.
- The host link can delete a vote. The API checks the private management token.
- Bulk deletion requires the separately configured admin secret.

API routes and fields are unchanged. X-DnD-Vote-Management-Token and
X-DnD-Vote-Admin-Secret retain their names for compatibility. Server date checks
and timestamps use UTC; the browser displays dates in its local format.
