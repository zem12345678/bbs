package query

import (
	"context"

	domain "content-service/internal/domain/channel"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, id, viewerID int64, includeArchived bool) (*domain.Channel, error) {
	return s.repo.FindChannelByID(ctx, id, viewerID, includeArchived)
}

func (s *Service) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Channel, int64, error) {
	return s.repo.ListChannels(ctx, filter)
}

func (s *Service) ListCategoryAggregates(ctx context.Context, includeArchived bool) ([]domain.CategoryAggregate, error) {
	return s.repo.ListChannelCategoryAggregates(ctx, includeArchived)
}
