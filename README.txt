DnD Vote is a small web app for choosing a Dungeons and Dragons session date with a friend group. It runs as a Python standard-library web server with static HTML, CSS, and JavaScript pages, and stores poll data in SQLite.

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

Local run:

1. Install Python dependencies with `pip install -r requirements.txt` if your environment does not already have them.
2. Start the server with `python main.py`.
3. Open `http://localhost:8000`.
4. Create a poll from the host page and share the generated voter link.

Local verification:

Run `python scripts\verify.py` before deploying or handing off changes.

The verification command checks Python syntax, runs the Python behavior tests against a temporary SQLite database, and checks JavaScript syntax with `node --check` when Node.js is installed.

Data storage:

Polls, candidate dates, votes, and host management tokens are stored in SQLite. By default, the local database file is `data/dnd_vote.db`.

For Railway, the SQLite database must live on a persistent volume so deploys do not erase poll data:

1. Push this repo to GitHub.
2. In Railway, create a new project from the repo.
3. Add a Railway volume and mount it at `/data`.
4. Set the environment variable `DND_VOTE_DATA_DIR=/data`.
5. Deploy the service.
6. Generate a Railway public domain and share that URL with the group.

Management and destructive actions:

The voter and results links are public poll links. The host management link is private because it carries the poll's management token in the `manage` query parameter.

Keep the host management link with the person running the poll. It allows host-level cleanup such as deleting an individual vote from that poll. The server checks the management token before allowing those actions, so hiding buttons in the browser is not the security boundary.

Bulk clearing every poll is protected separately. To allow an operator to clear all polls, set `DND_VOTE_ADMIN_SECRET` in the server environment and send that same value as the admin secret when calling the bulk-clear API. Without the configured secret, or with the wrong secret, the server rejects the action.

Important URLs:

- `/` or `/index.html` is the host poll-creation page.
- `/vote.html?poll=...` is the voter ballot page.
- `/results.html?poll=...` is the public results page.
- `/results.html?poll=...&manage=...` is the host management view for that poll.
