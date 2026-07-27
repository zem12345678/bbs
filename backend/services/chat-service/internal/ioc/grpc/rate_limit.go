package grpc

import (
	"context"
	"strings"

	"chat-service/pkg/ratelimit"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewRateLimiter creates the service-wide gRPC limiter from the already
// configured Redis client. Keeping this in the transport IoC package makes it
// available to Wire alongside NewServer.
func NewRateLimiter(v *viper.Viper, client *redis.Client) ratelimit.Limiter {
	return ratelimit.NewRedisSlidingWindowLimiter(
		client,
		v.GetDuration("grpc.server.rateLimit.interval"),
		v.GetInt("grpc.server.rateLimit.rate"),
	)
}

func newServiceRateLimitUnaryServerInterceptor(limiter ratelimit.Limiter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isUnauthenticatedHealthCheck(info.FullMethod) {
			return handler(ctx, req)
		}
		limited, err := limiter.Limit(ctx, serviceRateLimitKey(info.FullMethod))
		if err != nil {
			return nil, status.Error(codes.Unavailable, "chat service rate limiter unavailable")
		}
		if limited {
			return nil, status.Error(codes.ResourceExhausted, "chat service rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

func serviceRateLimitKey(fullMethod string) string {
	method := strings.ReplaceAll(strings.TrimPrefix(fullMethod, "/"), "/", ":")
	return "rate:chat-service:grpc:" + method
}
