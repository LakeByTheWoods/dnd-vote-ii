"""Exercise the real HTTP handler and SQLite database without external packages."""
import concurrent.futures
import http.client
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

import server


class HttpAuthorizationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.key = "test-host-" + "a" * 40
        self.config = patch.multiple(server, DATA_DIR=Path(self.temp.name),
                                     DB_PATH=Path(self.temp.name) / "test.db", ADMIN_TOKEN=self.key)
        self.config.start()
        self.addCleanup(self.config.stop)
        server.initialize_database()
        self.httpd = server.ThreadingHTTPServer(("127.0.0.1", 0), server.AppHandler)
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_server)
        self.host = {"Authorization": "Bearer " + self.key}
        self.poll = self.create_poll()
        self.ids = [date["id"] for date in self.poll["dates"]]

    def stop_server(self):
        self.httpd.shutdown()
        self.httpd.server_close()
        self.thread.join(timeout=5)

    def request(self, method, path, body=None, headers=None):
        connection = http.client.HTTPConnection(*self.httpd.server_address, timeout=5)
        try:
            request_headers = {"Content-Type": "application/json", **(headers or {})}
            connection.request(method, path, body=json.dumps(body) if body is not None else None,
                               headers=request_headers)
            response = connection.getresponse()
            raw = response.read()
            data = json.loads(raw) if response.getheader("Content-Type", "").startswith("application/json") else raw.decode()
            return response.status, data, dict(response.getheaders())
        finally:
            connection.close()

    def create_poll(self):
        status, data, _ = self.request("POST", "/api/polls", {
            "title": "Campaign", "dates": ["2026-11-01", "2026-11-02", "2026-11-03"]
        }, self.host)
        self.assertEqual(status, 201)
        return data["poll"]

    def ballot(self, name="Alice", ids=None):
        return {"voterName": name, "rankings": ids or self.ids, "unavailableDateIds": []}

    def submit(self, ballot=None, headers=None, poll_id=None):
        return self.request("POST", f'/api/polls/{poll_id or self.poll["id"]}/votes',
                            ballot or self.ballot(), headers)

    def get_poll(self):
        status, data, _ = self.request("GET", f'/api/polls/{self.poll["id"]}')
        self.assertEqual(status, 200)
        return data["poll"]

    def test_poll_discovery_and_creation_require_host(self):
        for headers in ({}, {"Authorization": "Bearer wrong"}, {"Authorization": "Basic " + self.key}):
            self.assertEqual(self.request("GET", "/api/polls", headers=headers)[0], 401)
            self.assertEqual(self.request("POST", "/api/polls", {}, headers)[0], 401)
        status, data, _ = self.request("GET", "/api/polls", headers=self.host)
        self.assertEqual(status, 200)
        self.assertEqual([poll["id"] for poll in data["polls"]], [self.poll["id"]])
        self.assertNotIn(self.key, json.dumps(data))

    def test_unauthorised_clear_does_not_change_database(self):
        _, vote, _ = self.submit()
        before = self.get_poll()
        for headers in ({}, {"Authorization": "Bearer wrong"}, {"X-Vote-Token": vote["voteToken"]}):
            self.assertEqual(self.request("DELETE", "/api/polls", headers=headers)[0], 401)
            self.assertEqual(self.get_poll(), before)

    def test_authorised_clear_cascades_all_poll_data(self):
        self.submit()
        self.create_poll()
        self.assertEqual(self.request("DELETE", "/api/polls", headers=self.host)[0], 200)
        with server.get_connection() as connection:
            for table in ("polls", "poll_dates", "votes", "vote_rankings", "vote_unavailability"):
                self.assertEqual(connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0], 0)

    def test_ballot_deletion_is_host_only_and_poll_scoped(self):
        _, data, _ = self.submit()
        vote_id = data["poll"]["votes"][0]["id"]
        path = f'/api/polls/{self.poll["id"]}/votes/{vote_id}'
        before = self.get_poll()
        for headers in ({}, {"Authorization": "Bearer wrong"}, {"X-Vote-Token": data["voteToken"]}):
            self.assertEqual(self.request("DELETE", path, headers=headers)[0], 401)
            self.assertEqual(self.get_poll(), before)
        other = self.create_poll()
        self.assertEqual(self.request("DELETE", f'/api/polls/{other["id"]}/votes/{vote_id}', headers=self.host)[0], 404)
        self.assertEqual(self.get_poll(), before)
        self.assertEqual(self.request("DELETE", path, headers=self.host)[0], 200)
        self.assertEqual(self.get_poll()["votes"], [])
        self.assertEqual(self.request("DELETE", path, headers=self.host)[0], 404)

    def test_key_in_query_or_json_does_not_authorise(self):
        self.assertEqual(self.request("GET", "/api/polls?token=" + self.key)[0], 401)
        self.assertEqual(self.request("DELETE", "/api/polls?token=" + self.key)[0], 401)
        self.assertEqual(self.request("POST", "/api/polls", {"token": self.key})[0], 401)

    def test_missing_configuration_fails_closed(self):
        with patch.object(server, "ADMIN_TOKEN", ""):
            for method in ("GET", "POST", "DELETE"):
                self.assertEqual(self.request(method, "/api/polls", headers={"Authorization": "Bearer "})[0], 401)
        self.assertEqual(len(self.get_poll()["dates"]), 3)

    def test_public_voting_results_and_disqualification_unchanged(self):
        status, data, headers = self.submit({"voterName": "Alice", "rankings": self.ids[1::-1],
                                           "unavailableDateIds": [self.ids[2]]})
        self.assertEqual(status, 201)
        self.assertEqual(headers["Cache-Control"], "no-store")
        self.assertGreaterEqual(len(data["voteToken"]), 32)
        self.assertEqual(self.submit(self.ballot("Bob", [self.ids[1], self.ids[0], self.ids[2]]))[0], 201)
        status, results, _ = self.request("GET", f'/api/polls/{self.poll["id"]}/results')
        self.assertEqual(status, 200)
        self.assertEqual(results["winner"]["id"], self.ids[1])
        self.assertEqual(results["winner"]["record"], "1-0-0")
        excluded = next(date for date in results["talliedDates"] if date["id"] == self.ids[2])
        self.assertTrue(excluded["disqualified"])
        self.assertEqual(excluded["unavailableBy"], ["Alice"])
        public_json = json.dumps(results)
        self.assertNotIn(data["voteToken"], public_json)
        self.assertNotIn("edit_token_hash", public_json)
        self.assertNotIn(self.key, public_json)
        self.assertEqual(self.request("GET", "/api/polls/nonexistent/results")[0], 404)
        self.assertEqual(self.request("GET", "/api/polls/nonexistent")[0], 404)

    def test_name_impersonation_cannot_replace_ballot(self):
        _, data, _ = self.submit()
        before = self.get_poll()
        replacement = self.ballot("ALICE", list(reversed(self.ids)))
        for headers in ({}, {"X-Vote-Token": "wrong"}):
            self.assertEqual(self.submit(replacement, headers)[0], 403)
            self.assertEqual(self.get_poll(), before)
        self.assertEqual(self.submit({**replacement, "voteToken": data["voteToken"]})[0], 403)
        status, edited, _ = self.submit(replacement, {"X-Vote-Token": data["voteToken"]})
        self.assertEqual(status, 201)
        self.assertEqual(edited["voteToken"], data["voteToken"])
        self.assertEqual(len(edited["poll"]["votes"]), 1)
        self.assertEqual(edited["poll"]["votes"][0]["rankings"][0]["dateId"], self.ids[2])
        # A receipt remains usable for subsequent edits.
        self.assertEqual(self.submit(headers={"X-Vote-Token": edited["voteToken"]})[0], 201)

    def test_receipts_are_scoped_to_one_ballot_and_poll(self):
        _, alice, _ = self.submit()
        self.submit(self.ballot("Bob"))
        self.assertEqual(self.submit(self.ballot("Bob"), {"X-Vote-Token": alice["voteToken"]})[0], 403)
        other = self.create_poll()
        other_ids = [date["id"] for date in other["dates"]]
        self.submit(self.ballot(ids=other_ids), poll_id=other["id"])
        self.assertEqual(self.submit(self.ballot(ids=other_ids), {"X-Vote-Token": alice["voteToken"]}, other["id"])[0], 403)

    def test_host_can_recover_legacy_ballot_and_revoke_old_receipt(self):
        _, data, _ = self.submit()
        with server.get_connection() as connection:
            connection.execute("UPDATE votes SET edit_token_hash = NULL")
        self.assertEqual(self.submit(headers={"X-Vote-Token": data["voteToken"]})[0], 403)
        status, recovered, _ = self.submit(headers=self.host)
        self.assertEqual(status, 201)
        self.assertNotEqual(recovered["voteToken"], data["voteToken"])
        self.assertEqual(self.submit(headers={"X-Vote-Token": data["voteToken"]})[0], 403)
        self.assertEqual(self.submit(headers={"X-Vote-Token": recovered["voteToken"]})[0], 201)

    def test_simultaneous_first_votes_cannot_overwrite_each_other(self):
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            results = list(executor.map(lambda _: self.submit(), range(2)))
        self.assertEqual(sorted(result[0] for result in results), [201, 403])
        self.assertEqual(len(self.get_poll()["votes"]), 1)

    def test_validation_and_static_files_remain_available(self):
        self.assertEqual(self.submit({"voterName": "", "rankings": self.ids})[0], 400)
        self.assertEqual(self.submit({"voterName": "Alice", "rankings": ["unknown"]})[0], 400)
        for path in server.STATIC_FILES:
            status, body, headers = self.request("GET", path)
            self.assertEqual(status, 200)
            self.assertNotIn(self.key, body)
            self.assertEqual(headers["Referrer-Policy"], "no-referrer")


class ConfigurationAndMigrationTests(unittest.TestCase):
    def test_startup_rejects_missing_short_or_whitespace_secret(self):
        for token in ("", "short", "x" * 32 + " ", "é" * 32):
            env = {**os.environ, "DND_VOTE_ADMIN_TOKEN": token}
            result = subprocess.run([sys.executable, "main.py"], env=env, capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 2)
            self.assertIn("DND_VOTE_ADMIN_TOKEN", result.stderr)
            if token:
                self.assertNotIn(token, result.stderr)

    def test_existing_database_migration_is_idempotent_and_keeps_ballots(self):
        with tempfile.TemporaryDirectory() as directory:
            db = Path(directory) / "legacy.db"
            with sqlite3.connect(db) as connection:
                connection.executescript("""
                    CREATE TABLE polls (id TEXT PRIMARY KEY, title TEXT NOT NULL, created_at TEXT NOT NULL);
                    CREATE TABLE votes (id TEXT PRIMARY KEY, poll_id TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
                                        voter_name TEXT NOT NULL, created_at TEXT NOT NULL);
                    INSERT INTO polls VALUES ('old-poll', 'Old campaign', '2026-01-01');
                    INSERT INTO votes VALUES ('old-vote', 'old-poll', 'Alice', '2026-01-01');
                """)
            with patch.multiple(server, DATA_DIR=Path(directory), DB_PATH=db):
                server.initialize_database()
                server.initialize_database()
                self.assertEqual(server.fetch_poll("old-poll")["votes"][0]["id"], "old-vote")
                with server.get_connection() as connection:
                    self.assertIsNone(connection.execute("SELECT edit_token_hash FROM votes").fetchone()[0])

    def test_valid_configuration_defaults_to_loopback(self):
        with patch.object(server, "ADMIN_TOKEN", "x" * 40), patch.dict(os.environ, {}, clear=True), \
             patch.object(sys, "argv", ["main.py"]), patch.object(server, "initialize_database"), \
             patch.object(server, "ThreadingHTTPServer") as http_server, patch("builtins.print"):
            server.main()
            self.assertEqual(http_server.call_args.args[0], ("127.0.0.1", 8000))
            http_server.return_value.serve_forever.assert_called_once()


if __name__ == "__main__":
    unittest.main()
