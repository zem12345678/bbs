package grpc

import (
	"context"
	"net"
	"testing"

	pb "mall-service/api/proto/mallpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestRequiresInternalAuthentication(t *testing.T) {
	if !requiresInternalAuthentication(pb.MallService_CreateOrder_FullMethodName) {
		t.Fatal("CreateOrder must require internal authentication")
	}
	if requiresInternalAuthentication(pb.MallService_HealthCheck_FullMethodName) {
		t.Fatal("HealthCheck must remain available to unauthenticated health probes")
	}
	if requiresInternalAuthentication("/grpc.health.v1.Health/Check") {
		t.Fatal("standard gRPC health checks must not require mall authentication")
	}
}

func TestInternalAuthUnaryServerInterceptor(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor("mall-internal-token")
	called := false
	handler := func(context.Context, any) (any, error) {
		called = true
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: pb.MallService_CreateOrder_FullMethodName}

	_, err := interceptor(context.Background(), nil, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token status = %v, want Unauthenticated", status.Code(err))
	}
	if called {
		t.Fatal("handler called with no authentication token")
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "mall-internal-token"))
	response, err := interceptor(ctx, nil, info, handler)
	if err != nil || response != "ok" {
		t.Fatalf("authenticated interceptor response/error = %#v/%v", response, err)
	}
	if !called {
		t.Fatal("handler not called with valid authentication token")
	}
}

func TestInternalAuthUnaryServerInterceptorAllowsHealthCheck(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor("mall-internal-token")
	called := false

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: pb.MallService_HealthCheck_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("HealthCheck interceptor error = %v", err)
	}
	if !called {
		t.Fatal("HealthCheck handler was not called")
	}
}

func TestInternalAuthStreamServerInterceptor(t *testing.T) {
	interceptor := newInternalAuthStreamServerInterceptor("mall-internal-token")
	called := false
	info := &grpc.StreamServerInfo{FullMethod: pb.MallService_CreateOrder_FullMethodName}

	err := interceptor(nil, mallAuthTestStream{ctx: context.Background()}, info, func(any, grpc.ServerStream) error {
		called = true
		return nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token status = %v, want Unauthenticated", status.Code(err))
	}
	if called {
		t.Fatal("stream handler called with no authentication token")
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "mall-internal-token"))
	err = interceptor(nil, mallAuthTestStream{ctx: ctx}, info, func(any, grpc.ServerStream) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("authenticated stream interceptor error: %v", err)
	}
	if !called {
		t.Fatal("stream handler not called with valid authentication token")
	}
}

func TestInternalAuthUnaryServerInterceptorEnforcesMetadataOverGRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(newInternalAuthUnaryServerInterceptor("mall-internal-token")))
	pb.RegisterMallServiceServer(server, mallAuthTestServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	dialOptions := []grpc.DialOption{
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	unauthenticatedConn, err := grpc.NewClient("passthrough:///mall-auth-test", dialOptions...)
	if err != nil {
		t.Fatalf("create unauthenticated client: %v", err)
	}
	t.Cleanup(func() { _ = unauthenticatedConn.Close() })

	_, err = pb.NewMallServiceClient(unauthenticatedConn).ListProducts(context.Background(), &pb.ListProductsRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated ListProducts status = %v, want Unauthenticated", status.Code(err))
	}

	authenticatedConn, err := grpc.NewClient("passthrough:///mall-auth-test", append(dialOptions, grpc.WithPerRPCCredentials(testInternalAuthCredentials{token: "mall-internal-token"}))...)
	if err != nil {
		t.Fatalf("create authenticated client: %v", err)
	}
	t.Cleanup(func() { _ = authenticatedConn.Close() })

	if _, err := pb.NewMallServiceClient(authenticatedConn).ListProducts(context.Background(), &pb.ListProductsRequest{}); err != nil {
		t.Fatalf("authenticated ListProducts: %v", err)
	}
}

type mallAuthTestServer struct {
	pb.UnimplementedMallServiceServer
}

type mallAuthTestStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s mallAuthTestStream) Context() context.Context { return s.ctx }

func (mallAuthTestServer) ListProducts(context.Context, *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	return &pb.ListProductsResponse{}, nil
}

type testInternalAuthCredentials struct {
	token string
}

func (c testInternalAuthCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{internalAuthMetadataKey: c.token}, nil
}

func (testInternalAuthCredentials) RequireTransportSecurity() bool {
	return false
}
