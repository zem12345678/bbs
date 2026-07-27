package admin

import (
	"context"

	domain "admin/internal/domain/admin"
)

func (s *Service) StartSearchRebuild(ctx context.Context, actor domain.Actor) (domain.SearchRebuildStatus, error) {
	if err := actor.Validate(); err != nil {
		return domain.SearchRebuildStatus{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionRebuildSearch); err != nil {
		return domain.SearchRebuildStatus{}, err
	}
	if s.searchRebuild == nil {
		return domain.SearchRebuildStatus{}, domain.ErrSearchRebuildUnavailable
	}
	return s.searchRebuild.StartSearchRebuild(ctx, actor.ID)
}

func (s *Service) GetSearchRebuildStatus(ctx context.Context, actor domain.Actor) (domain.SearchRebuildStatus, error) {
	if err := actor.Validate(); err != nil {
		return domain.SearchRebuildStatus{}, err
	}
	if err := s.auth.Authorize(ctx, actor, domain.ActionViewSearchRebuild); err != nil {
		return domain.SearchRebuildStatus{}, err
	}
	if s.searchRebuild == nil {
		return domain.SearchRebuildStatus{}, domain.ErrSearchRebuildUnavailable
	}
	return s.searchRebuild.GetSearchRebuildStatus(ctx)
}
