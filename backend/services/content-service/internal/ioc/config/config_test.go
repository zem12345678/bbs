package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsContentUpstreams(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.comment"); got != "bbs-comment-service" {
		t.Fatalf("upstreams.comment = %q", got)
	}
	if got := v.GetString("upstreams.commentInternalAuthToken"); got != localDevCommentInternalAuthToken {
		t.Fatalf("upstreams.commentInternalAuthToken = %q", got)
	}
	if got := v.GetString("upstreams.mall"); got != "bbs-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
	if got := v.GetString("upstreams.mallInternalAuthToken"); got != localDevMallInternalAuthToken {
		t.Fatalf("upstreams.mallInternalAuthToken = %q", got)
	}
	if got := v.GetString("upstreams.credit"); got != "bbs-credit-service" {
		t.Fatalf("upstreams.credit = %q", got)
	}
	if got := v.GetString("upstreams.creditInternalAuthToken"); got != localDevCreditInternalAuthToken {
		t.Fatalf("upstreams.creditInternalAuthToken = %q", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetString("outbox.owner"); got != "bbs-content-service" {
		t.Fatalf("outbox.owner = %q", got)
	}
	if got := v.GetInt("outbox.batchSize"); got != 20 {
		t.Fatalf("outbox.batchSize = %d", got)
	}
	if got := v.GetDuration("outbox.leaseDuration"); got != 30*time.Second {
		t.Fatalf("outbox.leaseDuration = %s", got)
	}
}

func TestConfigureEnvBindsCommentInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_CONTENT_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN", "configured-comment-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.commentInternalAuthToken"); got != "configured-comment-token" {
		t.Fatalf("upstreams.commentInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsMallInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_CONTENT_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN", "configured-mall-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mallInternalAuthToken"); got != "configured-mall-token" {
		t.Fatalf("upstreams.mallInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsCreditInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_CONTENT_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN", "configured-credit-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.creditInternalAuthToken"); got != "configured-credit-token" {
		t.Fatalf("upstreams.creditInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsTraceEnv(t *testing.T) {
	t.Setenv("BBS_CONTENT_TRACE_ENV", "production")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
}

func TestConfigureEnvBindsStatefulSetSnowflakeSettings(t *testing.T) {
	t.Setenv("BBS_CONTENT_SNOWFLAKE_INSTANCE_NAME", "bbs-content-service-7")
	t.Setenv("BBS_CONTENT_SNOWFLAKE_WORKER_ID_RANGE_START", "256")
	t.Setenv("BBS_CONTENT_SNOWFLAKE_WORKER_ID_RANGE_SIZE", "192")
	v := viper.New()
	configureEnv(v)

	if got := v.GetString("snowflake.instanceName"); got != "bbs-content-service-7" {
		t.Fatalf("snowflake.instanceName = %q", got)
	}
	if got := v.GetInt64("snowflake.workerIdRangeStart"); got != 256 {
		t.Fatalf("snowflake.workerIdRangeStart = %d", got)
	}
	if got := v.GetInt64("snowflake.workerIdRangeSize"); got != 192 {
		t.Fatalf("snowflake.workerIdRangeSize = %d", got)
	}
}

func TestValidateRejectsDefaultMallInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default mall token rejection")
	}
}

func TestValidateAcceptsConfiguredMallInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "prod")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "production-comment-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", "production-content-internal-token-with-32-bytes")

	if err := validate(v); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestValidateRejectsDefaultCreditInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default credit token rejection")
	}
}

func TestValidateRejectsShortCreditInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "too-short")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want short credit token rejection")
	}
}

func TestValidateRejectsDefaultCommentInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default comment token rejection")
	}
}

func TestValidateRejectsMissingCommentInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want missing comment token rejection")
	}
}

func TestValidateRejectsShortCommentInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "too-short")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want short comment token rejection")
	}
}

func TestValidateRejectsDefaultServerInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "production-comment-internal-token-with-32-bytes")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want default server token rejection")
	}
}

func TestValidateRejectsMissingServerInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "production-comment-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", " ")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want missing server token rejection")
	}
}

func TestValidateRejectsShortServerInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("upstreams.creditInternalAuthToken", "production-credit-internal-token-with-32-bytes")
	v.Set("upstreams.commentInternalAuthToken", "production-comment-internal-token-with-32-bytes")
	v.Set("grpc.server.internalAuthToken", "too-short")

	if err := validate(v); err == nil {
		t.Fatal("validate() error = nil, want short server token rejection")
	}
}

func TestConfigureEnvBindsMallUpstream(t *testing.T) {
	t.Setenv("BBS_CONTENT_UPSTREAMS_MALL", "file-mall-service")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "file-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
}

func TestApplyEnvOverridesSetsEtcdEndpoints(t *testing.T) {
	t.Setenv("BBS_CONTENT_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379,,")
	t.Setenv("BBS_CONTENT_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")
	t.Setenv("BBS_CONTENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-content-internal-token")
	t.Setenv("BBS_CONTENT_INTERNAL_AUTH_TOKEN", "legacy-content-internal-token")

	v := viper.New()
	configureEnv(v)
	applyEnvOverrides(v)

	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-content-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestApplyEnvOverridesSupportsLegacyInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_CONTENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "")
	t.Setenv("BBS_CONTENT_INTERNAL_AUTH_TOKEN", "legacy-content-internal-token")

	v := viper.New()
	applyEnvOverrides(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != "legacy-content-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestApplyNacosEnvOverrides(t *testing.T) {
	t.Setenv("BBS_CONTENT_NACOS_ADDR", "nacos.example")
	t.Setenv("BBS_CONTENT_NACOS_DATA_ID", "content-service-local.yaml")

	v := viper.New()
	v.Set("nacos.addr", "127.0.0.1")
	v.Set("nacos.dataId", "bbs-content-service.yaml")
	applyNacosEnvOverrides(v)

	if got := v.GetString("nacos.addr"); got != "nacos.example" {
		t.Fatalf("nacos.addr = %q", got)
	}
	if got := v.GetString("nacos.dataId"); got != "content-service-local.yaml" {
		t.Fatalf("nacos.dataId = %q", got)
	}
}

func TestSkipNacosRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("BBS_CONTENT_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without an environment override")
	}
	t.Setenv("BBS_CONTENT_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_CONTENT_SKIP_NACOS=true")
	}
}

func TestApplyGRPCPortEnvOverrideReplacesNestedServerPort(t *testing.T) {
	t.Setenv("BBS_CONTENT_GRPC_SERVER_PORT", "19103")
	t.Setenv("BBS_CONTENT_SERVICE_GRPC_PORT", "19104")

	v := viper.New()
	if err := v.MergeConfigMap(map[string]interface{}{
		"grpc": map[string]interface{}{"server": map[string]interface{}{"port": 9103}},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := applyGRPCPortEnvOverride(v,
		"BBS_CONTENT_GRPC_SERVER_PORT",
		"BBS_CONTENT_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}

	if got := v.GetInt("service.grpcPort"); got != 19103 {
		t.Fatalf("service.grpcPort = %d, want 19103", got)
	}
	var server struct{ Port int }
	if err := v.UnmarshalKey("grpc.server", &server); err != nil {
		t.Fatalf("unmarshal grpc.server: %v", err)
	}
	if server.Port != 19103 {
		t.Fatalf("grpc server port = %d, want 19103", server.Port)
	}

	t.Setenv("BBS_CONTENT_GRPC_SERVER_PORT", "invalid")
	if err := applyGRPCPortEnvOverride(v, "BBS_CONTENT_GRPC_SERVER_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}
