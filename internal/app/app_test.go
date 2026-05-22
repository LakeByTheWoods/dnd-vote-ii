package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestLoadConfigReadsEnvironmentWithLocalDefaults(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg, err := LoadConfigFromLookup(func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatalf("LoadConfigFromLookup returned error: %v", err)
		}

		if cfg.Host != "0.0.0.0" {
			t.Fatalf("Host = %q, want local bind default", cfg.Host)
		}
		if cfg.Port != "8000" {
			t.Fatalf("Port = %q, want 8000", cfg.Port)
		}
		if cfg.DataDir != "data" {
			t.Fatalf("DataDir = %q, want data", cfg.DataDir)
		}
		if cfg.DatabasePath != filepath.Join("data", "dnd_vote.db") {
			t.Fatalf("DatabasePath = %q, want default data database", cfg.DatabasePath)
		}
		if cfg.AdminSecret != "" {
			t.Fatalf("AdminSecret = %q, want empty local default", cfg.AdminSecret)
		}
		if cfg.MaxRequestBodyBytes != 8192 {
			t.Fatalf("MaxRequestBodyBytes = %d, want 8192", cfg.MaxRequestBodyBytes)
		}
	})

	t.Run("environment overrides", func(t *testing.T) {
		env := map[string]string{
			"DND_VOTE_HOST":                   "127.0.0.1",
			"DND_VOTE_PORT":                   "9000",
			"DND_VOTE_DATA_DIR":               filepath.Join("tmp", "data"),
			"DND_VOTE_DB_PATH":                filepath.Join("tmp", "custom.db"),
			"DND_VOTE_ADMIN_SECRET":           "secret",
			"DND_VOTE_MAX_REQUEST_BODY_BYTES": "1024",
		}

		cfg, err := LoadConfigFromLookup(func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		})
		if err != nil {
			t.Fatalf("LoadConfigFromLookup returned error: %v", err)
		}

		if cfg.Host != "127.0.0.1" || cfg.Port != "9000" {
			t.Fatalf("address config = %s:%s, want 127.0.0.1:9000", cfg.Host, cfg.Port)
		}
		if cfg.DataDir != filepath.Join("tmp", "data") {
			t.Fatalf("DataDir = %q", cfg.DataDir)
		}
		if cfg.DatabasePath != filepath.Join("tmp", "custom.db") {
			t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
		}
		if cfg.AdminSecret != "secret" {
			t.Fatalf("AdminSecret = %q", cfg.AdminSecret)
		}
		if cfg.MaxRequestBodyBytes != 1024 {
			t.Fatalf("MaxRequestBodyBytes = %d", cfg.MaxRequestBodyBytes)
		}
	})
}

func TestOpenStoreCreatesFreshSQLiteSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "dnd_vote.db")

	store, err := OpenStore(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("querying sqlite schema: %v", err)
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning schema row: %v", err)
		}
		tables[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema rows error: %v", err)
	}

	for _, table := range []string{"polls", "poll_dates", "votes", "vote_rankings", "vote_unavailability"} {
		if !tables[table] {
			t.Fatalf("schema is missing table %s; got %#v", table, tables)
		}
	}
}

func TestRouterServesExistingStaticAppShell(t *testing.T) {
	handler := testRouter(t, 8192)

	for _, test := range []struct {
		path     string
		contains string
	}{
		{path: "/", contains: "Dungeons & Dates"},
		{path: "/index.html", contains: "New Poll"},
		{path: "/vote.html", contains: "Submit Vote"},
		{path: "/results.html", contains: "Results"},
		{path: "/styles.css", contains: "page-shell"},
		{path: "/shared.js", contains: "window.DndVoteApp"},
		{path: "/host.js", contains: "createPoll"},
		{path: "/vote.js", contains: "saveVote"},
		{path: "/results.js", contains: "getResults"},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET %s returned status %d", test.path, response.Code)
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("GET %s did not include %q", test.path, test.contains)
			}
		})
	}
}

func TestJSONAPIRequestsEnforceMaximumBodySize(t *testing.T) {
	handler := testRouter(t, 32)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/polls",
		strings.NewReader(`{"title":"`+strings.Repeat("x", 64)+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response was not JSON: %v", err)
	}
	if payload["error"] != "Request body must be 32 bytes or fewer." {
		t.Fatalf("error payload = %#v", payload)
	}
}

func TestPollCreationFetchResultsAndListAPIs(t *testing.T) {
	handler := testRouter(t, 8192)
	dates := futureDateValues()

	createResponse := requestJSON(t, handler, http.MethodPost, "/api/polls", map[string]any{
		"title": " Waterdeep Campaign Night ",
		"dates": []any{dates[2], dates[0], dates[1], " " + dates[0] + " "},
	})
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	var created struct {
		Poll            pollPayload `json:"poll"`
		ManagementToken string      `json:"managementToken"`
	}
	decodeJSON(t, createResponse, &created)
	if created.ManagementToken == "" || len(created.ManagementToken) < 24 {
		t.Fatalf("management token was not generated securely enough: %q", created.ManagementToken)
	}
	if created.Poll.ManagementToken != "" {
		t.Fatalf("created poll leaked management token: %#v", created.Poll)
	}
	if created.Poll.Title != "Waterdeep Campaign Night" {
		t.Fatalf("title = %q", created.Poll.Title)
	}
	if got := pollDateValues(created.Poll.Dates); !equalStrings(got, dates) {
		t.Fatalf("dates = %#v, want %#v", got, dates)
	}
	if len(created.Poll.Votes) != 0 {
		t.Fatalf("created poll votes = %#v, want none", created.Poll.Votes)
	}

	fetchResponse := requestJSON(t, handler, http.MethodGet, "/api/polls/"+created.Poll.ID, nil)
	if fetchResponse.Code != http.StatusOK {
		t.Fatalf("fetch status = %d, want %d", fetchResponse.Code, http.StatusOK)
	}
	var fetched struct {
		Poll pollPayload `json:"poll"`
	}
	decodeJSON(t, fetchResponse, &fetched)
	if fetched.Poll.ID != created.Poll.ID {
		t.Fatalf("fetched poll id = %q, want %q", fetched.Poll.ID, created.Poll.ID)
	}
	if bytes.Contains(fetchResponse.Body.Bytes(), []byte(created.ManagementToken)) {
		t.Fatalf("fetch response leaked management token: %s", fetchResponse.Body.String())
	}

	resultsResponse := requestJSON(t, handler, http.MethodGet, "/api/polls/"+created.Poll.ID+"/results", nil)
	if resultsResponse.Code != http.StatusOK {
		t.Fatalf("results status = %d, want %d", resultsResponse.Code, http.StatusOK)
	}
	if bytes.Contains(resultsResponse.Body.Bytes(), []byte(created.ManagementToken)) {
		t.Fatalf("results response leaked management token: %s", resultsResponse.Body.String())
	}

	listResponse := requestJSON(t, handler, http.MethodGet, "/api/polls", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResponse.Code, http.StatusOK)
	}
	var listed struct {
		Polls []pollSummaryPayload `json:"polls"`
	}
	decodeJSON(t, listResponse, &listed)
	if len(listed.Polls) != 1 {
		t.Fatalf("poll summaries length = %d, want 1", len(listed.Polls))
	}
	summary := listed.Polls[0]
	if summary.ID != created.Poll.ID || summary.Title != created.Poll.Title || summary.DateCount != 3 || summary.VoteCount != 0 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestVoteCreationReplacementAndResultsAPIs(t *testing.T) {
	handler := testRouter(t, 8192)
	created := createTestPoll(t, handler)
	first, second, third := created.Poll.Dates[0].ID, created.Poll.Dates[1].ID, created.Poll.Dates[2].ID

	voteResponse := requestJSON(t, handler, http.MethodPost, "/api/polls/"+created.Poll.ID+"/votes", map[string]any{
		"voterName":          "Alice",
		"rankings":           []any{first, second},
		"unavailableDateIds": []any{third},
	})
	if voteResponse.Code != http.StatusCreated {
		t.Fatalf("vote status = %d, want %d; body: %s", voteResponse.Code, http.StatusCreated, voteResponse.Body.String())
	}
	var voted struct {
		Poll pollPayload `json:"poll"`
	}
	decodeJSON(t, voteResponse, &voted)
	if len(voted.Poll.Votes) != 1 {
		t.Fatalf("votes length = %d, want 1", len(voted.Poll.Votes))
	}
	if voted.Poll.Votes[0].VoterName != "Alice" {
		t.Fatalf("voter name = %q, want Alice", voted.Poll.Votes[0].VoterName)
	}
	if got := voted.Poll.Votes[0].RankingDateIDs(); !equalStrings(got, []string{first, second}) {
		t.Fatalf("rankings = %#v, want %#v", got, []string{first, second})
	}
	if got := voted.Poll.Votes[0].UnavailableDateIDs; !equalStrings(got, []string{third}) {
		t.Fatalf("unavailable = %#v, want %#v", got, []string{third})
	}

	replacementResponse := requestJSON(t, handler, http.MethodPost, "/api/polls/"+created.Poll.ID+"/votes", map[string]any{
		"voterName":          " alice ",
		"rankings":           []any{second, first},
		"unavailableDateIds": []any{},
	})
	if replacementResponse.Code != http.StatusCreated {
		t.Fatalf("replacement status = %d, want %d; body: %s", replacementResponse.Code, http.StatusCreated, replacementResponse.Body.String())
	}
	var replaced struct {
		Poll pollPayload `json:"poll"`
	}
	decodeJSON(t, replacementResponse, &replaced)
	if len(replaced.Poll.Votes) != 1 {
		t.Fatalf("replacement votes length = %d, want 1: %#v", len(replaced.Poll.Votes), replaced.Poll.Votes)
	}
	replacedVote := replaced.Poll.Votes[0]
	if replacedVote.ID == voted.Poll.Votes[0].ID {
		t.Fatalf("replacement kept old vote id %q", replacedVote.ID)
	}
	if replacedVote.VoterName != "alice" {
		t.Fatalf("replacement voter name = %q, want trimmed submitted casing", replacedVote.VoterName)
	}
	if got := replacedVote.RankingDateIDs(); !equalStrings(got, []string{second, first}) {
		t.Fatalf("replacement rankings = %#v, want %#v", got, []string{second, first})
	}
	if len(replacedVote.UnavailableDateIDs) != 0 {
		t.Fatalf("replacement unavailable = %#v, want none", replacedVote.UnavailableDateIDs)
	}

	fetchResponse := requestJSON(t, handler, http.MethodGet, "/api/polls/"+created.Poll.ID, nil)
	var fetched struct {
		Poll pollPayload `json:"poll"`
	}
	decodeJSON(t, fetchResponse, &fetched)
	if len(fetched.Poll.Votes) != 1 || !equalStrings(fetched.Poll.Votes[0].RankingDateIDs(), []string{second, first}) {
		t.Fatalf("fetched votes did not reflect replacement: %#v", fetched.Poll.Votes)
	}

	listResponse := requestJSON(t, handler, http.MethodGet, "/api/polls", nil)
	var listed struct {
		Polls []pollSummaryPayload `json:"polls"`
	}
	decodeJSON(t, listResponse, &listed)
	if listed.Polls[0].VoteCount != 1 {
		t.Fatalf("vote count = %d, want one current ballot", listed.Polls[0].VoteCount)
	}

	resultsResponse := requestJSON(t, handler, http.MethodGet, "/api/polls/"+created.Poll.ID+"/results", nil)
	if resultsResponse.Code != http.StatusOK {
		t.Fatalf("results status = %d, want %d; body: %s", resultsResponse.Code, http.StatusOK, resultsResponse.Body.String())
	}
	var results struct {
		Poll         pollPayload          `json:"poll"`
		TalliedDates []talliedDatePayload `json:"talliedDates"`
		Winner       talliedDatePayload   `json:"winner"`
	}
	decodeJSON(t, resultsResponse, &results)
	if len(results.Poll.Votes) != 1 {
		t.Fatalf("results poll votes length = %d, want 1", len(results.Poll.Votes))
	}
	if results.Winner.ID != second {
		t.Fatalf("winner = %q, want %q", results.Winner.ID, second)
	}
}

func TestTallyResultsRankedPairsCases(t *testing.T) {
	tests := []struct {
		name                  string
		poll                  Poll
		expectedOrder         []string
		expectedRecords       []string
		expectedRanks         []int
		expectedDisqualified  []bool
		expectedWinnerID      string
		expectedUnavailableBy map[string][]string
	}{
		{
			name: "skips weak cycle closing edge",
			poll: tallyPollFixture(
				[]Vote{
					tallyVoteFixture("Alice", []string{"date-a", "date-b", "date-c"}, nil),
					tallyVoteFixture("Bob", []string{"date-a", "date-b", "date-c"}, nil),
					tallyVoteFixture("Cara", []string{"date-a", "date-b", "date-c"}, nil),
					tallyVoteFixture("Dan", []string{"date-a", "date-b", "date-c"}, nil),
					tallyVoteFixture("Eli", []string{"date-b", "date-c", "date-a"}, nil),
					tallyVoteFixture("Fran", []string{"date-b", "date-c", "date-a"}, nil),
					tallyVoteFixture("Gail", []string{"date-b", "date-c", "date-a"}, nil),
					tallyVoteFixture("Hank", []string{"date-c", "date-a", "date-b"}, nil),
					tallyVoteFixture("Ivy", []string{"date-c", "date-a", "date-b"}, nil),
				},
				[]string{"2026-06-03", "2026-06-01", "2026-06-02"},
			),
			expectedOrder:        []string{"date-a", "date-b", "date-c"},
			expectedRecords:      []string{"1-1-0", "1-1-0", "1-1-0"},
			expectedRanks:        []int{1, 2, 3},
			expectedDisqualified: []bool{false, false, false},
			expectedWinnerID:     "date-a",
		},
		{
			name: "reports ties and orders by date",
			poll: tallyPollFixture(
				[]Vote{
					tallyVoteFixture("Alice", []string{"date-a", "date-b"}, nil),
					tallyVoteFixture("Bob", []string{"date-b", "date-a"}, nil),
				},
				[]string{"2026-06-02", "2026-06-01"},
			),
			expectedOrder:        []string{"date-b", "date-a"},
			expectedRecords:      []string{"0-0-1", "0-0-1"},
			expectedRanks:        []int{1, 2},
			expectedDisqualified: []bool{false, false},
			expectedWinnerID:     "date-b",
		},
		{
			name: "one eligible date wins without pairwise comparisons",
			poll: tallyPollFixture(
				[]Vote{
					tallyVoteFixture("Alice", []string{"date-a"}, []string{"date-b", "date-c"}),
					tallyVoteFixture("Bob", []string{"date-a"}, []string{"date-b"}),
				},
				nil,
			),
			expectedOrder:        []string{"date-a", "date-b", "date-c"},
			expectedRecords:      []string{"0-0-0", "0-0-0", "0-0-0"},
			expectedRanks:        []int{1, 0, 0},
			expectedDisqualified: []bool{false, true, true},
			expectedWinnerID:     "date-a",
			expectedUnavailableBy: map[string][]string{
				"date-b": {"Alice", "Bob"},
				"date-c": {"Alice"},
			},
		},
		{
			name: "all disqualified dates have no eligible winner",
			poll: tallyPollFixture(
				[]Vote{
					tallyVoteFixture("Alice", []string{"date-a"}, []string{"date-b"}),
					tallyVoteFixture("Bob", []string{"date-b"}, []string{"date-a"}),
				},
				[]string{"2026-06-02", "2026-06-01"},
			),
			expectedOrder:        []string{"date-b", "date-a"},
			expectedRecords:      []string{"0-0-0", "0-0-0"},
			expectedRanks:        []int{0, 0},
			expectedDisqualified: []bool{true, true},
			expectedWinnerID:     "",
			expectedUnavailableBy: map[string][]string{
				"date-a": {"Bob"},
				"date-b": {"Alice"},
			},
		},
		{
			name: "unavailable dates are disqualified before head to head tallying",
			poll: tallyPollFixture(
				[]Vote{
					tallyVoteFixture("Alice", []string{"date-a", "date-b"}, []string{"date-c"}),
					tallyVoteFixture("Bob", []string{"date-b", "date-a"}, nil),
					tallyVoteFixture("Cara", []string{"date-b", "date-a"}, nil),
				},
				nil,
			),
			expectedOrder:        []string{"date-b", "date-a", "date-c"},
			expectedRecords:      []string{"1-0-0", "0-1-0", "0-0-0"},
			expectedRanks:        []int{1, 2, 0},
			expectedDisqualified: []bool{false, false, true},
			expectedWinnerID:     "date-b",
			expectedUnavailableBy: map[string][]string{
				"date-c": {"Alice"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			talliedDates, winner := TallyResults(test.poll)

			if got := tallyIDs(talliedDates); !equalStrings(got, test.expectedOrder) {
				t.Fatalf("order = %#v, want %#v", got, test.expectedOrder)
			}
			if got := tallyRecords(talliedDates); !equalStrings(got, test.expectedRecords) {
				t.Fatalf("records = %#v, want %#v", got, test.expectedRecords)
			}
			if got := tallyRankValues(talliedDates); !equalInts(got, test.expectedRanks) {
				t.Fatalf("ranks = %#v, want %#v", got, test.expectedRanks)
			}
			if got := tallyDisqualifiedValues(talliedDates); !equalBools(got, test.expectedDisqualified) {
				t.Fatalf("disqualified = %#v, want %#v", got, test.expectedDisqualified)
			}
			if test.expectedWinnerID == "" {
				if winner != nil {
					t.Fatalf("winner = %#v, want nil", winner)
				}
			} else if winner == nil || winner.ID != test.expectedWinnerID {
				t.Fatalf("winner = %#v, want %q", winner, test.expectedWinnerID)
			}
			for dateID, expectedNames := range test.expectedUnavailableBy {
				tally := tallyByID(talliedDates, dateID)
				if tally == nil {
					t.Fatalf("missing tally for unavailable date %q", dateID)
				}
				if !equalStrings(tally.UnavailableBy, expectedNames) {
					t.Fatalf("%s unavailableBy = %#v, want %#v", dateID, tally.UnavailableBy, expectedNames)
				}
			}
		})
	}
}

func TestVoteValidationErrorsArePredictable(t *testing.T) {
	handler := testRouter(t, 8192)
	created := createTestPoll(t, handler)
	first, second, third := created.Poll.Dates[0].ID, created.Poll.Dates[1].ID, created.Poll.Dates[2].ID

	tests := []struct {
		name    string
		payload map[string]any
		error   string
	}{
		{
			name: "rankings must be array",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           first,
				"unavailableDateIds": []any{},
			},
			error: "Rankings and unavailableDateIds must be arrays.",
		},
		{
			name: "unavailable must be array",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           []any{first},
				"unavailableDateIds": second,
			},
			error: "Rankings and unavailableDateIds must be arrays.",
		},
		{
			name: "blank voter name",
			payload: map[string]any{
				"voterName":          " ",
				"rankings":           []any{first},
				"unavailableDateIds": []any{},
			},
			error: "Voter name is required.",
		},
		{
			name: "oversized voter name",
			payload: map[string]any{
				"voterName":          strings.Repeat("B", 81),
				"rankings":           []any{first},
				"unavailableDateIds": []any{},
			},
			error: "Voter name must be 80 characters or fewer.",
		},
		{
			name: "duplicate rankings",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           []any{first, first},
				"unavailableDateIds": []any{},
			},
			error: "Ranked dates must be unique.",
		},
		{
			name: "duplicate unavailable dates",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           []any{first},
				"unavailableDateIds": []any{second, second},
			},
			error: "Unavailable dates must be unique.",
		},
		{
			name: "contradictory date",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           []any{second},
				"unavailableDateIds": []any{second},
			},
			error: "A date cannot be both ranked and unavailable.",
		},
		{
			name: "unknown ranked date",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           []any{third, "unknown-date-id"},
				"unavailableDateIds": []any{},
			},
			error: "Rankings include an unknown date.",
		},
		{
			name: "unknown unavailable date",
			payload: map[string]any{
				"voterName":          "Bob",
				"rankings":           []any{first},
				"unavailableDateIds": []any{"unknown-date-id"},
			},
			error: "Unavailable dates include an unknown date.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestJSON(t, handler, http.MethodPost, "/api/polls/"+created.Poll.ID+"/votes", test.payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			var payload map[string]string
			decodeJSON(t, response, &payload)
			if payload["error"] != test.error {
				t.Fatalf("error = %q, want %q", payload["error"], test.error)
			}
		})
	}
}

func TestVoteRejectsNonStringVoterNames(t *testing.T) {
	handler := testRouter(t, 8192)
	created := createTestPoll(t, handler)
	first := created.Poll.Dates[0].ID

	for _, voterName := range []any{nil, map[string]any{"name": "Bob"}, []any{"Bob"}, true, float64(42)} {
		t.Run(fmt.Sprintf("%T", voterName), func(t *testing.T) {
			response := requestJSON(t, handler, http.MethodPost, "/api/polls/"+created.Poll.ID+"/votes", map[string]any{
				"voterName":          voterName,
				"rankings":           []any{first},
				"unavailableDateIds": []any{},
			})
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			var payload map[string]string
			decodeJSON(t, response, &payload)
			if payload["error"] != "Voter name must be a string." {
				t.Fatalf("error = %q, want voter-name type error", payload["error"])
			}
		})
	}
}

func TestCreatePollValidationErrorsArePredictable(t *testing.T) {
	handler := testRouter(t, 8192)
	dates := futureDateValues()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	tests := []struct {
		name    string
		payload map[string]any
		error   string
	}{
		{
			name:    "blank title",
			payload: map[string]any{"title": " ", "dates": dates[:2]},
			error:   "Title is required.",
		},
		{
			name:    "oversized title",
			payload: map[string]any{"title": strings.Repeat("G", 121), "dates": dates[:2]},
			error:   "Title must be 120 characters or fewer.",
		},
		{
			name:    "dates must be array",
			payload: map[string]any{"title": "Game", "dates": dates[0]},
			error:   "Dates must be an array.",
		},
		{
			name:    "too few unique dates",
			payload: map[string]any{"title": "Game", "dates": []any{dates[0], " " + dates[0] + " "}},
			error:   "At least two unique dates are required.",
		},
		{
			name:    "invalid date value",
			payload: map[string]any{"title": "Game", "dates": []any{dates[0], "not-a-date"}},
			error:   "Dates must use YYYY-MM-DD format.",
		},
		{
			name:    "past date",
			payload: map[string]any{"title": "Game", "dates": []any{dates[0], yesterday}},
			error:   "Candidate dates must be today or later.",
		},
		{
			name:    "invalid date payload type",
			payload: map[string]any{"title": "Game", "dates": []any{dates[0], map[string]any{"date": dates[1]}}},
			error:   "Dates must use YYYY-MM-DD format.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestJSON(t, handler, http.MethodPost, "/api/polls", test.payload)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			var payload map[string]string
			decodeJSON(t, response, &payload)
			if payload["error"] != test.error {
				t.Fatalf("error = %q, want %q", payload["error"], test.error)
			}
		})
	}
}

func TestNewHTTPServerSetsProductionTimeouts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = "0"

	server := NewHTTPServer(cfg, http.NewServeMux())

	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q, want 127.0.0.1:0", server.Addr)
	}
	for name, value := range map[string]time.Duration{
		"ReadHeaderTimeout": server.ReadHeaderTimeout,
		"ReadTimeout":       server.ReadTimeout,
		"WriteTimeout":      server.WriteTimeout,
		"IdleTimeout":       server.IdleTimeout,
	} {
		if value <= 0 {
			t.Fatalf("%s = %s, want a configured timeout", name, value)
		}
	}
}

func testRouter(t *testing.T, maxBodyBytes int64) http.Handler {
	t.Helper()

	store, err := OpenStore(context.Background(), filepath.Join(t.TempDir(), "dnd_vote.db"))
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	cfg := DefaultConfig()
	cfg.StaticDir = repoRoot(t)
	cfg.MaxRequestBodyBytes = maxBodyBytes

	return NewRouter(cfg, store)
}

type pollPayload struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	CreatedAt       string        `json:"createdAt"`
	Dates           []datePayload `json:"dates"`
	Votes           []votePayload `json:"votes"`
	ManagementToken string        `json:"managementToken"`
}

type datePayload struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type votePayload struct {
	ID                 string               `json:"id"`
	VoterName          string               `json:"voterName"`
	CreatedAt          string               `json:"createdAt"`
	Rankings           []voteRankingPayload `json:"rankings"`
	UnavailableDateIDs []string             `json:"unavailableDateIds"`
}

func (v votePayload) RankingDateIDs() []string {
	dateIDs := make([]string, 0, len(v.Rankings))
	for _, ranking := range v.Rankings {
		dateIDs = append(dateIDs, ranking.DateID)
	}
	return dateIDs
}

type voteRankingPayload struct {
	DateID   string `json:"dateId"`
	Position int    `json:"position"`
}

func tallyPollFixture(votes []Vote, dateValues []string) Poll {
	if dateValues == nil {
		dateValues = []string{"2026-06-01", "2026-06-02", "2026-06-03"}
	}

	dates := make([]PollDate, 0, len(dateValues))
	for index, value := range dateValues {
		dateID := fmt.Sprintf("date-%c", 'a'+rune(index))
		dates = append(dates, PollDate{ID: dateID, Value: value})
	}

	return Poll{
		ID:        "poll-1",
		Title:     "Campaign Night",
		CreatedAt: "2026-05-19T00:00:00Z",
		Dates:     dates,
		Votes:     votes,
	}
}

func tallyVoteFixture(voterName string, rankings []string, unavailableDateIDs []string) Vote {
	voteRankings := make([]VoteRanking, 0, len(rankings))
	for index, dateID := range rankings {
		voteRankings = append(voteRankings, VoteRanking{DateID: dateID, Position: index + 1})
	}
	if unavailableDateIDs == nil {
		unavailableDateIDs = []string{}
	}

	return Vote{
		ID:                 "vote-" + strings.ToLower(voterName),
		VoterName:          voterName,
		CreatedAt:          "2026-05-19T00:00:00Z",
		Rankings:           voteRankings,
		UnavailableDateIDs: unavailableDateIDs,
	}
}

func tallyIDs(talliedDates []TalliedDate) []string {
	ids := make([]string, 0, len(talliedDates))
	for _, tally := range talliedDates {
		ids = append(ids, tally.ID)
	}
	return ids
}

func tallyRecords(talliedDates []TalliedDate) []string {
	records := make([]string, 0, len(talliedDates))
	for _, tally := range talliedDates {
		records = append(records, tally.Record)
	}
	return records
}

func tallyRankValues(talliedDates []TalliedDate) []int {
	ranks := make([]int, 0, len(talliedDates))
	for _, tally := range talliedDates {
		if tally.Rank == nil {
			ranks = append(ranks, 0)
			continue
		}
		ranks = append(ranks, *tally.Rank)
	}
	return ranks
}

func tallyDisqualifiedValues(talliedDates []TalliedDate) []bool {
	values := make([]bool, 0, len(talliedDates))
	for _, tally := range talliedDates {
		values = append(values, tally.Disqualified)
	}
	return values
}

func tallyByID(talliedDates []TalliedDate, dateID string) *TalliedDate {
	for index := range talliedDates {
		if talliedDates[index].ID == dateID {
			return &talliedDates[index]
		}
	}
	return nil
}

type pollSummaryPayload struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	DateCount int    `json:"dateCount"`
	VoteCount int    `json:"voteCount"`
}

type talliedDatePayload struct {
	ID             string   `json:"id"`
	Value          string   `json:"value"`
	PairwiseWins   int      `json:"pairwiseWins"`
	PairwiseLosses int      `json:"pairwiseLosses"`
	PairwiseTies   int      `json:"pairwiseTies"`
	Record         string   `json:"record"`
	Rank           int      `json:"rank"`
	UnavailableBy  []string `json:"unavailableBy"`
	Disqualified   bool     `json:"disqualified"`
}

type createdPollPayload struct {
	Poll            pollPayload `json:"poll"`
	ManagementToken string      `json:"managementToken"`
}

func createTestPoll(t *testing.T, handler http.Handler) createdPollPayload {
	t.Helper()

	dates := futureDateValues()
	response := requestJSON(t, handler, http.MethodPost, "/api/polls", map[string]any{
		"title": "Waterdeep Campaign Night",
		"dates": []any{dates[2], dates[0], dates[1]},
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var created createdPollPayload
	decodeJSON(t, response, &created)
	return created
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if payload != nil {
		content, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal returned error: %v", err)
		}
		body = bytes.NewReader(content)
	}

	request := httptest.NewRequest(method, path, body)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, payload any) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), payload); err != nil {
		t.Fatalf("response was not JSON: %v; body: %s", err, response.Body.String())
	}
}

func futureDateValues() []string {
	base := time.Now().AddDate(0, 0, 30)
	return []string{
		base.Format("2006-01-02"),
		base.AddDate(0, 0, 2).Format("2006-01-02"),
		base.AddDate(0, 0, 4).Format("2006-01-02"),
	}
}

func pollDateValues(dates []datePayload) []string {
	values := make([]string, 0, len(dates))
	for _, date := range dates {
		values = append(values, date.Value)
	}
	return values
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalInts(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalBools(left []bool, right []bool) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
