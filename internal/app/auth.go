package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
)

const (
	ManagementTokenHeader = "X-DnD-Vote-Management-Token"
	AdminSecretHeader     = "X-DnD-Vote-Admin-Secret"
)

var (
	ErrManagementTokenRequired = errors.New("management token required")
	ErrManagementTokenInvalid  = errors.New("management token invalid")
	ErrAdminSecretRequired     = errors.New("admin secret required")
	ErrAdminSecretInvalid      = errors.New("admin secret invalid")
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

func AuthorizeAdminSecret(cfg Config, candidateSecret string) error {
	if candidateSecret == "" {
		return ErrAdminSecretRequired
	}
	if !constantTimeStringEqual(cfg.AdminSecret, candidateSecret) {
		return ErrAdminSecretInvalid
	}
	return nil
}

func constantTimeStringEqual(expected string, candidate string) bool {
	expectedDigest := sha256.Sum256([]byte(expected))
	candidateDigest := sha256.Sum256([]byte(candidate))
	matches := subtle.ConstantTimeCompare(expectedDigest[:], candidateDigest[:]) == 1
	return expected != "" && matches
}
