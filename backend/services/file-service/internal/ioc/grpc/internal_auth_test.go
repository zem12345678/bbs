package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "file-service/api/proto/filepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testInternalAuthToken = "file-internal-token"

func TestInternalAuthInterceptorProtectsEveryFileServiceMethod(t *testing.T) {
	interceptor := NewInternalAuthUnaryServerInterceptor(testInternalAuthToken)

	for _, test := range []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing token", ctx: context.Background()},
		{name: "wrong token", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "wrong-token"))},
	} {
		for _, method := range pb.FileService_ServiceDesc.Methods {
			fullMethod := "/" + pb.FileService_ServiceDesc.ServiceName + "/" + method.MethodName
			t.Run(test.name+"/"+method.MethodName, func(t *testing.T) {
				handlerCalled := false
				_, err := interceptor(test.ctx, nil, &grpc.UnaryServerInfo{FullMethod: fullMethod}, func(context.Context, any) (any, error) {
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
}

func TestInternalAuthPermitsOnlyHealthCheckWithoutToken(t *testing.T) {
	if requiresInternalAuthentication(grpc_health_v1.Health_Check_FullMethodName) {
		t.Fatal("Health Check must remain available without internal authentication")
	}
	if !requiresInternalAuthentication(grpc_health_v1.Health_Watch_FullMethodName) {
		t.Fatal("Health Watch must require internal authentication")
	}
}

func TestInternalAuthGRPCServer(t *testing.T) {
	conn := newAuthenticatedTestConnection(t)
	fileClient := pb.NewFileServiceClient(conn)
	healthClient := grpc_health_v1.NewHealthClient(conn)

	if _, err := fileClient.GetAttachment(context.Background(), &pb.GetAttachmentRequest{AttachmentId: 1}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous business RPC status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	authCtx := metadata.AppendToOutgoingContext(context.Background(), internalAuthMetadataKey, testInternalAuthToken)
	if _, err := fileClient.GetAttachment(authCtx, &pb.GetAttachmentRequest{AttachmentId: 1}); err != nil {
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
}

func newAuthenticatedTestConnection(t *testing.T) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(NewInternalAuthUnaryServerInterceptor(testInternalAuthToken)),
		grpc.StreamInterceptor(NewInternalAuthStreamServerInterceptor(testInternalAuthToken)),
	)
	pb.RegisterFileServiceServer(server, testFileService{})
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

type testFileService struct {
	pb.UnimplementedFileServiceServer
}

func (testFileService) GetAttachment(context.Context, *pb.GetAttachmentRequest) (*pb.AttachmentResponse, error) {
	return &pb.AttachmentResponse{}, nil
}
