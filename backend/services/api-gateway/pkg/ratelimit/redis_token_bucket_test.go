package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestTokenBucketCapacityAndRefill(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	key := "rate:test:token-bucket"
	limitAt := func(now int64) bool {
		t.Helper()
		limited, err := client.Eval(ctx, luaTokenBucket, []string{key}, 3, 500, now).Bool()
		if err != nil {
			t.Fatalf("eval token bucket: %v", err)
		}
		return limited
	}

	if limitAt(1_000) || limitAt(1_000) || limitAt(1_000) {
		t.Fatal("initial capacity was not available")
	}
	if !limitAt(1_000) || !limitAt(1_499) {
		t.Fatal("empty bucket allowed a request before refill")
	}
	if limitAt(1_500) {
		t.Fatal("one token was not restored after 500ms")
	}
	if !limitAt(1_500) {
		t.Fatal("refilled token was not consumed")
	}
	if limitAt(3_000) || limitAt(3_000) || limitAt(3_000) {
		t.Fatal("bucket did not refill to its capacity")
	}
	if !limitAt(3_000) {
		t.Fatal("bucket exceeded its capacity")
	}
}
