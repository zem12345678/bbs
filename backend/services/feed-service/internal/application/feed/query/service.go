package query

import (
	"context"

	domain "feed-service/internal/domain/feed"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListLatest(ctx context.Context, limit, offset int, authorIDs []int64) ([]domain.Item, error) {
	return s.repo.ListLatest(ctx, normalizeLimit(limit), normalizeOffset(offset), authorIDs)
}

func (s *Service) ListHot(ctx context.Context, limit, offset int) ([]domain.Item, error) {
	return s.repo.ListHot(ctx, normalizeLimit(limit), normalizeOffset(offset))
}

func (s *Service) ListActive(ctx context.Context, limit, offset int) ([]domain.Item, error) {
	return s.repo.ListActive(ctx, normalizeLimit(limit), normalizeOffset(offset))
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
