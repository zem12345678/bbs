package persistence

import (
	"context"
	"strconv"
	"testing"

	domain "feed-service/internal/domain/feed"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisRepositoryListLatestFiltersBeforePagination(t *testing.T) {
	repo, _ := newTestRedisRepository(t)
	ctx := context.Background()
	for _, item := range []domain.Item{
		{ID: 10, AuthorID: 1, Title: "ten", PublishedAt: 400},
		{ID: 11, AuthorID: 2, Title: "eleven", PublishedAt: 300},
		{ID: 12, AuthorID: 3, Title: "twelve", PublishedAt: 250},
		{ID: 13, AuthorID: 1, Title: "thirteen", PublishedAt: 200},
	} {
		require.NoError(t, repo.UpsertArticle(ctx, item))
	}

	items, err := repo.ListLatest(ctx, 2, 1, []int64{1, 3, 1})
	require.NoError(t, err)
	require.Equal(t, []int64{12, 13}, itemIDs(items))

	all, err := repo.ListLatest(ctx, 2, 1, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{11, 12}, itemIDs(all))
}

func TestRedisRepositoryListLatestUsesStableOrderForEqualScores(t *testing.T) {
	repo, _ := newTestRedisRepository(t)
	ctx := context.Background()
	for _, item := range []domain.Item{
		{ID: 10, AuthorID: 1, Title: "ten", PublishedAt: 100},
		{ID: 2, AuthorID: 2, Title: "two", PublishedAt: 100},
	} {
		require.NoError(t, repo.UpsertArticle(ctx, item))
	}

	first, err := repo.ListLatest(ctx, 1, 0, []int64{1, 2})
	require.NoError(t, err)
	second, err := repo.ListLatest(ctx, 1, 1, []int64{1, 2})
	require.NoError(t, err)
	require.Equal(t, []int64{2}, itemIDs(first))
	require.Equal(t, []int64{10}, itemIDs(second))
}

func TestRedisRepositoryMaintainsAuthorLatestIndex(t *testing.T) {
	repo, _ := newTestRedisRepository(t)
	ctx := context.Background()
	item := domain.Item{ID: 20, AuthorID: 1, Title: "item", PublishedAt: 100}
	require.NoError(t, repo.UpsertTopic(ctx, item))

	item.AuthorID = 2
	require.NoError(t, repo.UpsertTopic(ctx, item))
	items, err := repo.ListLatest(ctx, 10, 0, []int64{1})
	require.NoError(t, err)
	require.Empty(t, items)
	items, err = repo.ListLatest(ctx, 10, 0, []int64{2})
	require.NoError(t, err)
	require.Equal(t, []int64{20}, itemIDs(items))

	require.NoError(t, repo.RemoveTopic(ctx, item.ID))
	items, err = repo.ListLatest(ctx, 10, 0, []int64{2})
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestRedisRepositoryBackfillsPreexistingLatestItems(t *testing.T) {
	repo, client := newTestRedisRepository(t)
	ctx := context.Background()
	item := domain.Item{ID: 30, AuthorID: 7, Title: "legacy", PublishedAt: 100}
	require.NoError(t, repo.UpsertArticle(ctx, item))
	require.NoError(t, client.Del(ctx, authorLatestKey(item.AuthorID), authorLatestBackfillReadyKey, authorLatestBackfillCursorKey).Err())

	items, err := repo.ListLatest(ctx, 10, 0, []int64{7})
	require.NoError(t, err)
	require.Equal(t, []int64{30}, itemIDs(items))
	require.NoError(t, client.ZScore(ctx, authorLatestKey(item.AuthorID), strconv.FormatInt(item.ID, 10)).Err())
}

func newTestRedisRepository(t *testing.T) (*RedisRepository, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisRepository(client), client
}

func itemIDs(items []domain.Item) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
