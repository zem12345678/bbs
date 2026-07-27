package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSlideWindowKeepsSameMillisecondRequestsDistinct(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	key := "rate:test:same-millisecond"
	now := int64(1_700_000_000_000)
	for _, nonce := range []string{"first", "second"} {
		limited, err := client.Eval(ctx, luaSlideWindow, []string{key}, 60_000, 2, now, nonce).Bool()
		if err != nil {
			t.Fatalf("eval limiter: %v", err)
		}
		if limited {
			t.Fatalf("request with nonce %q was unexpectedly limited", nonce)
		}
	}
	if count, err := client.ZCard(ctx, key).Result(); err != nil || count != 2 {
		t.Fatalf("zcard = %d, err = %v, want 2", count, err)
	}

	limited, err := client.Eval(ctx, luaSlideWindow, []string{key}, 60_000, 2, now, "third").Bool()
	if err != nil {
		t.Fatalf("eval third request: %v", err)
	}
	if !limited {
		t.Fatal("third request was not limited")
	}
}
