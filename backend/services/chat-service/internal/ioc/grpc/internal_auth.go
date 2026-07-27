package grpc

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const internalAuthMetadataKey = "x-bbs-internal-token"

func newInternalAuthUnaryServerInterceptor(expectedToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isUnauthenticatedHealthCheck(info.FullMethod) {
			return handler(ctx, req)
		}
		if err := authorizeInternalRequest(ctx, expectedToken); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func newInternalAuthStreamServerInterceptor(expectedToken string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := authorizeInternalRequest(stream.Context(), expectedToken); err != nil {
			return err
		}
		return handler(srv, stream)
	}
}

// Health Check is intentionally unauthenticated so Kubernetes and standard
// gRPC health-probe clients can determine whether this process is serving.
// Business RPCs and the streaming Health.Watch endpoint still require the
// internal token, keeping the unauthenticated surface to a single liveness
// response.
func isUnauthenticatedHealthCheck(fullMethod string) bool {
	return fullMethod == grpc_health_v1.Health_Check_FullMethodName
}

func authorizeInternalRequest(ctx context.Context, expectedToken string) error {
	expectedToken = strings.TrimSpace(expectedToken)
	if expectedToken == "" {
		return status.Error(codes.Unauthenticated, "invalid internal authentication token")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "invalid internal authentication token")
	}
	for _, token := range md.Get(internalAuthMetadataKey) {
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1 {
			return nil
		}
	}
	return status.Error(codes.Unauthenticated, "invalid internal authentication token")
}
