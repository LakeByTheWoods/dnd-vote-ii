package tally

import (
	"fmt"
	"strings"
	"testing"
)

func TestRankedPairsResults(t *testing.T) {
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
			name: "selects ordinary head to head winner",
			poll: pollFixture(
				[]Vote{
					voteFixture("Alice", []string{"date-a", "date-b", "date-c"}, nil),
					voteFixture("Bob", []string{"date-a", "date-c", "date-b"}, nil),
					voteFixture("Cara", []string{"date-b", "date-c", "date-a"}, nil),
				},
				nil,
			),
			expectedOrder:        []string{"date-a", "date-b", "date-c"},
			expectedRecords:      []string{"2-0-0", "1-1-0", "0-2-0"},
			expectedRanks:        []int{1, 2, 3},
			expectedDisqualified: []bool{false, false, false},
			expectedWinnerID:     "date-a",
		},
		{
			name: "skips weak cycle closing edge",
			poll: pollFixture(
				[]Vote{
					voteFixture("Alice", []string{"date-a", "date-b", "date-c"}, nil),
					voteFixture("Bob", []string{"date-a", "date-b", "date-c"}, nil),
					voteFixture("Cara", []string{"date-a", "date-b", "date-c"}, nil),
					voteFixture("Dan", []string{"date-a", "date-b", "date-c"}, nil),
					voteFixture("Eli", []string{"date-b", "date-c", "date-a"}, nil),
					voteFixture("Fran", []string{"date-b", "date-c", "date-a"}, nil),
					voteFixture("Gail", []string{"date-b", "date-c", "date-a"}, nil),
					voteFixture("Hank", []string{"date-c", "date-a", "date-b"}, nil),
					voteFixture("Ivy", []string{"date-c", "date-a", "date-b"}, nil),
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
			poll: pollFixture(
				[]Vote{
					voteFixture("Alice", []string{"date-a", "date-b"}, nil),
					voteFixture("Bob", []string{"date-b", "date-a"}, nil),
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
			poll: pollFixture(
				[]Vote{
					voteFixture("Alice", []string{"date-a"}, []string{"date-b", "date-c"}),
					voteFixture("Bob", []string{"date-a"}, []string{"date-b"}),
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
			poll: pollFixture(
				[]Vote{
					voteFixture("Alice", []string{"date-a"}, []string{"date-b"}),
					voteFixture("Bob", []string{"date-b"}, []string{"date-a"}),
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
			poll: pollFixture(
				[]Vote{
					voteFixture("Alice", []string{"date-a", "date-b"}, []string{"date-c"}),
					voteFixture("Bob", []string{"date-b", "date-a"}, nil),
					voteFixture("Cara", []string{"date-b", "date-a"}, nil),
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
			result := Calculate(test.poll)

			if got := resultIDs(result.Dates); !equalStrings(got, test.expectedOrder) {
				t.Fatalf("order = %#v, want %#v", got, test.expectedOrder)
			}
			if got := resultRecords(result.Dates); !equalStrings(got, test.expectedRecords) {
				t.Fatalf("records = %#v, want %#v", got, test.expectedRecords)
			}
			if got := resultRanks(result.Dates); !equalInts(got, test.expectedRanks) {
				t.Fatalf("ranks = %#v, want %#v", got, test.expectedRanks)
			}
			if got := resultDisqualified(result.Dates); !equalBools(got, test.expectedDisqualified) {
				t.Fatalf("disqualified = %#v, want %#v", got, test.expectedDisqualified)
			}
			if test.expectedWinnerID == "" {
				if result.Winner != nil {
					t.Fatalf("winner = %#v, want nil", result.Winner)
				}
			} else if result.Winner == nil || result.Winner.ID != test.expectedWinnerID {
				t.Fatalf("winner = %#v, want %q", result.Winner, test.expectedWinnerID)
			}
			for dateID, expectedNames := range test.expectedUnavailableBy {
				dateResult := resultByID(result.Dates, dateID)
				if dateResult == nil {
					t.Fatalf("missing result for unavailable date %q", dateID)
				}
				if !equalStrings(dateResult.UnavailableBy, expectedNames) {
					t.Fatalf("%s unavailableBy = %#v, want %#v", dateID, dateResult.UnavailableBy, expectedNames)
				}
			}
		})
	}
}

func pollFixture(votes []Vote, dateValues []string) Poll {
	if dateValues == nil {
		dateValues = []string{"2026-06-01", "2026-06-02", "2026-06-03"}
	}

	dates := make([]Date, 0, len(dateValues))
	for index, value := range dateValues {
		dateID := fmt.Sprintf("date-%c", 'a'+rune(index))
		dates = append(dates, Date{ID: dateID, Value: value})
	}

	return Poll{Dates: dates, Votes: votes}
}

func voteFixture(voterName string, rankings []string, unavailableDateIDs []string) Vote {
	voteRankings := make([]Ranking, 0, len(rankings))
	for index, dateID := range rankings {
		voteRankings = append(voteRankings, Ranking{DateID: dateID, Position: index + 1})
	}
	if unavailableDateIDs == nil {
		unavailableDateIDs = []string{}
	}

	return Vote{
		VoterName:          voterName,
		Rankings:           voteRankings,
		UnavailableDateIDs: unavailableDateIDs,
	}
}

func resultIDs(results []DateResult) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.ID)
	}
	return ids
}

func resultRecords(results []DateResult) []string {
	records := make([]string, 0, len(results))
	for _, result := range results {
		records = append(records, result.Record)
	}
	return records
}

func resultRanks(results []DateResult) []int {
	ranks := make([]int, 0, len(results))
	for _, result := range results {
		if result.Rank == nil {
			ranks = append(ranks, 0)
			continue
		}
		ranks = append(ranks, *result.Rank)
	}
	return ranks
}

func resultDisqualified(results []DateResult) []bool {
	values := make([]bool, 0, len(results))
	for _, result := range results {
		values = append(values, result.Disqualified)
	}
	return values
}

func resultByID(results []DateResult, dateID string) *DateResult {
	for index := range results {
		if results[index].ID == dateID {
			return &results[index]
		}
	}
	return nil
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

func TestUnavailableByNamesAreDeduplicatedPerDate(t *testing.T) {
	result := Calculate(Poll{
		Dates: []Date{{ID: "date-a", Value: "2026-06-01"}},
		Votes: []Vote{
			voteFixture("Alice", nil, []string{"date-a", "date-a"}),
			voteFixture("Alice", nil, []string{"date-a"}),
		},
	})

	if result.Winner != nil {
		t.Fatalf("winner = %#v, want nil", result.Winner)
	}
	if got := strings.Join(result.Dates[0].UnavailableBy, ","); got != "Alice" {
		t.Fatalf("unavailableBy = %q, want Alice", got)
	}
}
