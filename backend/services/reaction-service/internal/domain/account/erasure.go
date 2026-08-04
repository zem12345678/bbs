package account

import (
	"context"
	"errors"
)

var (
	ErrInvalidErasure = errors.New("INVALID_ACCOUNT_ERASURE")
	ErrUserErased     = errors.New("REACTION_USER_ERASED")
)

type ErasureResult struct {
	DeletedLikes             int64
	DeletedFavorites         int64
	DeletedCollections       int64
	AnonymizedReports        int64
	AnonymizedHandledReports int64
}

type ErasureRepository interface {
	EraseAccountReactions(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (ErasureResult, error)
}

type ErasureCache interface {
	TombstoneAccount(ctx context.Context, userID, deletionJobID int64, policyVersion int32) error
	PurgeAccount(ctx context.Context, userID int64) error
}
