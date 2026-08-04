package account

import (
	"context"

	domain "reaction-service/internal/domain/account"
)

type Service struct {
	repo  domain.ErasureRepository
	cache domain.ErasureCache
}

func NewService(repo domain.ErasureRepository, cache domain.ErasureCache) *Service {
	return &Service{repo: repo, cache: cache}
}

func (s *Service) EraseAccountReactions(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.ErasureResult, error) {
	if s == nil || s.repo == nil || s.cache == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.ErasureResult{}, domain.ErrInvalidErasure
	}
	if err := s.cache.TombstoneAccount(ctx, userID, deletionJobID, policyVersion); err != nil {
		return domain.ErasureResult{}, err
	}
	result, err := s.repo.EraseAccountReactions(ctx, userID, deletionJobID, policyVersion)
	if err != nil {
		return domain.ErasureResult{}, err
	}
	if err := s.cache.PurgeAccount(ctx, userID); err != nil {
		return domain.ErasureResult{}, err
	}
	return result, nil
}
