package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "credit-service/api/proto/creditpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testInternalAuthToken = "credit-internal-token"

func TestInternalAuthInterceptorProtectsEveryCreditServiceMethod(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor(testInternalAuthToken)

	for _, method := range pb.CreditService_ServiceDesc.Methods {
		fullMethod := "/" + pb.CreditService_ServiceDesc.ServiceName + "/" + method.MethodName
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
	if !requiresInternalAuthentication("/example.v1.OtherService/Method") {
		t.Fatal("unknown RPCs must require internal authentication")
	}
}

func TestInternalAuthInterceptorAcceptsValidToken(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor(testInternalAuthToken)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, testInternalAuthToken))
	response, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: pb.CreditService_GetBalance_FullMethodName}, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil || response != "ok" {
		t.Fatalf("authorized response = %#v, error = %v", response, err)
	}
}

func TestInternalAuthStreamServerInterceptor(t *testing.T) {
	interceptor := newInternalAuthStreamServerInterceptor(testInternalAuthToken)

	for _, test := range []struct {
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
			ctx:    metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, testInternalAuthToken)),
			want:   codes.OK,
			called: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			err := interceptor(nil, testServerStream{ctx: test.ctx}, &grpc.StreamServerInfo{FullMethod: grpc_health_v1.Health_Watch_FullMethodName}, func(any, grpc.ServerStream) error {
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

func TestInternalAuthGRPCServer(t *testing.T) {
	conn := newAuthenticatedTestConnection(t)
	creditClient := pb.NewCreditServiceClient(conn)
	healthClient := grpc_health_v1.NewHealthClient(conn)

	if _, err := creditClient.GetBalance(context.Background(), &pb.GetBalanceRequest{UserId: 1}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous business RPC status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	authCtx := metadata.AppendToOutgoingContext(context.Background(), internalAuthMetadataKey, testInternalAuthToken)
	if _, err := creditClient.GetBalance(authCtx, &pb.GetBalanceRequest{UserId: 1}); err != nil {
		t.Fatalf("authorized business RPC: %v", err)
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
	pb.RegisterCreditServiceServer(server, testCreditService{})
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	t.Cleanup(func() { _ = listener.Close() })

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

type testCreditService struct {
	pb.UnimplementedCreditServiceServer
}

func (testCreditService) GetBalance(context.Context, *pb.GetBalanceRequest) (*pb.BalanceResponse, error) {
	return &pb.BalanceResponse{}, nil
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
