package persistence

import (
	"context"
	"testing"

	domain "feed-service/internal/domain/feed"
	"github.com/stretchr/testify/require"
)

func TestRedisRepositoryListFilteredAppliesAntennaRulesBeforePagination(t *testing.T) {
	repo, _ := newTestRedisRepository(t)
	ctx := context.Background()
	items := []domain.Item{
		{ID: 1, AuthorID: 10, Title: "Go Redis guide", Summary: "backend", PublishedAt: 500, CoverURL: "https://cdn/1"},
		{ID: 2, AuthorID: 11, Title: "Go frontend guide", Summary: "ui", PublishedAt: 400, CoverURL: "https://cdn/2"},
		{ID: 3, AuthorID: 10, Title: "Rust guide", Summary: "backend", PublishedAt: 300},
		{ID: 4, AuthorID: 12, Title: "Go Redis guide", Summary: "blocked", PublishedAt: 200, CoverURL: "https://cdn/4"},
	}
	for _, item := range items {
		require.NoError(t, repo.UpsertArticle(ctx, item))
	}
	got, err := repo.ListFiltered(ctx, domain.Filter{
		Limit: 10, AuthorIDs: []int64{10, 11}, ExcludedAuthorIDs: []int64{11},
		Keywords: [][]string{{"go", "guide"}, {"rust"}}, ExcludeKeywords: [][]string{{"blocked"}}, WithFile: true,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, itemIDs(got))

	got, err = repo.ListFiltered(ctx, domain.Filter{Limit: 1, Offset: 1, Keywords: [][]string{{"guide"}}})
	require.NoError(t, err)
	require.Equal(t, []int64{2}, itemIDs(got))
}

func TestMatchesKeywordGroupsHonorsCaseSensitivity(t *testing.T) {
	require.True(t, matchesKeywordGroups("Go Redis", [][]string{{"go", "redis"}}, false))
	require.False(t, matchesKeywordGroups("Go Redis", [][]string{{"go"}}, true))
	require.True(t, matchesKeywordGroups("Go Redis", [][]string{{"go"}, {"Redis"}}, true))
}

func TestRedisRepositoryListFilteredReturnsNothingForEmptyRestrictedAuthors(t *testing.T) {
	repo, _ := newTestRedisRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.UpsertArticle(ctx, domain.Item{ID: 1, AuthorID: 10, Title: "public", PublishedAt: 500}))

	got, err := repo.ListFiltered(ctx, domain.Filter{Limit: 10, RestrictAuthors: true})

	require.NoError(t, err)
	require.Empty(t, got)
}
