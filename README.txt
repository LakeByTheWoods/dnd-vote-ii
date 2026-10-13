Dungeons & Dates
===============
A web app for friends to choose a Dungeons and Dragons date. Hosts create polls
and share voting/results links. Participants rank dates without accounts. Any
date marked unavailable by any voter is disqualified. The existing Ranked Pairs
voting implementation is unchanged by the authorisation changes.

Access model
------------
One random host key authorises administration of the whole installation (not
per-poll ownership). Share it only with trusted co-hosts: they can manage every
poll and ballot. The server requires DND_VOTE_ADMIN_TOKEN at startup, with at
least 32 ASCII characters and no whitespace. There is no default or bypass for
local use. Generate a random key, rather than padding a memorable password.

HTTP operations:
- GET /api/polls: host only; poll discovery is deliberately private.
- POST /api/polls: host only; create a poll.
- DELETE /api/polls: host only; irreversibly clear every poll and its votes.
- DELETE /api/polls/{pollId}/votes/{voteId}: host only; remove a ballot.
- GET /api/polls/{pollId}: accessible with the poll link.
- GET /api/polls/{pollId}/results: accessible with the poll link.
- POST /api/polls/{pollId}/votes: create a ballot with the poll link; replacing
  a ballot with the same name requires its private editing receipt or host key.

Host requests send Authorization: Bearer <host key>. Missing/incorrect keys
receive HTTP 401 before any protected database operation. Supplying a key in a
URL or JSON body does not authorise a request. No cross-origin CORS access is
enabled; administrative requests use an explicit header, not ambient cookies.

The host page has Unlock and Lock controls. The key is kept in sessionStorage
for that browser tab, never in share links or persistent localStorage. Lock
removes it from the tab; other unlocked tabs remain unlocked. Rotating the
server key and restarting revokes the old key everywhere. No host credential
is served in static files or API responses. API responses use Cache-Control:
no-store. Pages also set Referrer-Policy: no-referrer.

Poll links contain random UUIDs and are bearer capabilities: anyone who gets
one may view the poll, participant names, rankings, availability and results,
and create a new ballot. They cannot enumerate other polls or administer them.
Only share these links with the intended group. This remains a friends app,
not verified identity or a one-person-one-vote election system: link holders
can submit under new names. There are no participant accounts.

Editing receipts
----------------
The first successful ballot submission returns a separate voteToken. The
browser saves it in localStorage, scoped to that poll and voter name, and sends
it in X-Vote-Token when editing. Only a SHA-256 hash is stored in SQLite; neither
the token nor its hash appears in public poll/results responses. A name alone
cannot replace someone else's ballot (HTTP 403). A receipt cannot delete a
ballot: deletion is host only. Results show Edit Vote only when this browser
has the receipt or host key, and Delete Vote only for an unlocked host.

Keep using the same browser to edit. Clearing browser storage, private browsing
or moving to another device loses access to editing; ask the host to remove
that ballot so you can resubmit. A trusted API client may retain the voteToken
from submission and send it as X-Vote-Token on another device; treat it as a
secret and never put it in a URL. A host replacement issues a fresh receipt and
revokes the previous one. Normal participant edits preserve their receipt.

Existing databases are automatically migrated without changing votes or
results. Older ballots have no receipt and can only be changed by the host.
For a voter to resume editing, the simplest recovery is host deletion followed
by a new participant submission. Back up the SQLite database before upgrading.

Run locally (Windows PowerShell)
-------------------------------
1. In the repository directory, generate a key for this server process:
   $env:DND_VOTE_ADMIN_TOKEN = python -c "import secrets; print(secrets.token_urlsafe(32))"
2. Copy it to paste into the host page, without printing it to the terminal:
   Set-Clipboard -Value $env:DND_VOTE_ADMIN_TOKEN
3. Start with: python main.py
4. Open http://localhost:8000, paste the key into Host Access, and click Unlock.
5. Clear the clipboard after use and save the key in a password manager if you
   want to reuse it; an environment variable set this way lasts for that shell.

On Linux/macOS, generate the process variable with:
   export DND_VOTE_ADMIN_TOKEN="$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')"
Then start python3 main.py. Retrieve the key privately for the host page, or
supply a previously generated key through your password manager/secret tooling.

The local default is 127.0.0.1:8000. For trusted LAN access, explicitly start
python main.py --host 0.0.0.0 and share links using the server's LAN address
(rather than localhost). Use HTTPS whenever credentials cross an untrusted
network; the built-in HTTP server does not terminate TLS. HOST and PORT can
also configure the listener. DND_VOTE_DATA_DIR selects the SQLite directory;
the local default is data/dnd_vote.db.

Railway deployment
------------------
1. Push the updated repository to GitHub and deploy it as a Railway service.
2. Add a volume mounted at /data and set DND_VOTE_DATA_DIR=/data.
3. Generate a host key locally with Python's secrets.token_urlsafe(32), save it
   in a password manager, and set DND_VOTE_ADMIN_TOKEN as a service variable.
   Do not commit it to the repository, image, frontend, railway.toml or .env
   examples. Do not paste it into issue reports, logs or screenshots.
4. railway.toml explicitly starts python main.py --host 0.0.0.0; the server
   uses Railway's PORT. The unauthenticated / health check serves the host page
   shell but exposes no saved polls or credentials.
5. Deploy and generate a public HTTPS domain. Open / on that domain and unlock
   with the host key, create a poll and share only its generated voting/results
   links. Participants never need the key.
6. If the key leaks, replace the variable with a new random key and redeploy.
   All requests with the previous key then fail. Keep database backups private:
   they include participant information and editing receipt hashes.

Pages
-----
- / or /index.html: host page; public shell, protected operations.
- /vote.html?poll=...: ballot.
- /results.html?poll=...: results and permitted ballot actions.

Verification
------------
Run: python -m unittest discover -s tests -v
Optional client integration check (Node.js 18 or newer):
   node --test tests/test_client.cjs
It runs shared.js in isolated browser-like storage contexts against a real
Python HTTP server, checking request headers, receipt persistence, host locking
and credential-free shared links. Set PYTHON to your Python executable if needed.
Uses only the Python standard library: a real local threaded HTTP server and
isolated temporary SQLite databases. Tests cover unauthorised and authorised
host endpoints, cascades, receipt ownership and replacement, concurrent first
submissions, public voting/results, disqualification, legacy migration and
startup configuration. They do not contact a deployed instance. Railway's
HTTPS proxy, environment variables and volume configuration require deployment
verification; this suite is not a live security audit. The client check does
not exercise rendered DOM interactions in a real browser.
