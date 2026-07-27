package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "notification-service/api/proto/notificationpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testInternalAuthToken = "notification-internal-token"

func TestInternalAuthInterceptorProtectsEveryNotificationMethod(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor(testInternalAuthToken)
	services := []grpc.ServiceDesc{pb.NotificationService_ServiceDesc, pb.InternalNotificationService_ServiceDesc}
	for _, service := range services {
		for _, method := range service.Methods {
			fullMethod := "/" + service.ServiceName + "/" + method.MethodName
			t.Run(service.ServiceName+"/"+method.MethodName, func(t *testing.T) {
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
}

func TestInternalAuthProtectsUnknownMethodsAndHealthWatch(t *testing.T) {
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
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: pb.NotificationService_CountUnread_FullMethodName}, func(context.Context, any) (any, error) {
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
	client := pb.NewNotificationServiceClient(conn)
	internalClient := pb.NewInternalNotificationServiceClient(conn)
	healthClient := grpc_health_v1.NewHealthClient(conn)

	if _, err := client.CountUnread(context.Background(), &pb.CountUnreadRequest{UserId: 1}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous user RPC status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}
	if _, err := internalClient.DispatchSystemNotifications(context.Background(), &pb.DispatchSystemNotificationsRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("anonymous internal RPC status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	authCtx := metadata.AppendToOutgoingContext(context.Background(), internalAuthMetadataKey, testInternalAuthToken)
	if _, err := client.CountUnread(authCtx, &pb.CountUnreadRequest{UserId: 1}); err != nil {
		t.Fatalf("authorized user RPC: %v", err)
	}
	if _, err := internalClient.DispatchSystemNotifications(authCtx, &pb.DispatchSystemNotificationsRequest{}); err != nil {
		t.Fatalf("authorized internal RPC: %v", err)
	}

	if _, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("anonymous health check: %v", err)
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
		t.Fatalf("start authorized health watch: %v", err)
	}
	if _, err := watch.Recv(); err != nil {
		t.Fatalf("authorized health watch: %v", err)
	}
}

func newAuthenticatedTestConnection(t *testing.T) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(newInternalAuthUnaryServerInterceptor(testInternalAuthToken)),
		grpc.StreamInterceptor(newInternalAuthStreamServerInterceptor(testInternalAuthToken)),
	)
	pb.RegisterNotificationServiceServer(server, testNotificationService{})
	pb.RegisterInternalNotificationServiceServer(server, testNotificationService{})
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

type testNotificationService struct {
	pb.UnimplementedNotificationServiceServer
	pb.UnimplementedInternalNotificationServiceServer
}

func (testNotificationService) CountUnread(context.Context, *pb.CountUnreadRequest) (*pb.CountUnreadResponse, error) {
	return &pb.CountUnreadResponse{Count: 1}, nil
}

func (testNotificationService) DispatchSystemNotifications(context.Context, *pb.DispatchSystemNotificationsRequest) (*pb.DispatchSystemNotificationsResponse, error) {
	return &pb.DispatchSystemNotificationsResponse{DeliveredCount: 1}, nil
}
