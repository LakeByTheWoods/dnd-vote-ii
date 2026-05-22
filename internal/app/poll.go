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

const (
	maxPollTitleLength = 120
	maxVoterNameLength = 80
)

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

type VoteInput struct {
	VoterName          string
	Rankings           []string
	UnavailableDateIDs []string
}

type PollSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	DateCount int    `json:"dateCount"`
	VoteCount int    `json:"voteCount"`
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
	votes, err := s.fetchVotes(ctx, s.db, pollID)
	if err != nil {
		return Poll{}, err
	}
	poll.Votes = votes
	return poll, nil
}

func (s *Store) ReplaceVote(ctx context.Context, pollID string, voteInput VoteInput) (Poll, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Poll{}, err
	}
	defer rollbackUnlessCommitted(tx)

	poll, err := s.fetchPollForUpdate(ctx, tx, pollID)
	if err != nil {
		return Poll{}, err
	}
	if err := validateVoteAgainstPoll(voteInput, poll); err != nil {
		return Poll{}, err
	}

	var existingVoteID string
	err = tx.QueryRowContext(
		ctx,
		"SELECT id FROM votes WHERE poll_id = ? AND voter_name = ? COLLATE NOCASE",
		pollID,
		voteInput.VoterName,
	).Scan(&existingVoteID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Poll{}, err
	}
	if existingVoteID != "" {
		if _, err := tx.ExecContext(ctx, "DELETE FROM vote_rankings WHERE vote_id = ?", existingVoteID); err != nil {
			return Poll{}, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM vote_unavailability WHERE vote_id = ?", existingVoteID); err != nil {
			return Poll{}, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM votes WHERE id = ?", existingVoteID); err != nil {
			return Poll{}, err
		}
	}

	voteID := uuid.NewString()
	createdAt := utcNowISO()
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO votes (id, poll_id, voter_name, created_at) VALUES (?, ?, ?, ?)",
		voteID,
		pollID,
		voteInput.VoterName,
		createdAt,
	); err != nil {
		return Poll{}, err
	}
	for position, dateID := range voteInput.Rankings {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO vote_rankings (vote_id, date_id, position) VALUES (?, ?, ?)",
			voteID,
			dateID,
			position+1,
		); err != nil {
			return Poll{}, err
		}
	}
	for _, dateID := range voteInput.UnavailableDateIDs {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO vote_unavailability (vote_id, date_id) VALUES (?, ?)",
			voteID,
			dateID,
		); err != nil {
			return Poll{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Poll{}, err
	}

	return s.FetchPoll(ctx, pollID)
}

func (s *Store) fetchPollForUpdate(ctx context.Context, tx *sql.Tx, pollID string) (Poll, error) {
	var poll Poll
	err := tx.QueryRowContext(
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

	dates, err := fetchPollDates(ctx, tx, pollID)
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
	return fetchPollDates(ctx, s.db, pollID)
}

type queryContext interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func fetchPollDates(ctx context.Context, queryer queryContext, pollID string) ([]PollDate, error) {
	rows, err := queryer.QueryContext(ctx, `
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

func (s *Store) fetchVotes(ctx context.Context, queryer queryContext, pollID string) ([]Vote, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, voter_name, created_at
		FROM votes
		WHERE poll_id = ?
		ORDER BY created_at ASC, voter_name COLLATE NOCASE ASC
	`, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	votes := []Vote{}
	for rows.Next() {
		var vote Vote
		if err := rows.Scan(&vote.ID, &vote.VoterName, &vote.CreatedAt); err != nil {
			return nil, err
		}
		vote.Rankings = []VoteRanking{}
		vote.UnavailableDateIDs = []string{}
		votes = append(votes, vote)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for index := range votes {
		rankings, err := s.fetchVoteRankings(ctx, queryer, votes[index].ID)
		if err != nil {
			return nil, err
		}
		unavailableDateIDs, err := s.fetchVoteUnavailableDateIDs(ctx, queryer, votes[index].ID)
		if err != nil {
			return nil, err
		}
		votes[index].Rankings = rankings
		votes[index].UnavailableDateIDs = unavailableDateIDs
	}
	return votes, nil
}

func (s *Store) fetchVoteRankings(ctx context.Context, queryer queryContext, voteID string) ([]VoteRanking, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT date_id, position
		FROM vote_rankings
		WHERE vote_id = ?
		ORDER BY position ASC
	`, voteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rankings := []VoteRanking{}
	for rows.Next() {
		var ranking VoteRanking
		if err := rows.Scan(&ranking.DateID, &ranking.Position); err != nil {
			return nil, err
		}
		rankings = append(rankings, ranking)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rankings, nil
}

func (s *Store) fetchVoteUnavailableDateIDs(ctx context.Context, queryer queryContext, voteID string) ([]string, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT date_id
		FROM vote_unavailability
		WHERE vote_id = ?
		ORDER BY date_id ASC
	`, voteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dateIDs := []string{}
	for rows.Next() {
		var dateID string
		if err := rows.Scan(&dateID); err != nil {
			return nil, err
		}
		dateIDs = append(dateIDs, dateID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dateIDs, nil
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

func NormalizeVoteInput(voterNameValue any, rankingsValue any, unavailableValue any) (VoteInput, error) {
	voterName, ok := voterNameValue.(string)
	if !ok {
		return VoteInput{}, validationError("Voter name must be a string.")
	}

	rankingItems, ok := rankingsValue.([]any)
	if !ok {
		return VoteInput{}, validationError("Rankings and unavailableDateIds must be arrays.")
	}
	unavailableItems, ok := unavailableValue.([]any)
	if !ok {
		return VoteInput{}, validationError("Rankings and unavailableDateIds must be arrays.")
	}

	voteInput := VoteInput{
		VoterName:          strings.TrimSpace(voterName),
		Rankings:           stringifyVoteDateIDs(rankingItems),
		UnavailableDateIDs: stringifyVoteDateIDs(unavailableItems),
	}
	if voteInput.VoterName == "" {
		return VoteInput{}, validationError("Voter name is required.")
	}
	if utf8.RuneCountInString(voteInput.VoterName) > maxVoterNameLength {
		return VoteInput{}, validationError(fmt.Sprintf("Voter name must be %d characters or fewer.", maxVoterNameLength))
	}
	return voteInput, nil
}

func stringifyVoteDateIDs(items []any) []string {
	dateIDs := make([]string, 0, len(items))
	for _, item := range items {
		dateIDs = append(dateIDs, fmt.Sprint(item))
	}
	return dateIDs
}

func validateVoteAgainstPoll(voteInput VoteInput, poll Poll) error {
	if len(voteInput.Rankings) == 0 {
		return validationError("At least one ranked date is required.")
	}
	if hasDuplicateStrings(voteInput.Rankings) {
		return validationError("Ranked dates must be unique.")
	}
	if hasDuplicateStrings(voteInput.UnavailableDateIDs) {
		return validationError("Unavailable dates must be unique.")
	}

	validDateIDs := map[string]bool{}
	for _, date := range poll.Dates {
		validDateIDs[date.ID] = true
	}
	for _, dateID := range voteInput.Rankings {
		if !validDateIDs[dateID] {
			return validationError("Rankings include an unknown date.")
		}
	}
	for _, dateID := range voteInput.UnavailableDateIDs {
		if !validDateIDs[dateID] {
			return validationError("Unavailable dates include an unknown date.")
		}
	}

	rankedDateIDs := map[string]bool{}
	for _, dateID := range voteInput.Rankings {
		rankedDateIDs[dateID] = true
	}
	for _, dateID := range voteInput.UnavailableDateIDs {
		if rankedDateIDs[dateID] {
			return validationError("A date cannot be both ranked and unavailable.")
		}
	}
	return nil
}

func hasDuplicateStrings(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
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
