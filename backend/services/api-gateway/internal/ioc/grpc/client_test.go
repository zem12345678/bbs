package grpc

import (
	"testing"

	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
)

func TestGrpcClientCodeToLevel(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		want zapcore.Level
	}{
		{name: "ok is info", code: codes.OK, want: zapcore.InfoLevel},
		{name: "internal is error", code: codes.Internal, want: zapcore.ErrorLevel},
		{name: "unavailable is error", code: codes.Unavailable, want: zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := grpcClientCodeToLevel(tt.code); got != tt.want {
				t.Fatalf("grpcClientCodeToLevel(%s) = %s, want %s", tt.code, got, tt.want)
			}
		})
	}
}
