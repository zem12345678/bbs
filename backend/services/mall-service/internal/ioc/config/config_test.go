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

func TestApplyEnvOverridesSetsMallServiceName(t *testing.T) {
	t.Setenv("BBS_MALL_GRPC_SERVER_SERVICE_NAME", "bbs-mall-service-e2e")

	v := viper.New()
	v.Set("service.name", "bbs-mall-service")
	v.Set("grpc.server.serviceName", "bbs-mall-service")
	configureEnv(v)
	applyEnvOverrides(v)

	if got := v.GetString("service.name"); got != "bbs-mall-service-e2e" {
		t.Fatalf("service.name = %q, want bbs-mall-service-e2e", got)
	}
	if got := v.GetString("grpc.server.serviceName"); got != "bbs-mall-service-e2e" {
		t.Fatalf("grpc.server.serviceName = %q, want bbs-mall-service-e2e", got)
	}
}

func TestSetDefaultsFillsCreditUpstream(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.credit"); got != "bbs-credit-service" {
		t.Fatalf("upstreams.credit = %q", got)
	}
}

func TestConfigureEnvBindsCreditUpstream(t *testing.T) {
	t.Setenv("BBS_MALL_UPSTREAMS_CREDIT", "file-credit-service")

	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	if got := v.GetString("upstreams.credit"); got != "file-credit-service" {
		t.Fatalf("upstreams.credit = %q", got)
	}
}
