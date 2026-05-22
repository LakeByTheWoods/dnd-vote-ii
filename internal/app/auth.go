package app

import (
	"context"
	"errors"
)

const ManagementTokenHeader = "X-DnD-Vote-Management-Token"

var (
	ErrManagementTokenRequired = errors.New("management token required")
	ErrManagementTokenInvalid  = errors.New("management token invalid")
)

func AuthorizeManagementToken(ctx context.Context, store *Store, pollID string, candidateToken string) error {
	if candidateToken == "" {
		return ErrManagementTokenRequired
	}

	matches, err := store.ManagementTokenMatches(ctx, pollID, candidateToken)
	if err != nil {
		return err
	}
	if !matches {
		return ErrManagementTokenInvalid
	}
	return nil
}
