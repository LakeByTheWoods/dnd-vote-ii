"""Run the production entry point with isolated deployment-like configuration.

These checks exercise local configuration, not a real host's proxy or volume.
"""
from contextlib import contextmanager
import http.client
import json
import os
from pathlib import Path
import queue
import sqlite3
import subprocess
import sys
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[1]
KEY = "startup-test-" + "a" * 40


@contextmanager
def running_server(directory, key=KEY, listener=None, arguments=()):
    env = {**os.environ, "DND_VOTE_ADMIN_TOKEN": key,
           "DND_VOTE_DATA_DIR": str(directory), "HOST": "127.0.0.1", "PORT": "0",
           **(listener or {})}
    with tempfile.TemporaryFile(mode="w+") as errors:
        process = subprocess.Popen([sys.executable, "-u", str(ROOT / "main.py"), *arguments],
                                   cwd=ROOT, env=env, stdout=subprocess.PIPE,
                                   stderr=errors, text=True)
        lines = queue.Queue()
        reader = threading.Thread(target=lambda: lines.put(process.stdout.readline()), daemon=True)
        reader.start()
        try:
            try:
                banner = lines.get(timeout=5).strip()
            except queue.Empty:
                raise AssertionError("Production entry point did not start within five seconds")
            if not banner.startswith("Serving D&D vote app on http://"):
                raise AssertionError("Production entry point failed to start")
            yield int(banner.rsplit(":", 1)[1]), banner
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
            reader.join(timeout=5)
            process.stdout.close()


def request(port, method, path, body=None, headers=None):
    connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
    try:
        connection.request(method, path, body=None if body is None else json.dumps(body),
                           headers={"Content-Type": "application/json", **(headers or {})})
        response = connection.getresponse()
        raw = response.read()
        data = json.loads(raw) if response.getheader("Content-Type", "").startswith("application/json") else raw
        return response.status, data
    finally:
        connection.close()


class ProductionStartupTests(unittest.TestCase):
    def test_environment_can_bind_external_listener_and_health_page(self):
        with tempfile.TemporaryDirectory() as directory:
            with running_server(directory, listener={"HOST": "0.0.0.0"}) as (port, banner):
                self.assertIn("http://0.0.0.0:", banner)
                self.assertEqual(request(port, "GET", "/")[0], 200)
                self.assertEqual(request(port, "GET", "/api/polls")[0], 401)

    def test_cli_listener_overrides_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            with running_server(directory, listener={"HOST": "invalid.example", "PORT": "1"},
                                arguments=("--host", "127.0.0.1", "--port", "0")) as (port, banner):
                self.assertIn("http://127.0.0.1:", banner)
                self.assertNotEqual(port, 1)
                self.assertEqual(request(port, "GET", "/")[0], 200)

    def test_polls_and_receipts_survive_restart_and_host_key_rotation(self):
        with tempfile.TemporaryDirectory() as directory:
            data_dir = Path(directory) / "private" / "persistent"
            host = {"Authorization": "Bearer " + KEY}
            with running_server(data_dir) as (port, _):
                status, data = request(port, "POST", "/api/polls",
                                       {"title": "Persistent campaign", "dates": ["2026-11-01", "2026-11-02"]}, host)
                self.assertEqual(status, 201)
                poll = data["poll"]
                ballot = {"voterName": "Alice", "rankings": [date["id"] for date in poll["dates"]]}
                path = f'/api/polls/{poll["id"]}/votes'
                status, voted = request(port, "POST", path, ballot)
                self.assertEqual(status, 201)
                receipt = voted["voteToken"]
                for private_path in ("/data/dnd_vote.db", "/server.py", "/.env"):
                    self.assertEqual(request(port, "GET", private_path)[0], 404)

            database = data_dir / "dnd_vote.db"
            self.assertTrue(database.is_file())
            with sqlite3.connect(database) as connection:
                stored = connection.execute("SELECT edit_token_hash FROM votes").fetchone()[0]
                self.assertEqual(len(stored), 64)
                self.assertNotEqual(stored, receipt)

            rotated_key = "rotated-test-" + "b" * 40
            with running_server(data_dir, key=rotated_key) as (port, _):
                self.assertEqual(request(port, "GET", "/api/polls", headers=host)[0], 401)
                status, listing = request(port, "GET", "/api/polls",
                                          headers={"Authorization": "Bearer " + rotated_key})
                self.assertEqual(status, 200)
                self.assertEqual([item["id"] for item in listing["polls"]], [poll["id"]])
                status, public = request(port, "GET", f'/api/polls/{poll["id"]}')
                self.assertEqual(status, 200)
                self.assertEqual(public["poll"], voted["poll"])
                self.assertNotIn(receipt, json.dumps(public))
                self.assertEqual(request(port, "POST", path, ballot)[0], 403)
                status, edited = request(port, "POST", path, ballot, {"X-Vote-Token": receipt})
                self.assertEqual(status, 201)
                self.assertEqual(edited["voteToken"], receipt)


if __name__ == "__main__":
    unittest.main()
