package account

import (
	"context"
	"errors"
)

var (
	ErrInvalidErasure = errors.New("INVALID_ACCOUNT_ERASURE")
	ErrUserErased     = errors.New("CONTENT_USER_ERASED")
)

type ErasureResult struct {
	ArchivedArticles   int64
	ArchivedTopics     int64
	DeletedPollBallots int64
	ArticleSlugs       []string
}

type ErasureRepository interface {
	ArchiveAccountContent(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (ErasureResult, error)
}
