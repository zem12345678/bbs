package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "search-service/api/proto/searchpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testInternalAuthToken = "search-internal-token"

func TestInternalAuthInterceptorProtectsEverySearchMethod(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor(testInternalAuthToken)
	if got := len(pb.SearchService_ServiceDesc.Methods); got != 16 {
		t.Fatalf("search unary method count = %d, want 16", got)
	}

	for _, method := range pb.SearchService_ServiceDesc.Methods {
		fullMethod := "/" + pb.SearchService_ServiceDesc.ServiceName + "/" + method.MethodName
		t.Run(method.MethodName, func(t *testing.T) {
			handlerCalled := false
			_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: fullMethod}, func(context.Context, any) (any, error) {
				handlerCalled = true
				return "ok", nil
			})
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("status code = %s, want %s", status.Code(err), codes.Unauthenticated)
			}
			if handlerCalled {
				t.Fatal("business handler was called without internal authentication")
			}
		})
	}
}

func TestInternalAuthPermitsOnlyHealthCheckWithoutToken(t *testing.T) {
	if requiresInternalAuthentication(grpc_health_v1.Health_Check_FullMethodName) {
		t.Fatal("Health Check must remain available without internal authentication")
	}
	if !requiresInternalAuthentication(grpc_health_v1.Health_Watch_FullMethodName) {
		t.Fatal("Health Watch must require internal authentication")
	}
	if !requiresInternalAuthentication("/example.v1.FutureService/FutureMethod") {
		t.Fatal("unknown RPCs must require internal authentication")
	}
}

func TestInternalAuthInterceptorAcceptsOnlyValidToken(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor(testInternalAuthToken)
	for _, tc := range []struct {
		name  string
		token string
		want  codes.Code
	}{
		{name: "valid", token: testInternalAuthToken, want: codes.OK},
		{name: "invalid", token: "wrong-token", want: codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, tc.token))
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: pb.SearchService_SearchArticles_FullMethodName}, func(context.Context, any) (any, error) {
				return "ok", nil
			})
			if status.Code(err) != tc.want {
				t.Fatalf("status code = %s, want %s", status.Code(err), tc.want)
			}
		})
	}
}

func TestInternalAuthGRPCServer(t *testing.T) {
	conn := newAuthenticatedTestConnection(t)
	client := pb.NewSearchServiceClient(conn)
	healthClient := grpc_health_v1.NewHealthClient(conn)

	if _, err := client.EnsureArticleIndex(context.Background(), &pb.EnsureArticleIndexRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous business RPC status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	authCtx := metadata.AppendToOutgoingContext(context.Background(), internalAuthMetadataKey, testInternalAuthToken)
	response, err := client.EnsureArticleIndex(authCtx, &pb.EnsureArticleIndexRequest{})
	if err != nil {
		t.Fatalf("authorized business RPC: %v", err)
	}
	if !response.GetSuccess() {
		t.Fatalf("authorized business response = %+v", response)
	}

	check, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("anonymous health check: %v", err)
	}
	if check.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %s, want %s", check.GetStatus(), grpc_health_v1.HealthCheckResponse_SERVING)
	}

	watch, err := healthClient.Watch(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("start anonymous health watch: %v", err)
	}
	if _, err := watch.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous health watch status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	watch, err = healthClient.Watch(authCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("start authenticated health watch: %v", err)
	}
	watchStatus, err := watch.Recv()
	if err != nil {
		t.Fatalf("authenticated health watch: %v", err)
	}
	if watchStatus.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("authenticated health watch status = %s, want %s", watchStatus.GetStatus(), grpc_health_v1.HealthCheckResponse_SERVING)
	}
}

func newAuthenticatedTestConnection(t *testing.T) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(newInternalAuthUnaryServerInterceptor(testInternalAuthToken)),
		grpc.StreamInterceptor(newInternalAuthStreamServerInterceptor(testInternalAuthToken)),
	)
	pb.RegisterSearchServiceServer(server, testSearchService{})
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial test grpc server: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

type testSearchService struct {
	pb.UnimplementedSearchServiceServer
}

func (testSearchService) EnsureArticleIndex(context.Context, *pb.EnsureArticleIndexRequest) (*pb.SimpleResponse, error) {
	return &pb.SimpleResponse{Success: true}, nil
}
