package tally

import (
	"fmt"
	"sort"
)

type Poll struct {
	Dates []Date
	Votes []Vote
}

type Date struct {
	ID    string
	Value string
}

type Vote struct {
	VoterName          string
	Rankings           []Ranking
	UnavailableDateIDs []string
}

type Ranking struct {
	DateID   string
	Position int
}

type Result struct {
	Dates  []DateResult
	Winner *DateResult
}

type DateResult struct {
	ID             string
	Value          string
	PairwiseWins   int
	PairwiseLosses int
	PairwiseTies   int
	Record         string
	Rank           *int
	UnavailableBy  []string
	Disqualified   bool
}

func Calculate(poll Poll) Result {
	tallies := make([]DateResult, 0, len(poll.Dates))
	for _, date := range poll.Dates {
		tally := DateResult{
			ID:            date.ID,
			Value:         date.Value,
			Record:        "0-0-0",
			UnavailableBy: []string{},
		}
		tallies = append(tallies, tally)
	}
	byID := map[string]*DateResult{}
	for index := range tallies {
		byID[tallies[index].ID] = &tallies[index]
	}

	for _, vote := range poll.Votes {
		for _, dateID := range vote.UnavailableDateIDs {
			tally := byID[dateID]
			if tally == nil {
				continue
			}
			tally.Disqualified = true
			if !containsString(tally.UnavailableBy, vote.VoterName) {
				tally.UnavailableBy = append(tally.UnavailableBy, vote.VoterName)
			}
		}
	}

	eligibleIDs := []string{}
	for _, tally := range tallies {
		if !tally.Disqualified {
			eligibleIDs = append(eligibleIDs, tally.ID)
		}
	}
	if len(eligibleIDs) <= 1 {
		for index := range tallies {
			if !tallies[index].Disqualified {
				rank := 1
				tallies[index].Rank = &rank
			}
			tallies[index].Record = formatRecord(tallies[index])
		}
		sortTallies(tallies)
		return Result{Dates: tallies, Winner: firstEligibleTally(tallies)}
	}

	preferenceMatrix := buildPreferenceMatrix(poll.Votes, eligibleIDs)
	pairResults := buildPairResults(preferenceMatrix, eligibleIDs)

	for _, pair := range pairResults {
		byID[pair.WinnerID].PairwiseWins++
		byID[pair.LoserID].PairwiseLosses++
	}

	for leftIndex, leftID := range eligibleIDs {
		for _, rightID := range eligibleIDs[leftIndex+1:] {
			leftVotes := preferenceMatrix[leftID][rightID]
			rightVotes := preferenceMatrix[rightID][leftID]
			if leftVotes == rightVotes {
				byID[leftID].PairwiseTies++
				byID[rightID].PairwiseTies++
			}
		}
	}

	lockedEdges := buildRankedPairsGraph(pairResults, eligibleIDs)
	orderedIDs := topologicalOrder(lockedEdges, eligibleIDs, byID)
	for index, dateID := range orderedIDs {
		rank := index + 1
		byID[dateID].Rank = &rank
	}
	for index := range tallies {
		tallies[index].Record = formatRecord(tallies[index])
	}

	sortTallies(tallies)
	return Result{Dates: tallies, Winner: firstEligibleTally(tallies)}
}

type pairResult struct {
	WinnerID    string
	LoserID     string
	WinnerVotes int
	LoserVotes  int
	Margin      int
}

func buildPreferenceMatrix(votes []Vote, eligibleIDs []string) map[string]map[string]int {
	matrix := map[string]map[string]int{}
	for _, leftID := range eligibleIDs {
		matrix[leftID] = map[string]int{}
		for _, rightID := range eligibleIDs {
			if rightID != leftID {
				matrix[leftID][rightID] = 0
			}
		}
	}

	for _, vote := range votes {
		unavailableIDs := map[string]bool{}
		for _, dateID := range vote.UnavailableDateIDs {
			unavailableIDs[dateID] = true
		}

		rankingPositions := map[string]int{}
		for _, ranking := range vote.Rankings {
			if _, ok := matrix[ranking.DateID]; ok {
				rankingPositions[ranking.DateID] = ranking.Position
			}
		}
		maxPosition := len(rankingPositions) + 1

		for leftIndex, leftID := range eligibleIDs {
			if unavailableIDs[leftID] {
				continue
			}
			leftPosition := rankingPositions[leftID]
			if leftPosition == 0 {
				leftPosition = maxPosition
			}

			for _, rightID := range eligibleIDs[leftIndex+1:] {
				if unavailableIDs[rightID] {
					continue
				}
				rightPosition := rankingPositions[rightID]
				if rightPosition == 0 {
					rightPosition = maxPosition
				}

				if leftPosition < rightPosition {
					matrix[leftID][rightID]++
				} else if rightPosition < leftPosition {
					matrix[rightID][leftID]++
				}
			}
		}
	}
	return matrix
}

func buildPairResults(preferenceMatrix map[string]map[string]int, eligibleIDs []string) []pairResult {
	pairs := []pairResult{}
	for leftIndex, leftID := range eligibleIDs {
		for _, rightID := range eligibleIDs[leftIndex+1:] {
			leftVotes := preferenceMatrix[leftID][rightID]
			rightVotes := preferenceMatrix[rightID][leftID]
			if leftVotes == rightVotes {
				continue
			}

			winnerID := leftID
			loserID := rightID
			if rightVotes > leftVotes {
				winnerID = rightID
				loserID = leftID
			}
			winnerVotes := max(leftVotes, rightVotes)
			loserVotes := min(leftVotes, rightVotes)
			pairs = append(pairs, pairResult{
				WinnerID:    winnerID,
				LoserID:     loserID,
				WinnerVotes: winnerVotes,
				LoserVotes:  loserVotes,
				Margin:      winnerVotes - loserVotes,
			})
		}
	}
	sort.Slice(pairs, func(left, right int) bool {
		if pairs[left].Margin != pairs[right].Margin {
			return pairs[left].Margin > pairs[right].Margin
		}
		if pairs[left].WinnerVotes != pairs[right].WinnerVotes {
			return pairs[left].WinnerVotes > pairs[right].WinnerVotes
		}
		if pairs[left].WinnerID != pairs[right].WinnerID {
			return pairs[left].WinnerID < pairs[right].WinnerID
		}
		return pairs[left].LoserID < pairs[right].LoserID
	})
	return pairs
}

func buildRankedPairsGraph(pairResults []pairResult, eligibleIDs []string) map[string]map[string]bool {
	lockedEdges := map[string]map[string]bool{}
	for _, dateID := range eligibleIDs {
		lockedEdges[dateID] = map[string]bool{}
	}
	for _, pair := range pairResults {
		if createsCycle(lockedEdges, pair.WinnerID, pair.LoserID) {
			continue
		}
		lockedEdges[pair.WinnerID][pair.LoserID] = true
	}
	return lockedEdges
}

func createsCycle(lockedEdges map[string]map[string]bool, winnerID string, loserID string) bool {
	stack := []string{loserID}
	seen := map[string]bool{}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == winnerID {
			return true
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		for next := range lockedEdges[current] {
			stack = append(stack, next)
		}
	}
	return false
}

func topologicalOrder(lockedEdges map[string]map[string]bool, eligibleIDs []string, byID map[string]*DateResult) []string {
	incomingCounts := map[string]int{}
	for _, dateID := range eligibleIDs {
		incomingCounts[dateID] = 0
	}
	for _, losers := range lockedEdges {
		for loserID := range losers {
			incomingCounts[loserID]++
		}
	}

	ordered := []string{}
	remaining := map[string]bool{}
	for _, dateID := range eligibleIDs {
		remaining[dateID] = true
	}

	for len(remaining) > 0 {
		available := []string{}
		for dateID := range remaining {
			if incomingCounts[dateID] == 0 {
				available = append(available, dateID)
			}
		}
		if len(available) == 0 {
			for dateID := range remaining {
				available = append(available, dateID)
			}
		}

		sort.Slice(available, func(left, right int) bool {
			leftTally := byID[available[left]]
			rightTally := byID[available[right]]
			if leftTally.PairwiseWins != rightTally.PairwiseWins {
				return leftTally.PairwiseWins > rightTally.PairwiseWins
			}
			if leftTally.PairwiseLosses != rightTally.PairwiseLosses {
				return leftTally.PairwiseLosses < rightTally.PairwiseLosses
			}
			return leftTally.Value < rightTally.Value
		})

		chosenID := available[0]
		ordered = append(ordered, chosenID)
		delete(remaining, chosenID)
		for loserID := range lockedEdges[chosenID] {
			incomingCounts[loserID]--
		}
	}
	return ordered
}

func sortTallies(tallies []DateResult) {
	sort.Slice(tallies, func(left, right int) bool {
		if tallies[left].Disqualified != tallies[right].Disqualified {
			return !tallies[left].Disqualified
		}
		leftRank := 10000
		if tallies[left].Rank != nil {
			leftRank = *tallies[left].Rank
		}
		rightRank := 10000
		if tallies[right].Rank != nil {
			rightRank = *tallies[right].Rank
		}
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if tallies[left].PairwiseWins != tallies[right].PairwiseWins {
			return tallies[left].PairwiseWins > tallies[right].PairwiseWins
		}
		if tallies[left].PairwiseLosses != tallies[right].PairwiseLosses {
			return tallies[left].PairwiseLosses < tallies[right].PairwiseLosses
		}
		return tallies[left].Value < tallies[right].Value
	})
}

func firstEligibleTally(tallies []DateResult) *DateResult {
	for index := range tallies {
		if !tallies[index].Disqualified {
			return &tallies[index]
		}
	}
	return nil
}

func formatRecord(tally DateResult) string {
	return fmt.Sprintf("%d-%d-%d", tally.PairwiseWins, tally.PairwiseLosses, tally.PairwiseTies)
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
