package query

import (
	"context"
	"strings"

	domain "content-service/internal/domain/article"
	"content-service/internal/infrastructure/cache"
)

type ArticleView struct {
	Article *domain.Article
}

type Service struct {
	repo  domain.Repository
	cache *cache.ArticleCache
}

func NewService(repo domain.Repository, c *cache.ArticleCache) *Service {
	return &Service{repo: repo, cache: c}
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
		return ArticleView{Article: a}, nil
	}
	a, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		return ArticleView{}, err
	}
	s.cache.Set(ctx, a)
	return ArticleView{Article: a}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (ArticleView, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ArticleView{}, err
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
