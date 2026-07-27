//go:build integration

package grpc

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"chat-service/pkg/ratelimit"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	stdgrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServiceRateLimitIsSharedAcrossInstances(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("BBS_CHAT_TEST_REDIS_ADDR"))
	if addr == "" {
		t.Skip("set BBS_CHAT_TEST_REDIS_ADDR to run the Redis rate-limit integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientA := redis.NewClient(&redis.Options{Addr: addr})
	defer clientA.Close()
	clientB := redis.NewClient(&redis.Options{Addr: addr})
	defer clientB.Close()
	if err := clientA.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	info := &stdgrpc.UnaryServerInfo{FullMethod: "/test.RateLimitService/" + uuid.NewString()}
	key := serviceRateLimitKey(info.FullMethod)
	defer clientA.Del(context.Background(), key)
	first := ratelimit.NewRedisSlidingWindowLimiter(clientA, time.Minute, 1)
	second := ratelimit.NewRedisSlidingWindowLimiter(clientB, time.Minute, 1)

	if _, err := newServiceRateLimitUnaryServerInterceptor(first)(ctx, nil, info, func(context.Context, any) (any, error) { return nil, nil }); err != nil {
		t.Fatalf("first interceptor call: %v", err)
	}

	_, err := newServiceRateLimitUnaryServerInterceptor(second)(ctx, nil, info, func(context.Context, any) (any, error) { return nil, nil })
	if got := status.Code(err); got != codes.ResourceExhausted {
		t.Fatalf("second interceptor status = %s, want %s", got, codes.ResourceExhausted)
	}
}
