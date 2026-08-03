package persistence

import (
	"context"
	"strconv"
	"testing"

	domain "feed-service/internal/domain/feed"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisRepositoryPurgeByAuthorRemovesItemsAndPreventsRevival(t *testing.T) {
	repo, client := newTestRedisRepository(t)
	ctx := context.Background()
	targetAuthorID := int64(42)
	otherAuthorID := int64(43)
	targetArticle := domain.Item{ID: 101, AuthorID: targetAuthorID, Title: "article", PublishedAt: 300, UpdatedAt: 300}
	targetTopic := domain.Item{ID: 102, AuthorID: targetAuthorID, Title: "topic", PublishedAt: 200, UpdatedAt: 200}
	otherArticle := domain.Item{ID: 201, AuthorID: otherAuthorID, Title: "other", PublishedAt: 100, UpdatedAt: 100}

	require.NoError(t, repo.UpsertArticle(ctx, targetArticle))
	require.NoError(t, repo.UpsertTopic(ctx, targetTopic))
	require.NoError(t, repo.UpsertArticle(ctx, otherArticle))

	purged, err := repo.PurgeByAuthor(ctx, targetAuthorID)
	require.NoError(t, err)
	require.Equal(t, int64(2), purged)
	require.True(t, client.SIsMember(ctx, purgedAuthorsKey, strconv.FormatInt(targetAuthorID, 10)).Val())

	for _, item := range []domain.Item{targetArticle, targetTopic} {
		require.Equal(t, int64(0), client.Exists(ctx, articleKey(item.ID)).Val())
		requireIndexMemberMissing(t, ctx, client, latestKey, item.ID)
		requireIndexMemberMissing(t, ctx, client, hotKey, item.ID)
		requireIndexMemberMissing(t, ctx, client, activeKey, item.ID)
		requireIndexMemberMissing(t, ctx, client, authorLatestKey(targetAuthorID), item.ID)
	}
	require.Equal(t, int64(0), client.Exists(ctx, authorLatestKey(targetAuthorID)).Val())
	require.Equal(t, int64(1), client.Exists(ctx, articleKey(otherArticle.ID)).Val())
	requireIndexMemberPresent(t, ctx, client, latestKey, otherArticle.ID)
	requireIndexMemberPresent(t, ctx, client, hotKey, otherArticle.ID)
	requireIndexMemberPresent(t, ctx, client, activeKey, otherArticle.ID)
	requireIndexMemberPresent(t, ctx, client, authorLatestKey(otherAuthorID), otherArticle.ID)

	purged, err = repo.PurgeByAuthor(ctx, targetAuthorID)
	require.NoError(t, err)
	require.Zero(t, purged)

	require.NoError(t, repo.UpsertArticle(ctx, targetArticle))
	delayedTopic := domain.Item{ID: 103, AuthorID: targetAuthorID, Title: "delayed", PublishedAt: 400, UpdatedAt: 400}
	require.NoError(t, repo.UpsertTopic(ctx, delayedTopic))
	for _, item := range []domain.Item{targetArticle, delayedTopic} {
		require.Equal(t, int64(0), client.Exists(ctx, articleKey(item.ID)).Val())
		requireIndexMemberMissing(t, ctx, client, latestKey, item.ID)
		requireIndexMemberMissing(t, ctx, client, hotKey, item.ID)
		requireIndexMemberMissing(t, ctx, client, activeKey, item.ID)
		requireIndexMemberMissing(t, ctx, client, authorLatestKey(targetAuthorID), item.ID)
	}

	latest, err := repo.ListLatest(ctx, 10, 0, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{otherArticle.ID}, itemIDs(latest))
	hot, err := repo.ListHot(ctx, 10, 0)
	require.NoError(t, err)
	require.Equal(t, []int64{otherArticle.ID}, itemIDs(hot))
	active, err := repo.ListActive(ctx, 10, 0)
	require.NoError(t, err)
	require.Equal(t, []int64{otherArticle.ID}, itemIDs(active))
}

func TestRedisRepositoryPurgeByAuthorScansLegacyItemsWithoutAuthorIndex(t *testing.T) {
	repo, client := newTestRedisRepository(t)
	ctx := context.Background()
	userID := int64(52)
	itemID := int64(301)

	require.NoError(t, client.HSet(ctx, articleKey(itemID), map[string]any{
		"id":           itemID,
		"author_id":    userID,
		"title":        "legacy",
		"published_at": 500,
		"updated_at":   500,
	}).Err())
	for _, key := range []string{latestKey, hotKey, activeKey} {
		require.NoError(t, client.ZAdd(ctx, key, redis.Z{Score: 500, Member: itemID}).Err())
	}
	require.Equal(t, int64(0), client.Exists(ctx, authorLatestKey(userID)).Val())

	purged, err := repo.PurgeByAuthor(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, int64(1), purged)
	require.Equal(t, int64(0), client.Exists(ctx, articleKey(itemID)).Val())
	for _, key := range []string{latestKey, hotKey, activeKey, authorLatestKey(userID)} {
		requireIndexMemberMissing(t, ctx, client, key, itemID)
	}
}

func TestRedisRepositoryListsHideTombstonedLegacyItems(t *testing.T) {
	repo, client := newTestRedisRepository(t)
	ctx := context.Background()
	userID := int64(62)
	itemID := int64(401)
	otherItem := domain.Item{ID: 402, AuthorID: 63, Title: "visible", PublishedAt: 100, UpdatedAt: 100}
	require.NoError(t, repo.UpsertArticle(ctx, otherItem))

	require.NoError(t, client.SAdd(ctx, purgedAuthorsKey, strconv.FormatInt(userID, 10)).Err())
	require.NoError(t, client.HSet(ctx, articleKey(itemID), map[string]any{
		"id":           itemID,
		"author_id":    userID,
		"title":        "stale legacy item",
		"published_at": 900,
		"updated_at":   900,
	}).Err())
	for _, key := range []string{latestKey, hotKey, activeKey, authorLatestKey(userID)} {
		require.NoError(t, client.ZAdd(ctx, key, redis.Z{Score: 900, Member: itemID}).Err())
	}

	latest, err := repo.ListLatest(ctx, 10, 0, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{otherItem.ID}, itemIDs(latest))
	hot, err := repo.ListHot(ctx, 10, 0)
	require.NoError(t, err)
	require.Equal(t, []int64{otherItem.ID}, itemIDs(hot))
	active, err := repo.ListActive(ctx, 10, 0)
	require.NoError(t, err)
	require.Equal(t, []int64{otherItem.ID}, itemIDs(active))
	byAuthor, err := repo.ListLatest(ctx, 10, 0, []int64{userID})
	require.NoError(t, err)
	require.Empty(t, byAuthor)
}

func requireIndexMemberMissing(t *testing.T, ctx context.Context, client *redis.Client, key string, itemID int64) {
	t.Helper()
	_, err := client.ZScore(ctx, key, strconv.FormatInt(itemID, 10)).Result()
	require.ErrorIs(t, err, redis.Nil)
}

func requireIndexMemberPresent(t *testing.T, ctx context.Context, client *redis.Client, key string, itemID int64) {
	t.Helper()
	_, err := client.ZScore(ctx, key, strconv.FormatInt(itemID, 10)).Result()
	require.NoError(t, err)
}
