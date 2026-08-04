package account

import (
	"context"

	domain "chat-service/internal/domain/chat"
)

type Service struct {
	repo domain.AccountErasureRepository
}

func NewService(repo domain.AccountErasureRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	if s == nil || s.repo == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.AccountErasureResult{}, domain.ErrInvalidErasure
	}
	return s.repo.EraseUserData(ctx, userID, deletionJobID, policyVersion)
}
