package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestInternalAuthUnaryServerInterceptor(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor("expected-token")

	tests := []struct {
		name   string
		ctx    context.Context
		want   codes.Code
		called bool
	}{
		{
			name: "missing token",
			ctx:  context.Background(),
			want: codes.Unauthenticated,
		},
		{
			name: "wrong token",
			ctx:  metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "wrong-token")),
			want: codes.Unauthenticated,
		},
		{
			name:   "matching token",
			ctx:    metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "expected-token")),
			want:   codes.OK,
			called: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			_, err := interceptor(test.ctx, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if got := status.Code(err); got != test.want {
				t.Fatalf("status code = %s, want %s", got, test.want)
			}
			if called != test.called {
				t.Fatalf("handler called = %t, want %t", called, test.called)
			}
		})
	}
}

func TestInternalAuthStreamServerInterceptor(t *testing.T) {
	interceptor := newInternalAuthStreamServerInterceptor("expected-token")

	tests := []struct {
		name   string
		ctx    context.Context
		want   codes.Code
		called bool
	}{
		{
			name: "missing token",
			ctx:  context.Background(),
			want: codes.Unauthenticated,
		},
		{
			name:   "matching token",
			ctx:    metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "expected-token")),
			want:   codes.OK,
			called: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			err := interceptor(nil, testServerStream{ctx: test.ctx}, &grpc.StreamServerInfo{}, func(any, grpc.ServerStream) error {
				called = true
				return nil
			})
			if got := status.Code(err); got != test.want {
				t.Fatalf("status code = %s, want %s", got, test.want)
			}
			if called != test.called {
				t.Fatalf("handler called = %t, want %t", called, test.called)
			}
		})
	}
}

func TestInternalAuthInterceptorsAllowHealthCheckButProtectHealthWatch(t *testing.T) {
	const token = "expected-token"
	client := newInternalAuthHealthClient(t, token)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unauthenticated health check: %v", err)
	}
	if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("unauthenticated health status = %s, want %s", response.GetStatus(), grpc_health_v1.HealthCheckResponse_SERVING)
	}

	authorizedCtx, authorizedCancel := context.WithTimeout(
		metadata.AppendToOutgoingContext(context.Background(), internalAuthMetadataKey, token),
		time.Second,
	)
	defer authorizedCancel()
	response, err = client.Check(authorizedCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("authorized health check: %v", err)
	}
	if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %s, want %s", response.GetStatus(), grpc_health_v1.HealthCheckResponse_SERVING)
	}

	watch, err := client.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err == nil {
		_, err = watch.Recv()
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated health watch status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	authorizedWatch, err := client.Watch(authorizedCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("authorized health watch: %v", err)
	}
	watchResponse, err := authorizedWatch.Recv()
	if err != nil {
		t.Fatalf("receive authorized health watch: %v", err)
	}
	if watchResponse.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health watch status = %s, want %s", watchResponse.GetStatus(), grpc_health_v1.HealthCheckResponse_SERVING)
	}
}

func newInternalAuthHealthClient(t *testing.T, token string) grpc_health_v1.HealthClient {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(newInternalAuthUnaryServerInterceptor(token)),
		grpc.StreamInterceptor(newInternalAuthStreamServerInterceptor(token)),
	)
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///chat-internal-auth",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create grpc client: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return grpc_health_v1.NewHealthClient(conn)
}

type testServerStream struct {
	ctx context.Context
}

func (s testServerStream) SetHeader(metadata.MD) error  { return nil }
func (s testServerStream) SendHeader(metadata.MD) error { return nil }
func (s testServerStream) SetTrailer(metadata.MD)       {}
func (s testServerStream) Context() context.Context     { return s.ctx }
func (s testServerStream) SendMsg(any) error            { return nil }
func (s testServerStream) RecvMsg(any) error            { return nil }
