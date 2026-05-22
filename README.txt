DnD Vote is a small web app for choosing a Dungeons and Dragons session date with a friend group. The supported backend is the Go server in `cmd/dnd-vote`; it serves the static HTML, CSS, and JavaScript files and stores poll data in SQLite.

How the app works:

1. The host opens `/` or `/index.html`, enters a poll title, and selects at least two future candidate dates.
2. Creating the poll saves it in SQLite and returns three links:
   - a voter link for players to submit ballots;
   - a results link for anyone who should be able to watch the outcome;
   - a host management link that includes a private management token.
3. Voters open `/vote.html?poll=...`, enter their name, rank the dates they can attend, and mark any dates they cannot attend as unavailable. Ranking can be changed with drag-and-drop or with the up/down controls.
4. If the same voter name submits again for the same poll, the browser labels the ballot as an edit and the newer submission replaces the older one.
5. The results page at `/results.html?poll=...` shows submitted ballots, unavailable dates, the ranked-pairs winner, and the head-to-head comparison details.

Tally rule:

Votes are counted with a ranked-pairs head-to-head tally.

Any date marked unavailable by any voter is disqualified before tallying. The remaining eligible dates are compared against each other one pair at a time. For each pair, the app counts which date is preferred by more submitted rankings. The strongest pair wins are locked into an ordering first, but a win is skipped if locking it would create a cycle. The first date in the final locked ordering is the winner.

This means the result is based on pairwise preference strength between eligible dates. The results page also shows each eligible date's head-to-head record so the outcome can be explained without reading the source code.

Local install:

1. Install Go 1.23 or newer.
2. Install Python 3 if you want to use the repository verification helper.
3. Install Node.js if you want JavaScript syntax checks to run locally. The verifier skips JavaScript checks when Node.js is not on `PATH`.

Local run:

1. Start the Go server with `go run ./cmd/dnd-vote`.
2. Open `http://localhost:8000`.

The server listens on `0.0.0.0:8000` by default. Override the bind address with `DND_VOTE_HOST` and `DND_VOTE_PORT` when needed.

Local verification:

Run `python scripts\verify.py` before deploying or handing off changes.

The verification command runs `go test ./...` and checks JavaScript syntax with `node --check` when Node.js is installed. It does not run a Python server or Python server tests.

Data storage:

Polls, candidate dates, votes, and host management tokens are stored in SQLite. By default, the database file is `data/dnd_vote.db` under the server working directory.

Set `DND_VOTE_DATA_DIR` to choose the directory that contains `dnd_vote.db`. Set `DND_VOTE_DB_PATH` only when the database file needs an exact path; it overrides `DND_VOTE_DATA_DIR`.

For Railway, put the SQLite database on a persistent volume so deploys do not erase poll data:

1. Push this repo to GitHub.
2. In Railway, create a new project from the repo.
3. Add a Railway volume and mount it at `/data`.
4. Set `DND_VOTE_DATA_DIR=/data`.
5. Deploy the service. `railway.toml` starts the Go server with `go run ./cmd/dnd-vote`.
6. Generate a Railway public domain and share that URL with the group.

Management and destructive actions:

The voter and results links are public poll links. The host management link is private because it carries the poll's management token in the `manage` query parameter.

Keep the host management link with the person running the poll. The browser uses the `manage` query parameter value as `X-DnD-Vote-Management-Token` when deleting an individual vote from that poll. The Go server checks the token against the stored token for that poll before deleting anything, so hiding buttons in the browser is not the security boundary.

Bulk clearing every poll is protected separately. Configure `DND_VOTE_ADMIN_SECRET` in the server environment and send the same value in the `X-DnD-Vote-Admin-Secret` header when calling the bulk-clear API. Without a configured secret, without the header, or with the wrong header value, the Go server rejects the action.

Important URLs:

- `/` or `/index.html` is the host poll-creation page.
- `/vote.html?poll=...` is the voter ballot page.
- `/results.html?poll=...` is the public results page.
- `/results.html?poll=...&manage=...` is the host management view for that poll.
