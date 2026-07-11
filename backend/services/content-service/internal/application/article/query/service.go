package query

import (
	"context"
	"strings"

	domain "content-service/internal/domain/article"
	"content-service/internal/infrastructure/cache"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type ArticleView struct {
	Article *domain.Article
}

type Service struct {
	repo      domain.Repository
	cache     *cache.ArticleCache
	publisher messaging.EventPublisher
	log       logger.Logger
}

func NewService(repo domain.Repository, c *cache.ArticleCache, publisher messaging.EventPublisher, log logger.Logger) *Service {
	return &Service{repo: repo, cache: c, publisher: publisher, log: log}
}

func toViews(articles []*domain.Article) []ArticleView {
	out := make([]ArticleView, 0, len(articles))
	for _, a := range articles {
		out = append(out, ArticleView{Article: a})
	}
	return out
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (ArticleView, error) {
	if a, ok := s.cache.Get(ctx, slug); ok {
		if a.Status.CanReadPublicly() {
			if count, err := s.repo.IncrementViewCount(ctx, a.ID); err == nil {
				a.ViewCount = count
				s.publishEvents(ctx, domain.NewArticleViewedEvent(a))
				s.cache.Set(ctx, a)
			}
		}
		return ArticleView{Article: a}, nil
	}
	a, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return ArticleView{}, err
	}
	if a.Status.CanReadPublicly() {
		if count, err := s.repo.IncrementViewCount(ctx, a.ID); err == nil {
			a.ViewCount = count
			s.publishEvents(ctx, domain.NewArticleViewedEvent(a))
		}
	}
	s.cache.Set(ctx, a)
	return ArticleView{Article: a}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (ArticleView, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ArticleView{}, err
	}
	if a.Status.CanReadPublicly() {
		if count, err := s.repo.IncrementViewCount(ctx, a.ID); err == nil {
			a.ViewCount = count
			s.publishEvents(ctx, domain.NewArticleViewedEvent(a))
		}
	}
	return ArticleView{Article: a}, nil
}

func (s *Service) List(ctx context.Context, status domain.Status, tag string, authorID int64, limit, offset int) ([]ArticleView, error) {
	articles, err := s.repo.List(ctx, status, tag, authorID, limit, offset)
	if err != nil {
		return nil, err
	}
	return toViews(articles), nil
}

func (s *Service) ListTags(ctx context.Context, status domain.Status, keyword string, limit int) ([]domain.TagStats, error) {
	return s.repo.ListTags(ctx, status, strings.TrimSpace(keyword), limit)
}

func (s *Service) FeedByTime(ctx context.Context, limit, offset int) ([]ArticleView, error) {
	articles, err := s.repo.FeedByTime(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return toViews(articles), nil
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	out := make([]messaging.DomainEvent, 0, len(events))
	for _, event := range events {
		out = append(out, event)
	}
	if err := s.publisher.PublishDomainEvents(ctx, out); err != nil && s.log != nil {
		s.log.Warn("publish article view event failed", logger.Error(err))
	}
}
