package grpc

import (
	"context"
	"errors"
	"testing"

	"chat-service/pkg/ratelimit"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestServiceRateLimitUnaryServerInterceptor(t *testing.T) {
	tests := []struct {
		name       string
		limiter    *rateLimiterStub
		fullMethod string
		wantCode   codes.Code
		called     bool
		wantKey    string
	}{
		{
			name:       "allows business RPC",
			limiter:    &rateLimiterStub{},
			fullMethod: "/bbs.chat.v1.ChatService/SendMessage",
			wantCode:   codes.OK,
			called:     true,
			wantKey:    "rate:chat-service:grpc:bbs.chat.v1.ChatService:SendMessage",
		},
		{
			name:       "blocks business RPC",
			limiter:    &rateLimiterStub{limited: true},
			fullMethod: "/bbs.chat.v1.ChatService/SendMessage",
			wantCode:   codes.ResourceExhausted,
			wantKey:    "rate:chat-service:grpc:bbs.chat.v1.ChatService:SendMessage",
		},
		{
			name:       "fails closed when Redis is unavailable",
			limiter:    &rateLimiterStub{err: errors.New("redis unavailable")},
			fullMethod: "/bbs.chat.v1.ChatService/SendMessage",
			wantCode:   codes.Unavailable,
			wantKey:    "rate:chat-service:grpc:bbs.chat.v1.ChatService:SendMessage",
		},
		{
			name:       "does not limit health check",
			limiter:    &rateLimiterStub{limited: true},
			fullMethod: grpc_health_v1.Health_Check_FullMethodName,
			wantCode:   codes.OK,
			called:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			_, err := newServiceRateLimitUnaryServerInterceptor(test.limiter)(
				context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: test.fullMethod}, func(context.Context, any) (any, error) {
					called = true
					return nil, nil
				},
			)
			if got := status.Code(err); got != test.wantCode {
				t.Fatalf("status code = %s, want %s", got, test.wantCode)
			}
			if called != test.called {
				t.Fatalf("handler called = %t, want %t", called, test.called)
			}
			if test.wantKey == "" {
				if len(test.limiter.keys) != 0 {
					t.Fatalf("limiter keys = %#v, want none", test.limiter.keys)
				}
			} else if len(test.limiter.keys) != 1 || test.limiter.keys[0] != test.wantKey {
				t.Fatalf("limiter keys = %#v, want %q", test.limiter.keys, test.wantKey)
			}
		})
	}
}

type rateLimiterStub struct {
	limited bool
	err     error
	keys    []string
}

func (l *rateLimiterStub) Limit(_ context.Context, key string) (bool, error) {
	l.keys = append(l.keys, key)
	return l.limited, l.err
}

var _ ratelimit.Limiter = (*rateLimiterStub)(nil)
