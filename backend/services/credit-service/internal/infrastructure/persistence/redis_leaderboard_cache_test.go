package persistence

import (
	"context"
	"reflect"
	"testing"

	domain "credit-service/internal/domain/credit"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestLeaderboardMemberRoundTripPreservesNumericOrder(t *testing.T) {
	for _, userID := range []int64{1, 7, 42, 9223372036854775807} {
		member := leaderboardMember(userID)
		parsed, ok := parseLeaderboardMember(member)
		if !ok || parsed != userID {
			t.Fatalf("leaderboard member %q parsed as %d, %v", member, parsed, ok)
		}
	}
	if leaderboardMember(42) >= leaderboardMember(100) {
		t.Fatalf("leaderboard members must sort numerically: %q >= %q", leaderboardMember(42), leaderboardMember(100))
	}
}

func TestParseLeaderboardMemberRejectsInvalidValues(t *testing.T) {
	for _, member := range []string{"", "u:42", "u:0000000000000000000", "x:0000000000000000042"} {
		if _, ok := parseLeaderboardMember(member); ok {
			t.Fatalf("invalid leaderboard member %q was accepted", member)
		}
	}
}

func TestRedisLeaderboardCacheAppliesExpectedRevisionUpdates(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewRedisLeaderboardCache(client)
	ctx := context.Background()

	if err := cache.Replace(ctx, 7, []domain.LeaderboardEntry{
		{UserID: 1, Total: 50},
		{UserID: 2, Total: 10},
	}); err != nil {
		t.Fatalf("seed leaderboard cache: %v", err)
	}

	updated, err := cache.Apply(ctx, 7, 10, []domain.LeaderboardEntry{
		{UserID: 1, Total: 0},
		{UserID: 2, Total: 35},
		{UserID: 3, Total: 25},
	})
	if err != nil {
		t.Fatalf("apply leaderboard cache update: %v", err)
	}
	if !updated {
		t.Fatal("expected matching revision update to apply")
	}
	assertLeaderboardCache(t, cache, ctx, 10, []domain.LeaderboardEntry{
		{UserID: 2, Total: 35, Rank: 1},
		{UserID: 3, Total: 25, Rank: 2},
	})

	updated, err = cache.Apply(ctx, 7, 11, []domain.LeaderboardEntry{{UserID: 3, Total: 100}})
	if err != nil {
		t.Fatalf("apply stale leaderboard cache update: %v", err)
	}
	if updated {
		t.Fatal("stale revision update must not apply")
	}
	assertLeaderboardCache(t, cache, ctx, 10, []domain.LeaderboardEntry{
		{UserID: 2, Total: 35, Rank: 1},
		{UserID: 3, Total: 25, Rank: 2},
	})

	if err := cache.Replace(ctx, 9, []domain.LeaderboardEntry{{UserID: 4, Total: 99}}); err != nil {
		t.Fatalf("replace stale leaderboard cache snapshot: %v", err)
	}
	assertLeaderboardCache(t, cache, ctx, 10, []domain.LeaderboardEntry{
		{UserID: 2, Total: 35, Rank: 1},
		{UserID: 3, Total: 25, Rank: 2},
	})
}

func assertLeaderboardCache(t *testing.T, cache *RedisLeaderboardCache, ctx context.Context, wantRevision int64, want []domain.LeaderboardEntry) {
	t.Helper()
	items, revision, hit, err := cache.List(ctx, 10)
	if err != nil {
		t.Fatalf("list leaderboard cache: %v", err)
	}
	if !hit {
		t.Fatal("expected leaderboard cache hit")
	}
	if revision != wantRevision {
		t.Fatalf("leaderboard revision = %d, want %d", revision, wantRevision)
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("leaderboard items = %#v, want %#v", items, want)
	}
}
