package command

import (
	"context"
	"fmt"

	domain "feed-service/internal/domain/feed"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) PurgeAccountFeed(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (int64, error) {
	if userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return 0, fmt.Errorf("user ID, deletion job ID, and policy version must be greater than zero")
	}
	return s.repo.PurgeByAuthor(ctx, userID)
}
