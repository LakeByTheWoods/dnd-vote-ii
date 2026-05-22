package app

import "dnd-vote-ii/internal/tally"

type TalliedDate struct {
	ID             string   `json:"id"`
	Value          string   `json:"value"`
	PairwiseWins   int      `json:"pairwiseWins"`
	PairwiseLosses int      `json:"pairwiseLosses"`
	PairwiseTies   int      `json:"pairwiseTies"`
	Record         string   `json:"record"`
	Rank           *int     `json:"rank"`
	UnavailableBy  []string `json:"unavailableBy"`
	Disqualified   bool     `json:"disqualified"`
}

func TallyNoVoteResults(poll Poll) ([]TalliedDate, *TalliedDate) {
	return TallyResults(poll)
}

func TallyResults(poll Poll) ([]TalliedDate, *TalliedDate) {
	result := tally.Calculate(toTallyPoll(poll))
	talliedDates := toTalliedDates(result.Dates)
	if result.Winner == nil {
		return talliedDates, nil
	}
	winner := toTalliedDate(*result.Winner)
	return talliedDates, &winner
}

func toTallyPoll(poll Poll) tally.Poll {
	dates := make([]tally.Date, 0, len(poll.Dates))
	for _, date := range poll.Dates {
		dates = append(dates, tally.Date{ID: date.ID, Value: date.Value})
	}

	votes := make([]tally.Vote, 0, len(poll.Votes))
	for _, vote := range poll.Votes {
		rankings := make([]tally.Ranking, 0, len(vote.Rankings))
		for _, ranking := range vote.Rankings {
			rankings = append(rankings, tally.Ranking{DateID: ranking.DateID, Position: ranking.Position})
		}
		unavailableDateIDs := append([]string(nil), vote.UnavailableDateIDs...)
		votes = append(votes, tally.Vote{
			VoterName:          vote.VoterName,
			Rankings:           rankings,
			UnavailableDateIDs: unavailableDateIDs,
		})
	}

	return tally.Poll{Dates: dates, Votes: votes}
}

func toTalliedDates(results []tally.DateResult) []TalliedDate {
	talliedDates := make([]TalliedDate, 0, len(results))
	for _, result := range results {
		talliedDates = append(talliedDates, toTalliedDate(result))
	}
	return talliedDates
}

func toTalliedDate(result tally.DateResult) TalliedDate {
	unavailableBy := append([]string(nil), result.UnavailableBy...)
	return TalliedDate{
		ID:             result.ID,
		Value:          result.Value,
		PairwiseWins:   result.PairwiseWins,
		PairwiseLosses: result.PairwiseLosses,
		PairwiseTies:   result.PairwiseTies,
		Record:         result.Record,
		Rank:           result.Rank,
		UnavailableBy:  unavailableBy,
		Disqualified:   result.Disqualified,
	}
}
