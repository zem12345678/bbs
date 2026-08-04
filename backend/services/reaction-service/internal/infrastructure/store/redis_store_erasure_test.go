package store

import (
	"errors"
	"strconv"
	"testing"

	accountDomain "reaction-service/internal/domain/account"
	reactionDomain "reaction-service/internal/domain/reaction"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisStoreAccountTombstoneBlocksAddsAndPurgeRepairsCounts(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRedisStore(client)
	ctx := t.Context()

	userID := int64(42)
	otherUserID := int64(43)
	article := reactionDomain.EntityRef{Type: reactionDomain.EntityArticle, ID: 101}
	topic := reactionDomain.EntityRef{Type: reactionDomain.EntityTopic, ID: 102}
	for _, candidate := range []int64{userID, otherUserID} {
		if _, _, err := store.Like(ctx, article, candidate); err != nil {
			t.Fatalf("seed like: %v", err)
		}
		if _, _, err := store.Favorite(ctx, article, candidate); err != nil {
			t.Fatalf("seed favorite: %v", err)
		}
	}
	if _, _, err := store.Like(ctx, topic, userID); err != nil {
		t.Fatalf("seed second like: %v", err)
	}
	if _, _, err := store.Favorite(ctx, topic, userID); err != nil {
		t.Fatalf("seed second favorite: %v", err)
	}

	if err := store.TombstoneAccount(ctx, userID, 9001, 3); err != nil {
		t.Fatalf("TombstoneAccount() error = %v", err)
	}
	late := reactionDomain.EntityRef{Type: reactionDomain.EntityArticle, ID: 103}
	if _, _, err := store.Like(ctx, late, userID); !errors.Is(err, accountDomain.ErrUserErased) {
		t.Fatalf("late Like() error = %v, want ErrUserErased", err)
	}
	if _, _, err := store.Favorite(ctx, late, userID); !errors.Is(err, accountDomain.ErrUserErased) {
		t.Fatalf("late Favorite() error = %v, want ErrUserErased", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if err := store.PurgeAccount(ctx, userID); err != nil {
			t.Fatalf("PurgeAccount() attempt %d error = %v", attempt+1, err)
		}
	}
	if got, err := store.LikeCount(ctx, article); err != nil || got != 1 {
		t.Fatalf("article like count = %d, error = %v, want 1", got, err)
	}
	if got, err := store.FavoriteCount(ctx, article); err != nil || got != 1 {
		t.Fatalf("article favorite count = %d, error = %v, want 1", got, err)
	}
	if got, err := store.LikeCount(ctx, topic); err != nil || got != 0 {
		t.Fatalf("topic like count = %d, error = %v, want 0", got, err)
	}
	if got, err := store.FavoriteCount(ctx, topic); err != nil || got != 0 {
		t.Fatalf("topic favorite count = %d, error = %v, want 0", got, err)
	}
	if score, err := client.ZScore(ctx, hotKey(article.Type), strconv.FormatInt(article.ID, 10)).Result(); err != nil || score != 1 {
		t.Fatalf("article hot score = %v, error = %v, want 1", score, err)
	}
	if _, err := client.ZScore(ctx, hotKey(topic.Type), strconv.FormatInt(topic.ID, 10)).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("topic hot entry error = %v, want redis.Nil", err)
	}
	if exists, err := client.Exists(ctx, erasedUserKey(userID)).Result(); err != nil || exists != 1 {
		t.Fatalf("erased user tombstone exists = %d, error = %v", exists, err)
	}
}
