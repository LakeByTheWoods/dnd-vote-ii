from __future__ import annotations

import json
import gc
import tempfile
import unittest
from datetime import date, timedelta
from http import HTTPStatus
from http.server import ThreadingHTTPServer
from pathlib import Path
from threading import Thread
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import server as dnd_server


def poll_fixture(votes: list[dict], date_values: list[str] | None = None) -> dict:
    dates = [
        {"id": "date-a", "value": "2026-06-01"},
        {"id": "date-b", "value": "2026-06-02"},
        {"id": "date-c", "value": "2026-06-03"},
    ]
    if date_values is not None:
        dates = [
            {"id": f"date-{letter}", "value": value}
            for letter, value in zip(("a", "b", "c", "d", "e"), date_values)
        ]

    return {
        "id": "poll-1",
        "title": "Campaign Night",
        "createdAt": "2026-05-19T00:00:00+00:00",
        "dates": dates,
        "votes": votes,
    }


def vote_fixture(
    voter_name: str,
    rankings: list[str],
    unavailable_date_ids: list[str] | None = None,
) -> dict:
    return {
        "id": f"vote-{voter_name.lower()}",
        "voterName": voter_name,
        "createdAt": "2026-05-19T00:00:00+00:00",
        "rankings": [
            {"dateId": date_id, "position": position}
            for position, date_id in enumerate(rankings, start=1)
        ],
        "unavailableDateIds": unavailable_date_ids or [],
    }


class TallyResultsTest(unittest.TestCase):
    def test_ranked_pairs_selects_normal_head_to_head_winner(self) -> None:
        poll = poll_fixture(
            [
                vote_fixture("Alice", ["date-a", "date-b", "date-c"]),
                vote_fixture("Bob", ["date-a", "date-c", "date-b"]),
                vote_fixture("Cara", ["date-b", "date-c", "date-a"]),
            ]
        )

        tallied_dates = dnd_server.tally_results(poll)

        self.assertEqual(["date-a", "date-b", "date-c"], [date["id"] for date in tallied_dates])
        self.assertEqual(
            ["2-0-0", "1-1-0", "0-2-0"],
            [date["record"] for date in tallied_dates],
        )
        self.assertEqual([1, 2, 3], [date["rank"] for date in tallied_dates])

    def test_ranked_pairs_skips_weak_cycle_closing_edge(self) -> None:
        poll = poll_fixture(
            [
                vote_fixture("Alice", ["date-a", "date-b", "date-c"]),
                vote_fixture("Bob", ["date-a", "date-b", "date-c"]),
                vote_fixture("Cara", ["date-a", "date-b", "date-c"]),
                vote_fixture("Dan", ["date-a", "date-b", "date-c"]),
                vote_fixture("Eli", ["date-b", "date-c", "date-a"]),
                vote_fixture("Fran", ["date-b", "date-c", "date-a"]),
                vote_fixture("Gail", ["date-b", "date-c", "date-a"]),
                vote_fixture("Hank", ["date-c", "date-a", "date-b"]),
                vote_fixture("Ivy", ["date-c", "date-a", "date-b"]),
            ],
            date_values=["2026-06-03", "2026-06-01", "2026-06-02"],
        )

        tallied_dates = dnd_server.tally_results(poll)

        self.assertEqual(["date-a", "date-b", "date-c"], [date["id"] for date in tallied_dates])
        self.assertEqual([1, 2, 3], [date["rank"] for date in tallied_dates])
        self.assertEqual(["1-1-0", "1-1-0", "1-1-0"], [date["record"] for date in tallied_dates])

    def test_ranked_pairs_ties_are_reported_and_ordered_by_date(self) -> None:
        poll = poll_fixture(
            [
                vote_fixture("Alice", ["date-a", "date-b"]),
                vote_fixture("Bob", ["date-b", "date-a"]),
            ],
            date_values=["2026-06-02", "2026-06-01"],
        )

        tallied_dates = dnd_server.tally_results(poll)

        self.assertEqual(["date-b", "date-a"], [date["id"] for date in tallied_dates])
        self.assertEqual(["0-0-1", "0-0-1"], [date["record"] for date in tallied_dates])
        self.assertEqual([1, 2], [date["rank"] for date in tallied_dates])

    def test_one_eligible_date_wins_without_pairwise_comparisons(self) -> None:
        poll = poll_fixture(
            [
                vote_fixture("Alice", ["date-a"], ["date-b", "date-c"]),
                vote_fixture("Bob", ["date-a"], ["date-b"]),
            ]
        )

        tallied_dates = dnd_server.tally_results(poll)

        self.assertEqual("date-a", tallied_dates[0]["id"])
        self.assertEqual(1, tallied_dates[0]["rank"])
        self.assertEqual("0-0-0", tallied_dates[0]["record"])
        self.assertFalse(tallied_dates[0]["disqualified"])
        self.assertTrue(all(date["disqualified"] for date in tallied_dates[1:]))

    def test_all_disqualified_dates_have_no_eligible_winner(self) -> None:
        poll = poll_fixture(
            [
                vote_fixture("Alice", ["date-a"], ["date-b"]),
                vote_fixture("Bob", ["date-b"], ["date-a"]),
            ],
            date_values=["2026-06-02", "2026-06-01"],
        )

        tallied_dates = dnd_server.tally_results(poll)

        self.assertEqual(["date-b", "date-a"], [date["id"] for date in tallied_dates])
        self.assertTrue(all(date["disqualified"] for date in tallied_dates))
        self.assertEqual([None, None], [date["rank"] for date in tallied_dates])

    def test_unavailable_dates_are_disqualified_before_head_to_head_tallying(self) -> None:
        poll = poll_fixture(
            [
                vote_fixture("Alice", ["date-a", "date-b"], ["date-c"]),
                vote_fixture("Bob", ["date-b", "date-a"], []),
                vote_fixture("Cara", ["date-b", "date-a"], []),
            ]
        )

        tallied_dates = dnd_server.tally_results(poll)

        tallies_by_id = {date["id"]: date for date in tallied_dates}
        self.assertEqual("date-b", tallied_dates[0]["id"])
        self.assertEqual("1-0-0", tallies_by_id["date-b"]["record"])
        self.assertEqual("0-1-0", tallies_by_id["date-a"]["record"])
        self.assertTrue(tallies_by_id["date-c"]["disqualified"])
        self.assertEqual(["Alice"], tallies_by_id["date-c"]["unavailableBy"])


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
        return self.raw_request(method, path, body)

    def raw_request(self, method: str, path: str, body: bytes | None = None) -> ApiResponse:
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

    def future_date_values(self) -> list[str]:
        base_date = date.today() + timedelta(days=30)
        return [(base_date + timedelta(days=offset)).isoformat() for offset in (0, 2, 4)]

    def create_poll(self) -> dict:
        date_values = self.future_date_values()
        response = self.request(
            "POST",
            "/api/polls",
            {
                "title": "Waterdeep Campaign Night",
                "dates": [date_values[2], date_values[0], date_values[1]],
            },
        )

        self.assertEqual(HTTPStatus.CREATED, response.status)
        return response.payload["poll"]

    def test_poll_vote_replacement_results_and_disqualification(self) -> None:
        poll = self.create_poll()
        self.assertEqual("Waterdeep Campaign Night", poll["title"])
        self.assertEqual(self.future_date_values(), [date["value"] for date in poll["dates"]])
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

    def test_results_api_counts_replaced_voter_name_as_one_current_ballot(self) -> None:
        poll = self.create_poll()
        first, second, third = [date["id"] for date in poll["dates"]]

        first_vote_response = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {
                "voterName": "Morgan",
                "rankings": [first, second, third],
                "unavailableDateIds": [],
            },
        )
        self.assertEqual(HTTPStatus.CREATED, first_vote_response.status)

        replacement_response = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {
                "voterName": " morgan ",
                "rankings": [second, first, third],
                "unavailableDateIds": [],
            },
        )
        self.assertEqual(HTTPStatus.CREATED, replacement_response.status)

        results_response = self.request("GET", f"/api/polls/{poll['id']}/results")

        self.assertEqual(HTTPStatus.OK, results_response.status)
        self.assertEqual({"poll", "talliedDates", "winner"}, set(results_response.payload))
        self.assertEqual(poll["id"], results_response.payload["poll"]["id"])
        self.assertEqual(1, len(results_response.payload["poll"]["votes"]))
        self.assertEqual(second, results_response.payload["winner"]["id"])

        tallies_by_id = {
            entry["id"]: entry
            for entry in results_response.payload["talliedDates"]
        }
        self.assertEqual("2-0-0", tallies_by_id[second]["record"])
        self.assertEqual("1-1-0", tallies_by_id[first]["record"])
        self.assertEqual("0-2-0", tallies_by_id[third]["record"])

    def test_create_poll_validation_errors_are_predictable(self) -> None:
        date_values = self.future_date_values()

        missing_title = self.request("POST", "/api/polls", {"title": " ", "dates": date_values[:2]})
        self.assertEqual(HTTPStatus.BAD_REQUEST, missing_title.status)
        self.assertEqual({"error": "Title is required."}, missing_title.payload)

        invalid_dates_type = self.request("POST", "/api/polls", {"title": "Game", "dates": date_values[0]})
        self.assertEqual(HTTPStatus.BAD_REQUEST, invalid_dates_type.status)
        self.assertEqual({"error": "Dates must be an array."}, invalid_dates_type.payload)

        too_few_dates = self.request("POST", "/api/polls", {"title": "Game", "dates": [date_values[0], date_values[0]]})
        self.assertEqual(HTTPStatus.BAD_REQUEST, too_few_dates.status)
        self.assertEqual({"error": "At least two unique dates are required."}, too_few_dates.payload)

    def test_create_poll_rejects_invalid_and_past_dates(self) -> None:
        date_values = self.future_date_values()
        yesterday = (date.today() - timedelta(days=1)).isoformat()

        invalid_date = self.request(
            "POST",
            "/api/polls",
            {"title": "Game", "dates": [date_values[0], "not-a-date"]},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, invalid_date.status)
        self.assertEqual({"error": "Dates must use YYYY-MM-DD format."}, invalid_date.payload)

        past_date = self.request(
            "POST",
            "/api/polls",
            {"title": "Game", "dates": [date_values[0], yesterday]},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, past_date.status)
        self.assertEqual({"error": "Candidate dates must be today or later."}, past_date.payload)

    def test_create_poll_rejects_blank_candidate_dates(self) -> None:
        date_values = self.future_date_values()

        blank_date = self.request(
            "POST",
            "/api/polls",
            {"title": "Game", "dates": [date_values[0], "  ", date_values[1]]},
        )

        self.assertEqual(HTTPStatus.BAD_REQUEST, blank_date.status)
        self.assertEqual({"error": "Dates must use YYYY-MM-DD format."}, blank_date.payload)

    def test_create_poll_rejects_oversized_title_and_request_body(self) -> None:
        date_values = self.future_date_values()

        oversized_title = self.request(
            "POST",
            "/api/polls",
            {"title": "G" * 121, "dates": date_values[:2]},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, oversized_title.status)
        self.assertEqual({"error": "Title must be 120 characters or fewer."}, oversized_title.payload)

        oversized_body = self.raw_request("POST", "/api/polls", b"{" + (b" " * 9000) + b"}")
        self.assertEqual(HTTPStatus.BAD_REQUEST, oversized_body.status)
        self.assertEqual({"error": "Request body must be 8192 bytes or fewer."}, oversized_body.payload)

    def test_create_poll_accepts_upcoming_dates_and_normalizes_duplicates(self) -> None:
        date_values = self.future_date_values()

        response = self.request(
            "POST",
            "/api/polls",
            {
                "title": " Game ",
                "dates": [date_values[1], date_values[0], date_values[1], f" {date_values[0]} "],
            },
        )

        self.assertEqual(HTTPStatus.CREATED, response.status)
        self.assertEqual("Game", response.payload["poll"]["title"])
        self.assertEqual(date_values[:2], [date["value"] for date in response.payload["poll"]["dates"]])

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

        oversized_name = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "B" * 81, "rankings": [first], "unavailableDateIds": []},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, oversized_name.status)
        self.assertEqual({"error": "Voter name must be 80 characters or fewer."}, oversized_name.payload)

        duplicate_rankings = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": [first, first], "unavailableDateIds": []},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, duplicate_rankings.status)
        self.assertEqual({"error": "Ranked dates must be unique."}, duplicate_rankings.payload)

        duplicate_unavailable = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": [first], "unavailableDateIds": [second, second]},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, duplicate_unavailable.status)
        self.assertEqual({"error": "Unavailable dates must be unique."}, duplicate_unavailable.payload)

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

        unknown_unavailable_date = self.request(
            "POST",
            f"/api/polls/{poll['id']}/votes",
            {"voterName": "Bob", "rankings": [first], "unavailableDateIds": ["unknown-date-id"]},
        )
        self.assertEqual(HTTPStatus.BAD_REQUEST, unknown_unavailable_date.status)
        self.assertEqual({"error": "Unavailable dates include an unknown date."}, unknown_unavailable_date.payload)

    def test_vote_rejects_non_string_voter_names(self) -> None:
        poll = self.create_poll()
        first = poll["dates"][0]["id"]

        for voter_name in (None, {"name": "Bob"}, ["Bob"], True, 42):
            with self.subTest(voter_name=voter_name):
                response = self.request(
                    "POST",
                    f"/api/polls/{poll['id']}/votes",
                    {"voterName": voter_name, "rankings": [first], "unavailableDateIds": []},
                )

                self.assertEqual(HTTPStatus.BAD_REQUEST, response.status)
                self.assertEqual({"error": "Voter name must be a string."}, response.payload)


if __name__ == "__main__":
    unittest.main()
