package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyEnvOverridesSetsMallGRPCPorts(t *testing.T) {
	t.Setenv("BBS_MALL_GRPC_SERVER_PORT", "19115")
	t.Setenv("BBS_MALL_POSTGRES_DSN", "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_mall")
	t.Setenv("BBS_MALL_POSTGRES_DEBUG", "true")

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
	if got := v.GetString("postgres.dsn"); got != "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_mall" {
		t.Fatalf("postgres.dsn = %q", got)
	}
	if !v.GetBool("postgres.debug") {
		t.Fatal("postgres.debug should be true")
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

func TestApplyEnvOverridesSetsEtcdEndpoints(t *testing.T) {
	t.Setenv("BBS_MALL_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379,,")
	t.Setenv("BBS_MALL_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")

	v := viper.New()
	configureEnv(v)
	applyEnvOverrides(v)

	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
}

func TestSetDefaultsFillsCreditUpstream(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.credit"); got != "bbs-credit-service" {
		t.Fatalf("upstreams.credit = %q", got)
	}
	if got := v.GetString("upstreams.creditInternalAuthToken"); got != localDevCreditInternalAuthToken {
		t.Fatalf("upstreams.creditInternalAuthToken = %q", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_MALL_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "configured-mall-token")
	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != "configured-mall-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsPostgresSettings(t *testing.T) {
	t.Setenv("BBS_MALL_POSTGRES_DSN", "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_mall")
	t.Setenv("BBS_MALL_POSTGRES_DEBUG", "true")
	v := viper.New()
	configureEnv(v)

	if got := v.GetString("postgres.dsn"); got != "postgres://user:password@127.0.0.1:25432/bbs?sslmode=disable&search_path=bbs_mall" {
		t.Fatalf("postgres.dsn = %q", got)
	}
	if !v.GetBool("postgres.debug") {
		t.Fatal("postgres.debug should be true")
	}
}

func TestValidateRejectsDefaultInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default token rejection")
	}
}

func TestValidateAcceptsConfiguredInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "prod")
	v.Set("grpc.server.internalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")

	if err := validate(v); err != nil {
		t.Fatalf("validate() error = %v", err)
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

func TestConfigureEnvBindsCreditInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_MALL_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN", "configured-credit-token")
	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	if got := v.GetString("upstreams.creditInternalAuthToken"); got != "configured-credit-token" {
		t.Fatalf("upstreams.creditInternalAuthToken = %q", got)
	}
}

func TestValidateRejectsDefaultCreditInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("grpc.server.internalAuthToken", "production-mall-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default credit token rejection")
	}
}

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_MALL_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_MALL_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_MALL_SKIP_NACOS=true")
	}
}
