package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxPollTitleLength = 120

var ErrPollNotFound = errors.New("poll not found")

type Poll struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	CreatedAt string     `json:"createdAt"`
	Dates     []PollDate `json:"dates"`
	Votes     []Vote     `json:"votes"`
}

type PollDate struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type Vote struct {
	ID                 string        `json:"id"`
	VoterName          string        `json:"voterName"`
	CreatedAt          string        `json:"createdAt"`
	Rankings           []VoteRanking `json:"rankings"`
	UnavailableDateIDs []string      `json:"unavailableDateIds"`
}

type VoteRanking struct {
	DateID   string `json:"dateId"`
	Position int    `json:"position"`
}

type PollSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	DateCount int    `json:"dateCount"`
	VoteCount int    `json:"voteCount"`
}

type TalliedDate struct {
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

func (s *Store) CreatePoll(ctx context.Context, title string, dateValues []string) (Poll, string, error) {
	pollID := uuid.NewString()
	createdAt := utcNowISO()
	managementToken, err := generateManagementToken()
	if err != nil {
		return Poll{}, "", err
	}

	dates := make([]PollDate, 0, len(dateValues))
	for _, value := range dateValues {
		dates = append(dates, PollDate{ID: uuid.NewString(), Value: value})
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Poll{}, "", err
	}
	defer rollbackUnlessCommitted(tx)

	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO polls (id, title, created_at, management_token) VALUES (?, ?, ?, ?)",
		pollID,
		title,
		createdAt,
		managementToken,
	); err != nil {
		return Poll{}, "", err
	}

	for _, date := range dates {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO poll_dates (id, poll_id, date_value) VALUES (?, ?, ?)",
			date.ID,
			pollID,
			date.Value,
		); err != nil {
			return Poll{}, "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return Poll{}, "", err
	}

	return Poll{
		ID:        pollID,
		Title:     title,
		CreatedAt: createdAt,
		Dates:     dates,
		Votes:     []Vote{},
	}, managementToken, nil
}

func (s *Store) FetchPoll(ctx context.Context, pollID string) (Poll, error) {
	var poll Poll
	err := s.db.QueryRowContext(
		ctx,
		"SELECT id, title, created_at FROM polls WHERE id = ?",
		pollID,
	).Scan(&poll.ID, &poll.Title, &poll.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Poll{}, ErrPollNotFound
	}
	if err != nil {
		return Poll{}, err
	}

	dates, err := s.fetchPollDates(ctx, pollID)
	if err != nil {
		return Poll{}, err
	}
	poll.Dates = dates
	poll.Votes = []Vote{}
	return poll, nil
}

func (s *Store) ListPollSummaries(ctx context.Context) ([]PollSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
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
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []PollSummary{}
	for rows.Next() {
		var summary PollSummary
		if err := rows.Scan(
			&summary.ID,
			&summary.Title,
			&summary.CreatedAt,
			&summary.DateCount,
			&summary.VoteCount,
		); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *Store) fetchPollDates(ctx context.Context, pollID string) ([]PollDate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, date_value
		FROM poll_dates
		WHERE poll_id = ?
		ORDER BY date_value ASC
	`, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dates := []PollDate{}
	for rows.Next() {
		var date PollDate
		if err := rows.Scan(&date.ID, &date.Value); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dates, nil
}

func NormalizePollInput(titleValue any, datesValue any) (string, []string, error) {
	title, ok := titleValue.(string)
	if !ok {
		title = ""
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return "", nil, validationError("Title is required.")
	}
	if utf8.RuneCountInString(title) > maxPollTitleLength {
		return "", nil, validationError(fmt.Sprintf("Title must be %d characters or fewer.", maxPollTitleLength))
	}

	dateItems, ok := datesValue.([]any)
	if !ok {
		return "", nil, validationError("Dates must be an array.")
	}
	dates, err := normalizePollDates(dateItems, time.Now())
	if err != nil {
		return "", nil, err
	}
	return title, dates, nil
}

func normalizePollDates(dateValues []any, now time.Time) ([]string, error) {
	today := truncateToDate(now)
	uniqueDates := map[string]bool{}

	for _, value := range dateValues {
		dateValue, ok := value.(string)
		if !ok {
			return nil, validationError("Dates must use YYYY-MM-DD format.")
		}
		dateValue = strings.TrimSpace(dateValue)
		if dateValue == "" {
			return nil, validationError("Dates must use YYYY-MM-DD format.")
		}

		parsedDate, err := time.Parse("2006-01-02", dateValue)
		if err != nil || parsedDate.Format("2006-01-02") != dateValue {
			return nil, validationError("Dates must use YYYY-MM-DD format.")
		}
		if parsedDate.Before(today) {
			return nil, validationError("Candidate dates must be today or later.")
		}
		uniqueDates[dateValue] = true
	}

	dates := make([]string, 0, len(uniqueDates))
	for date := range uniqueDates {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	if len(dates) < 2 {
		return nil, validationError("At least two unique dates are required.")
	}
	return dates, nil
}

func TallyNoVoteResults(poll Poll) ([]TalliedDate, *TalliedDate) {
	tallies := make([]TalliedDate, 0, len(poll.Dates))
	for index, date := range poll.Dates {
		pairwiseTies := max(len(poll.Dates)-1, 0)
		tally := TalliedDate{
			ID:             date.ID,
			Value:          date.Value,
			PairwiseTies:   pairwiseTies,
			Record:         fmt.Sprintf("0-0-%d", pairwiseTies),
			Rank:           index + 1,
			UnavailableBy:  []string{},
			Disqualified:   false,
			PairwiseWins:   0,
			PairwiseLosses: 0,
		}
		tallies = append(tallies, tally)
	}
	if len(tallies) == 0 {
		return tallies, nil
	}
	return tallies, &tallies[0]
}

type validationError string

func (e validationError) Error() string {
	return string(e)
}

func isValidationError(err error) bool {
	var target validationError
	return errors.As(err, &target)
}

func generateManagementToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func utcNowISO() string {
	return time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
}

func truncateToDate(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	_ = tx.Rollback()
}
