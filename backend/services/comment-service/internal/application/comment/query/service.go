package query

import (
	"context"

	domain "comment-service/internal/domain/comment"
)

type CommentListResult struct {
	Items []*domain.Comment
	Total int64
}

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, id int64) (*domain.Comment, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidID
	}
	return s.repo.FindByID(ctx, id)
}

func (s *Service) ListByEntity(ctx context.Context, q domain.ListQuery) (CommentListResult, error) {
	if _, err := domain.ParseEntityType(q.EntityType); err != nil {
		return CommentListResult{}, err
	}
	if q.EntityID <= 0 {
		return CommentListResult{}, domain.ErrInvalidEntityID
	}
	items, total, err := s.repo.ListByEntity(ctx, q)
	if err != nil {
		return CommentListResult{}, err
	}
	return CommentListResult{Items: items, Total: total}, nil
}

func (s *Service) ListReplies(ctx context.Context, q domain.ReplyListQuery) (CommentListResult, error) {
	if q.RootID <= 0 {
		return CommentListResult{}, domain.ErrInvalidParent
	}
	items, total, err := s.repo.ListReplies(ctx, q)
	if err != nil {
		return CommentListResult{}, err
	}
	return CommentListResult{Items: items, Total: total}, nil
}

func (s *Service) ListForModeration(ctx context.Context, q domain.ModerationListQuery) (CommentListResult, error) {
	if q.EntityType != "" {
		if _, err := domain.ParseEntityType(q.EntityType); err != nil {
			return CommentListResult{}, err
		}
	}
	if q.EntityID < 0 {
		return CommentListResult{}, domain.ErrInvalidEntityID
	}
	if q.AuthorID < 0 {
		return CommentListResult{}, domain.ErrInvalidAuthorID
	}
	if q.Status >= 0 && !domain.Status(q.Status).IsValid() {
		return CommentListResult{}, domain.ErrInvalidStatus
	}
	items, total, err := s.repo.ListForModeration(ctx, q)
	if err != nil {
		return CommentListResult{}, err
	}
	return CommentListResult{Items: items, Total: total}, nil
}
