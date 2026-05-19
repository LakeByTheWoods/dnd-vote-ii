from __future__ import annotations

import json
import gc
import tempfile
import unittest
from http import HTTPStatus
from http.server import ThreadingHTTPServer
from pathlib import Path
from threading import Thread
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import server as dnd_server


class ApiResponse:
    def __init__(self, status: int, payload: dict):
        self.status = status
        self.payload = payload


class ServerBehaviorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.addCleanup(gc.collect)

        self.original_data_dir = dnd_server.DATA_DIR
        self.original_db_path = dnd_server.DB_PATH
        self.addCleanup(self.restore_database_paths)

        dnd_server.DATA_DIR = Path(self.temp_dir.name)
        dnd_server.DB_PATH = dnd_server.DATA_DIR / "dnd_vote.db"
        dnd_server.initialize_database()

        self.httpd = ThreadingHTTPServer(("127.0.0.1", 0), dnd_server.AppHandler)
        self.thread = Thread(target=self.httpd.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_server)

        host, port = self.httpd.server_address
        self.base_url = f"http://{host}:{port}"

    def restore_database_paths(self) -> None:
        dnd_server.DATA_DIR = self.original_data_dir
        dnd_server.DB_PATH = self.original_db_path

    def stop_server(self) -> None:
        self.httpd.shutdown()
        self.thread.join(timeout=5)
        self.httpd.server_close()

    def request(self, method: str, path: str, payload: dict | None = None) -> ApiResponse:
        body = None if payload is None else json.dumps(payload).encode("utf-8")
        request = Request(
            f"{self.base_url}{path}",
            data=body,
            method=method,
            headers={"Content-Type": "application/json"},
        )

        try:
            with urlopen(request, timeout=5) as response:
                return ApiResponse(response.status, json.loads(response.read().decode("utf-8")))
        except HTTPError as error:
            try:
                return ApiResponse(error.code, json.loads(error.read().decode("utf-8")))
            finally:
                error.close()

    def create_poll(self) -> dict:
        response = self.request(
            "POST",
            "/api/polls",
            {
                "title": "Waterdeep Campaign Night",
                "dates": ["2026-06-05", "2026-06-01", "2026-06-03"],
            },
        )

        self.assertEqual(HTTPStatus.CREATED, response.status)
        return response.payload["poll"]

    def test_poll_vote_replacement_results_and_disqualification(self) -> None:
        poll = self.create_poll()
        self.assertEqual("Waterdeep Campaign Night", poll["title"])
        self.assertEqual(["2026-06-01", "2026-06-03", "2026-06-05"], [date["value"] for date in poll["dates"]])
        self.assertEqual([], poll["votes"])

        fetch_response = self.request("GET", f"/api/polls/{poll['id']}")
        self.assertEqual(HTTPStatus.OK, fetch_response.status)
        self.assertEqual(poll["id"], fetch_response.payload["poll"]["id"])

        first, second, third = [date["id"] for date in poll["dates"]]
        vote_response = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {
                "voterName": "Alice",
                "rankings": [first, second],
                "unavailableDateIds": [third],
            },
        )
        self.assertEqual(HTTPStatus.CREATED, vote_response.status)
        self.assertEqual(1, len(vote_response.payload["poll"]["votes"]))

        replacement_response = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {
                "voterName": " alice ",
                "rankings": [second, first],
                "unavailableDateIds": [third],
            },
        )
        self.assertEqual(HTTPStatus.CREATED, replacement_response.status)

        replaced_votes = replacement_response.payload["poll"]["votes"]
        self.assertEqual(1, len(replaced_votes))
        self.assertEqual("alice", replaced_votes[0]["voterName"])
        self.assertEqual(
            [{"dateId": second, "position": 1}, {"dateId": first, "position": 2}],
            replaced_votes[0]["rankings"],
        )
        self.assertEqual([third], replaced_votes[0]["unavailableDateIds"])

        results_response = self.request("GET", f"/api/polls/{poll['id']}/results")
        self.assertEqual(HTTPStatus.OK, results_response.status)
        results = results_response.payload
        self.assertEqual(poll["id"], results["poll"]["id"])
        self.assertEqual(second, results["winner"]["id"])

        tallies_by_id = {entry["id"]: entry for entry in results["talliedDates"]}
        self.assertTrue(tallies_by_id[third]["disqualified"])
        self.assertEqual(["alice"], tallies_by_id[third]["unavailableBy"])

    def test_create_poll_validation_errors_are_predictable(self) -> None:
        missing_title = self.request("POST", "/api/polls", {"title": " ", "dates": ["2026-06-01", "2026-06-02"]})
        self.assertEqual(HTTPStatus.BAD_REQUEST, missing_title.status)
        self.assertEqual({"error": "Title is required."}, missing_title.payload)

        invalid_dates_type = self.request("POST", "/api/polls", {"title": "Game", "dates": "2026-06-01"})
        self.assertEqual(HTTPStatus.BAD_REQUEST, invalid_dates_type.status)
        self.assertEqual({"error": "Dates must be an array."}, invalid_dates_type.payload)

        too_few_dates = self.request("POST", "/api/polls", {"title": "Game", "dates": ["2026-06-01", "2026-06-01"]})
        self.assertEqual(HTTPStatus.BAD_REQUEST, too_few_dates.status)
        self.assertEqual({"error": "At least two unique dates are required."}, too_few_dates.payload)

    def test_vote_validation_errors_are_predictable(self) -> None:
        poll = self.create_poll()
        first, second, third = [date["id"] for date in poll["dates"]]

        malformed_arrays = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": first, "unavailableDateIds": []},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, malformed_arrays.status)
        self.assertEqual({"error": "Rankings and unavailableDateIds must be arrays."}, malformed_arrays.payload)

        empty_name = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": " ", "rankings": [first], "unavailableDateIds": []},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, empty_name.status)
        self.assertEqual({"error": "Voter name is required."}, empty_name.payload)

        duplicate_rankings = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": [first, first], "unavailableDateIds": []},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, duplicate_rankings.status)
        self.assertEqual({"error": "Ranked dates must be unique."}, duplicate_rankings.payload)

        contradictory_date = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": [second], "unavailableDateIds": [second]},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, contradictory_date.status)
        self.assertEqual({"error": "A date cannot be both ranked and unavailable."}, contradictory_date.payload)

        unknown_date = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": [third, "unknown-date-id"], "unavailableDateIds": []},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, unknown_date.status)
        self.assertEqual({"error": "Rankings include an unknown date."}, unknown_date.payload)


if __name__ == "__main__":
    unittest.main()
