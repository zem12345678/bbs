package grpc

import (
	"context"
	"testing"

	pb "admin/api/proto/adminpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestInternalAuthUnaryServerInterceptorProtectsEveryAdministrativeRPC(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor("expected-token")
	handler := func(context.Context, any) (any, error) { return "ok", nil }

	for _, method := range pb.AdminService_ServiceDesc.Methods {
		fullMethod := "/bbs.admin.v1.AdminService/" + method.MethodName
		t.Run(method.MethodName, func(t *testing.T) {
			info := &grpc.UnaryServerInfo{FullMethod: fullMethod}
			for _, ctx := range []context.Context{
				context.Background(),
				metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "wrong-token")),
			} {
				_, err := interceptor(ctx, nil, info, handler)
				if status.Code(err) != codes.Unauthenticated {
					t.Fatalf("status code = %s, want %s", status.Code(err), codes.Unauthenticated)
				}
			}

			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "expected-token"))
			response, err := interceptor(ctx, nil, info, handler)
			if err != nil || response != "ok" {
				t.Fatalf("authorized response = %#v, error = %v", response, err)
			}
		})
	}
}

func TestInternalAuthUnaryServerInterceptorAllowsHealthCheckOnly(t *testing.T) {
	interceptor := newInternalAuthUnaryServerInterceptor("expected-token")
	response, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: grpc_health_v1.Health_Check_FullMethodName}, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil || response != "ok" {
		t.Fatalf("health response = %#v, error = %v", response, err)
	}
}

func TestInternalAuthStreamServerInterceptorProtectsHealthWatch(t *testing.T) {
	interceptor := newInternalAuthStreamServerInterceptor("expected-token")
	info := &grpc.StreamServerInfo{FullMethod: grpc_health_v1.Health_Watch_FullMethodName}

	err := interceptor(nil, adminTestServerStream{ctx: context.Background()}, info, func(any, grpc.ServerStream) error { return nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthorized health watch status = %s, want %s", status.Code(err), codes.Unauthenticated)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalAuthMetadataKey, "expected-token"))
	err = interceptor(nil, adminTestServerStream{ctx: ctx}, info, func(any, grpc.ServerStream) error { return nil })
	if err != nil {
		t.Fatalf("authorized health watch error = %v", err)
	}
}

type adminTestServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s adminTestServerStream) Context() context.Context { return s.ctx }
