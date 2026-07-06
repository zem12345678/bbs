package command

import (
	"context"

	domain "search-service/internal/domain/search"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) EnsureArticleIndex(ctx context.Context) error {
	return s.repo.EnsureArticleIndex(ctx)
}

func (s *Service) EnsureTopicIndex(ctx context.Context) error {
	return s.repo.EnsureTopicIndex(ctx)
}

func (s *Service) IndexArticle(ctx context.Context, doc domain.ArticleDocument) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	return s.repo.IndexArticle(ctx, doc)
}

func (s *Service) IndexTopic(ctx context.Context, doc domain.TopicDocument) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	return s.repo.IndexTopic(ctx, doc)
}

func (s *Service) DeleteArticle(ctx context.Context, id int64) error {
	if id <= 0 {
		return domain.ErrInvalidArticleID
	}
	return s.repo.DeleteArticle(ctx, id)
}

func (s *Service) DeleteTopic(ctx context.Context, id int64) error {
	if id <= 0 {
		return domain.ErrInvalidArticleID
	}
	return s.repo.DeleteTopic(ctx, id)
}
