This project is a web app to help a friend group vote for and decide on a date for Dungeons and Dragons.
The app is allows a Host to create a new poll of potential dates.
Voters will then vote on those dates with preferential voting.
Voters may mark dates as being unavailable, if any Voter does this for a date, then that date will be disqualified.
Votes should be counted with a geometric scoring system, if a vote is someone's first preference, then that is 1 point, second preference should be 2/3rds a point, third preference should be 2/3rds of that etc.

This repository now includes a real cross-device version of the app backed by SQLite.

Railway deployment:
1. Push this repo to GitHub
2. In Railway, create a new project from the repo
3. Add a volume and mount it at `/data`
4. Set environment variable `DND_VOTE_DATA_DIR=/data`
5. Deploy the service
6. Generate a Railway public domain for the service and share that URL

Run locally:
1. Start the server with `python main.py`
2. Open `http://localhost:8000` on the host machine
3. Create polls from the host page and share the generated vote link

Pages:
- `/` or `/index.html` is the host page for creating polls
- `/vote.html?poll=...` is the voter ballot
- `/results.html?poll=...` is the results page

Notes:
- Polls and votes are stored in `data/dnd_vote.db`
- On Railway, the SQLite file should live on the mounted volume via `DND_VOTE_DATA_DIR=/data`
- Votes from the same voter name replace that voter's previous ballot for the same poll
- Any date marked unavailable by any voter is disqualified from scoring
