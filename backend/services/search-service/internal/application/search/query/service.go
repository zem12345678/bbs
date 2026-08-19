package query

import (
	"context"
	"strings"
	"unicode/utf8"

	domain "search-service/internal/domain/search"
)

const (
	defaultTagSearchLimit = 10
	maxTagSearchLimit     = 100
	maxTagQueryItems      = 8
	maxTagRunes           = 128
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

func (s *Service) SearchByTag(ctx context.Context, criteria domain.SearchByTagCriteria) ([]domain.NoteLikeHit, error) {
	criteria.Tag = normalizeTag(criteria.Tag)
	criteria.Scope = strings.ToLower(strings.TrimSpace(criteria.Scope))
	if criteria.Limit == 0 {
		criteria.Limit = defaultTagSearchLimit
	}
	if criteria.Limit < 1 || criteria.Limit > maxTagSearchLimit || criteria.SinceID < 0 || criteria.UntilID < 0 {
		return nil, domain.ErrTagQueryInvalid
	}
	if criteria.Scope != "" || criteria.Reply != nil || criteria.Renote != nil || criteria.Poll != nil || criteria.WithFiles {
		return nil, domain.ErrTagFilterUnsupported
	}

	if criteria.Tag != "" {
		if !validTag(criteria.Tag) {
			return nil, domain.ErrTagQueryInvalid
		}
		criteria.Query = nil
	} else {
		if len(criteria.Query) == 0 {
			return nil, domain.ErrTagQueryRequired
		}
		if len(criteria.Query) > maxTagQueryItems {
			return nil, domain.ErrTagQueryInvalid
		}
		for i := range criteria.Query {
			if len(criteria.Query[i].Tags) == 0 || len(criteria.Query[i].Tags) > maxTagQueryItems {
				return nil, domain.ErrTagQueryInvalid
			}
			for j := range criteria.Query[i].Tags {
				criteria.Query[i].Tags[j] = normalizeTag(criteria.Query[i].Tags[j])
				if !validTag(criteria.Query[i].Tags[j]) {
					return nil, domain.ErrTagQueryInvalid
				}
			}
		}
	}

	return s.repo.SearchByTag(ctx, criteria)
}

func normalizeTag(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validTag(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maxTagRunes
}
