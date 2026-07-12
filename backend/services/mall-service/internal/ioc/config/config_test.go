package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestApplyEnvOverridesSetsMallGRPCPorts(t *testing.T) {
	t.Setenv("BBS_MALL_GRPC_SERVER_PORT", "19115")

	v := viper.New()
	v.Set("service.grpcPort", 9115)
	v.Set("grpc.server.port", 9115)
	configureEnv(v)
	applyEnvOverrides(v)

	if got := v.GetInt("service.grpcPort"); got != 19115 {
		t.Fatalf("service.grpcPort = %d, want 19115", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19115 {
		t.Fatalf("grpc.server.port = %d, want 19115", got)
	}
}

func TestApplyEnvOverridesAcceptsLegacyMallServicePort(t *testing.T) {
	t.Setenv("BBS_MALL_SERVICE_GRPC_PORT", "19116")

	v := viper.New()
	v.Set("service.grpcPort", 9115)
	v.Set("grpc.server.port", 9115)
	configureEnv(v)
	applyEnvOverrides(v)

	if got := v.GetInt("service.grpcPort"); got != 19116 {
		t.Fatalf("service.grpcPort = %d, want 19116", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19116 {
		t.Fatalf("grpc.server.port = %d, want 19116", got)
	}
}
