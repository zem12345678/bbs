package account

import (
	"context"

	domain "content-service/internal/domain/account"
)

type ArticleCache interface {
	Del(ctx context.Context, slug string)
}

type Service struct {
	repo  domain.ErasureRepository
	cache ArticleCache
}

func NewService(repo domain.ErasureRepository, cache ArticleCache) *Service {
	return &Service{repo: repo, cache: cache}
}

func (s *Service) ArchiveAccountContent(ctx context.Context, userID, deletionJobID int64, policyVersion int32) (domain.ErasureResult, error) {
	if s == nil || s.repo == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.ErasureResult{}, domain.ErrInvalidErasure
	}
	result, err := s.repo.ArchiveAccountContent(ctx, userID, deletionJobID, policyVersion)
	if err != nil {
		return domain.ErasureResult{}, err
	}
	if s.cache != nil {
		for _, slug := range result.ArticleSlugs {
			s.cache.Del(ctx, slug)
		}
	}
	return result, nil
}
