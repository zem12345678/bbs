package query

import (
	"context"
	"strings"

	domain "search-service/internal/domain/search"
)

type ArticleSearchResult struct {
	Items []domain.ArticleHit
	Total int64
}

type TopicSearchResult struct {
	Items []domain.TopicHit
	Total int64
}

type UserSearchResult struct {
	Items []domain.UserHit
	Total int64
}

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SearchArticles(ctx context.Context, keyword string, page, pageSize int32) (ArticleSearchResult, error) {
	if strings.TrimSpace(keyword) == "" {
		return ArticleSearchResult{}, domain.ErrKeywordRequired
	}
	items, total, err := s.repo.SearchArticles(ctx, keyword, page, pageSize)
	if err != nil {
		return ArticleSearchResult{}, err
	}
	return ArticleSearchResult{Items: items, Total: total}, nil
}

func (s *Service) SearchTopics(ctx context.Context, keyword string, page, pageSize int32) (TopicSearchResult, error) {
	if strings.TrimSpace(keyword) == "" {
		return TopicSearchResult{}, domain.ErrKeywordRequired
	}
	items, total, err := s.repo.SearchTopics(ctx, keyword, page, pageSize)
	if err != nil {
		return TopicSearchResult{}, err
	}
	return TopicSearchResult{Items: items, Total: total}, nil
}

func (s *Service) SearchUsers(ctx context.Context, keyword string, page, pageSize int32) (UserSearchResult, error) {
	if strings.TrimSpace(keyword) == "" {
		return UserSearchResult{}, domain.ErrKeywordRequired
	}
	items, total, err := s.repo.SearchUsers(ctx, keyword, page, pageSize)
	if err != nil {
		return UserSearchResult{}, err
	}
	return UserSearchResult{Items: items, Total: total}, nil
}
