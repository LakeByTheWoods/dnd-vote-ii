from __future__ import annotations

import argparse
import json
import sqlite3
from datetime import datetime, timezone
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from uuid import uuid4


ROOT = Path(__file__).resolve().parent
DATA_DIR = ROOT / "data"
DB_PATH = DATA_DIR / "dnd_vote.db"
SCORE_RATIO = 2 / 3
STATIC_FILES = {
    "/": ("index.html", "text/html; charset=utf-8"),
    "/index.html": ("index.html", "text/html; charset=utf-8"),
    "/vote.html": ("vote.html", "text/html; charset=utf-8"),
    "/results.html": ("results.html", "text/html; charset=utf-8"),
    "/styles.css": ("styles.css", "text/css; charset=utf-8"),
    "/shared.js": ("shared.js", "application/javascript; charset=utf-8"),
    "/host.js": ("host.js", "application/javascript; charset=utf-8"),
    "/vote.js": ("vote.js", "application/javascript; charset=utf-8"),
    "/results.js": ("results.js", "application/javascript; charset=utf-8"),
}


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def get_connection() -> sqlite3.Connection:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(DB_PATH)
    connection.row_factory = sqlite3.Row
    connection.execute("PRAGMA foreign_keys = ON")
    return connection


def initialize_database() -> None:
    with get_connection() as connection:
        connection.executescript(
            """
            CREATE TABLE IF NOT EXISTS polls (
                id TEXT PRIMARY KEY,
                title TEXT NOT NULL,
                created_at TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS poll_dates (
                id TEXT PRIMARY KEY,
                poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
                date_value TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS votes (
                id TEXT PRIMARY KEY,
                poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
                voter_name TEXT NOT NULL,
                created_at TEXT NOT NULL
            );

            CREATE UNIQUE INDEX IF NOT EXISTS votes_poll_voter_name
            ON votes (poll_id, voter_name COLLATE NOCASE);

            CREATE TABLE IF NOT EXISTS vote_rankings (
                vote_id TEXT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
                date_id TEXT NOT NULL REFERENCES poll_dates(id) ON DELETE CASCADE,
                position INTEGER NOT NULL,
                PRIMARY KEY (vote_id, position)
            );

            CREATE TABLE IF NOT EXISTS vote_unavailability (
                vote_id TEXT NOT NULL REFERENCES votes(id) ON DELETE CASCADE,
                date_id TEXT NOT NULL REFERENCES poll_dates(id) ON DELETE CASCADE,
                PRIMARY KEY (vote_id, date_id)
            );
            """
        )


def fetch_poll(poll_id: str) -> dict | None:
    with get_connection() as connection:
        poll_row = connection.execute(
            "SELECT id, title, created_at FROM polls WHERE id = ?",
            (poll_id,),
        ).fetchone()
        if poll_row is None:
            return None

        date_rows = connection.execute(
            """
            SELECT id, date_value
            FROM poll_dates
            WHERE poll_id = ?
            ORDER BY date_value ASC
            """,
            (poll_id,),
        ).fetchall()

        vote_rows = connection.execute(
            """
            SELECT id, voter_name, created_at
            FROM votes
            WHERE poll_id = ?
            ORDER BY created_at ASC, voter_name COLLATE NOCASE ASC
            """,
            (poll_id,),
        ).fetchall()

        votes = []
        for vote_row in vote_rows:
            ranking_rows = connection.execute(
                """
                SELECT date_id, position
                FROM vote_rankings
                WHERE vote_id = ?
                ORDER BY position ASC
                """,
                (vote_row["id"],),
            ).fetchall()
            unavailable_rows = connection.execute(
                """
                SELECT date_id
                FROM vote_unavailability
                WHERE vote_id = ?
                ORDER BY date_id ASC
                """,
                (vote_row["id"],),
            ).fetchall()

            votes.append(
                {
                    "id": vote_row["id"],
                    "voterName": vote_row["voter_name"],
                    "createdAt": vote_row["created_at"],
                    "rankings": [
                        {"dateId": ranking["date_id"], "position": ranking["position"]}
                        for ranking in ranking_rows
                    ],
                    "unavailableDateIds": [row["date_id"] for row in unavailable_rows],
                }
            )

        return {
            "id": poll_row["id"],
            "title": poll_row["title"],
            "createdAt": poll_row["created_at"],
            "dates": [
                {"id": date_row["id"], "value": date_row["date_value"]}
                for date_row in date_rows
            ],
            "votes": votes,
        }


def fetch_poll_summaries() -> list[dict]:
    with get_connection() as connection:
        rows = connection.execute(
            """
            SELECT
                polls.id,
                polls.title,
                polls.created_at,
                COUNT(DISTINCT poll_dates.id) AS date_count,
                COUNT(DISTINCT votes.id) AS vote_count
            FROM polls
            LEFT JOIN poll_dates ON poll_dates.poll_id = polls.id
            LEFT JOIN votes ON votes.poll_id = polls.id
            GROUP BY polls.id
            ORDER BY polls.created_at DESC
            """
        ).fetchall()
        return [
            {
                "id": row["id"],
                "title": row["title"],
                "createdAt": row["created_at"],
                "dateCount": row["date_count"],
                "voteCount": row["vote_count"],
            }
            for row in rows
        ]


def create_poll(title: str, date_values: list[str]) -> dict:
    poll_id = str(uuid4())
    created_at = utc_now_iso()
    dates = [{"id": str(uuid4()), "value": value} for value in sorted(set(date_values))]

    with get_connection() as connection:
        connection.execute(
            "INSERT INTO polls (id, title, created_at) VALUES (?, ?, ?)",
            (poll_id, title, created_at),
        )
        connection.executemany(
            "INSERT INTO poll_dates (id, poll_id, date_value) VALUES (?, ?, ?)",
            [(date["id"], poll_id, date["value"]) for date in dates],
        )

    return fetch_poll(poll_id)


def replace_vote(
    poll_id: str,
    voter_name: str,
    rankings: list[str],
    unavailable_date_ids: list[str],
) -> dict | None:
    poll = fetch_poll(poll_id)
    if poll is None:
        return None

    valid_date_ids = {date["id"] for date in poll["dates"]}
    clean_name = voter_name.strip()

    if not clean_name:
        raise ValueError("Voter name is required.")
    if not rankings:
        raise ValueError("At least one ranked date is required.")
    if len(rankings) != len(set(rankings)):
        raise ValueError("Ranked dates must be unique.")
    if len(unavailable_date_ids) != len(set(unavailable_date_ids)):
        raise ValueError("Unavailable dates must be unique.")
    if not set(rankings).issubset(valid_date_ids):
        raise ValueError("Rankings include an unknown date.")
    if not set(unavailable_date_ids).issubset(valid_date_ids):
        raise ValueError("Unavailable dates include an unknown date.")
    if set(rankings) & set(unavailable_date_ids):
        raise ValueError("A date cannot be both ranked and unavailable.")

    vote_id = str(uuid4())
    created_at = utc_now_iso()

    with get_connection() as connection:
        existing_vote = connection.execute(
            "SELECT id FROM votes WHERE poll_id = ? AND voter_name = ? COLLATE NOCASE",
            (poll_id, clean_name),
        ).fetchone()
        if existing_vote is not None:
            connection.execute("DELETE FROM votes WHERE id = ?", (existing_vote["id"],))

        connection.execute(
            "INSERT INTO votes (id, poll_id, voter_name, created_at) VALUES (?, ?, ?, ?)",
            (vote_id, poll_id, clean_name, created_at),
        )
        connection.executemany(
            "INSERT INTO vote_rankings (vote_id, date_id, position) VALUES (?, ?, ?)",
            [(vote_id, date_id, position) for position, date_id in enumerate(rankings, start=1)],
        )
        connection.executemany(
            "INSERT INTO vote_unavailability (vote_id, date_id) VALUES (?, ?)",
            [(vote_id, date_id) for date_id in unavailable_date_ids],
        )

    return fetch_poll(poll_id)


def tally_results(poll: dict) -> list[dict]:
    tallies = [
        {
            "id": date["id"],
            "value": date["value"],
            "score": 0.0,
            "unavailableBy": [],
            "disqualified": False,
        }
        for date in poll["dates"]
    ]
    by_id = {item["id"]: item for item in tallies}

    for vote in poll["votes"]:
        for date_id in vote["unavailableDateIds"]:
            result = by_id.get(date_id)
            if result is None:
                continue
            result["disqualified"] = True
            if vote["voterName"] not in result["unavailableBy"]:
                result["unavailableBy"].append(vote["voterName"])

    for vote in poll["votes"]:
        for ranking in vote["rankings"]:
            result = by_id.get(ranking["dateId"])
            if result is None or result["disqualified"]:
                continue
            result["score"] += SCORE_RATIO ** (ranking["position"] - 1)

    return sorted(
        tallies,
        key=lambda item: (item["disqualified"], -item["score"], item["value"]),
    )


class AppHandler(BaseHTTPRequestHandler):
    server_version = "DnDVoteHTTP/1.0"

    def do_GET(self) -> None:
        parsed = urlparse(self.path)

        if parsed.path.startswith("/api/"):
            self.handle_api_get(parsed.path)
            return

        static_entry = STATIC_FILES.get(parsed.path)
        if static_entry is None:
            self.send_error(HTTPStatus.NOT_FOUND, "Not Found")
            return

        file_name, content_type = static_entry
        file_path = ROOT / file_name
        if not file_path.exists():
            self.send_error(HTTPStatus.NOT_FOUND, "File not found")
            return

        content = file_path.read_bytes()
        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(content)))
        self.end_headers()
        self.wfile.write(content)

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if not parsed.path.startswith("/api/"):
            self.send_error(HTTPStatus.NOT_FOUND, "Not Found")
            return
        self.handle_api_post(parsed.path)

    def do_DELETE(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path != "/api/polls":
            self.send_error(HTTPStatus.NOT_FOUND, "Not Found")
            return

        with get_connection() as connection:
            connection.execute("DELETE FROM polls")

        self.send_json({"ok": True})

    def handle_api_get(self, path: str) -> None:
        if path == "/api/polls":
            self.send_json({"polls": fetch_poll_summaries()})
            return

        parts = [part for part in path.split("/") if part]
        if len(parts) == 3 and parts[1] == "polls":
            poll = fetch_poll(parts[2])
            if poll is None:
                self.send_json({"error": "Poll not found."}, status=HTTPStatus.NOT_FOUND)
                return
            self.send_json({"poll": poll})
            return

        if len(parts) == 4 and parts[1] == "polls" and parts[3] == "results":
            poll = fetch_poll(parts[2])
            if poll is None:
                self.send_json({"error": "Poll not found."}, status=HTTPStatus.NOT_FOUND)
                return
            tallied_dates = tally_results(poll)
            winner = next((item for item in tallied_dates if not item["disqualified"]), None)
            self.send_json({"poll": poll, "talliedDates": tallied_dates, "winner": winner})
            return

        self.send_json({"error": "Route not found."}, status=HTTPStatus.NOT_FOUND)

    def handle_api_post(self, path: str) -> None:
        try:
            payload = self.read_json()
        except ValueError as error:
            self.send_json({"error": str(error)}, status=HTTPStatus.BAD_REQUEST)
            return

        if path == "/api/polls":
            title = str(payload.get("title", "")).strip()
            date_values = payload.get("dates", [])
            if not title:
                self.send_json({"error": "Title is required."}, status=HTTPStatus.BAD_REQUEST)
                return
            if not isinstance(date_values, list):
                self.send_json({"error": "Dates must be an array."}, status=HTTPStatus.BAD_REQUEST)
                return

            cleaned_dates = sorted({str(value).strip() for value in date_values if str(value).strip()})
            if len(cleaned_dates) < 2:
                self.send_json(
                    {"error": "At least two unique dates are required."},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return

            poll = create_poll(title, cleaned_dates)
            self.send_json({"poll": poll}, status=HTTPStatus.CREATED)
            return

        parts = [part for part in path.split("/") if part]
        if len(parts) == 4 and parts[1] == "polls" and parts[3] == "votes":
            rankings = payload.get("rankings", [])
            unavailable = payload.get("unavailableDateIds", [])
            if not isinstance(rankings, list) or not isinstance(unavailable, list):
                self.send_json(
                    {"error": "Rankings and unavailableDateIds must be arrays."},
                    status=HTTPStatus.BAD_REQUEST,
                )
                return

            try:
                poll = replace_vote(
                    parts[2],
                    str(payload.get("voterName", "")),
                    [str(item) for item in rankings],
                    [str(item) for item in unavailable],
                )
            except ValueError as error:
                self.send_json({"error": str(error)}, status=HTTPStatus.BAD_REQUEST)
                return

            if poll is None:
                self.send_json({"error": "Poll not found."}, status=HTTPStatus.NOT_FOUND)
                return

            self.send_json({"poll": poll}, status=HTTPStatus.CREATED)
            return

        self.send_json({"error": "Route not found."}, status=HTTPStatus.NOT_FOUND)

    def read_json(self) -> dict:
        content_length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(content_length) if content_length else b"{}"
        try:
            payload = json.loads(body.decode("utf-8"))
        except json.JSONDecodeError as error:
            raise ValueError("Request body must be valid JSON.") from error
        if not isinstance(payload, dict):
            raise ValueError("Request body must be a JSON object.")
        return payload

    def send_json(self, payload: dict, status: HTTPStatus = HTTPStatus.OK) -> None:
        content = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(content)))
        self.end_headers()
        self.wfile.write(content)

    def log_message(self, format: str, *args) -> None:
        return


def main() -> None:
    parser = argparse.ArgumentParser(description="Run the D&D date voting app.")
    parser.add_argument("--host", default="127.0.0.1", help="Host interface to bind.")
    parser.add_argument("--port", default=8000, type=int, help="Port to serve on.")
    args = parser.parse_args()

    initialize_database()
    server = ThreadingHTTPServer((args.host, args.port), AppHandler)
    print(f"Serving D&D vote app on http://{args.host}:{args.port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
