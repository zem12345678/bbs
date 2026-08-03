package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsMallUpstream(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "bbs-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
	if got := v.GetString("upstreams.mallInternalAuthToken"); got != localDevMallInternalAuthToken {
		t.Fatalf("upstreams.mallInternalAuthToken = %q", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsMallInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_USER_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN", "configured-mall-token")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mallInternalAuthToken"); got != "configured-mall-token" {
		t.Fatalf("upstreams.mallInternalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsMallUpstream(t *testing.T) {
	t.Setenv("BBS_USER_UPSTREAMS_MALL", "file-mall-service")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "file-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
}

func TestConfigureEnvBindsInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_USER_GRPC_SERVER_INTERNAL_AUTH_TOKEN", " configured-user-token ")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != " configured-user-token " {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestConfigureEnvBindsMFASettings(t *testing.T) {
	t.Setenv("BBS_USER_MFA_ENCRYPTION_KEY", "configured-mfa-encryption-key")
	t.Setenv("BBS_USER_MFA_ISSUER", "Configured Community")
	v := viper.New()
	configureEnv(v)

	if got := v.GetString("mfa.encryptionKey"); got != "configured-mfa-encryption-key" {
		t.Fatalf("mfa.encryptionKey = %q", got)
	}
	if got := v.GetString("mfa.issuer"); got != "Configured Community" {
		t.Fatalf("mfa.issuer = %q", got)
	}
}

func TestConfigureEnvBindsStatefulSetSnowflakeSettings(t *testing.T) {
	t.Setenv("BBS_USER_SNOWFLAKE_INSTANCE_NAME", "bbs-user-service-7")
	t.Setenv("BBS_USER_SNOWFLAKE_WORKER_ID_RANGE_START", "64")
	t.Setenv("BBS_USER_SNOWFLAKE_WORKER_ID_RANGE_SIZE", "192")
	v := viper.New()
	configureEnv(v)

	if got := v.GetString("snowflake.instanceName"); got != "bbs-user-service-7" {
		t.Fatalf("snowflake.instanceName = %q", got)
	}
	if got := v.GetInt64("snowflake.workerIdRangeStart"); got != 64 {
		t.Fatalf("snowflake.workerIdRangeStart = %d", got)
	}
	if got := v.GetInt64("snowflake.workerIdRangeSize"); got != 192 {
		t.Fatalf("snowflake.workerIdRangeSize = %d", got)
	}
}

func TestValidateRejectsDefaultInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "internalAuthToken") {
		t.Fatalf("validate error = %v, want production internal auth token error", err)
	}
}

func TestValidateRejectsShortInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "prod")
	v.Set("grpc.server.internalAuthToken", "too-short")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("validate error = %v, want short internal auth token error", err)
	}
}

func TestValidateAcceptsConfiguredInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("grpc.server.internalAuthToken", "production-user-internal-token-with-32-bytes")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("mfa.encryptionKey", "production-mfa-encryption-key-with-32-bytes")

	if err := validate(v); err != nil {
		t.Fatalf("validate configured internal auth token: %v", err)
	}
}

func TestValidateRejectsShortMFAEncryptionKeyInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("grpc.server.internalAuthToken", "production-user-internal-token-with-32-bytes")
	v.Set("upstreams.mallInternalAuthToken", "production-mall-internal-token-with-32-bytes")
	v.Set("mfa.encryptionKey", "too-short")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "mfa.encryptionKey") {
		t.Fatalf("validate error = %v, want production MFA encryption key error", err)
	}
}

func TestValidateRejectsDefaultMallInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("grpc.server.internalAuthToken", "production-user-internal-token-with-32-bytes")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "mallInternalAuthToken") {
		t.Fatalf("validate error = %v, want production mall internal auth token error", err)
	}
}

func TestApplyEnvOverridesSetsRuntimeEndpoints(t *testing.T) {
	t.Setenv("BBS_USER_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092,,")
	t.Setenv("BBS_USER_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379")
	t.Setenv("BBS_USER_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")
	t.Setenv("BBS_USER_GRPC_SERVER_PORT", "19102")

	v := viper.New()
	if err := v.MergeConfigMap(map[string]interface{}{
		"kafka": map[string]interface{}{"topic": "user.events"},
		"grpc": map[string]interface{}{
			"server": map[string]interface{}{"port": 9102},
			"client": map[string]interface{}{"timeout": "10s"},
		},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("apply environment overrides: %v", err)
	}

	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092", "kafka-b:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka brokers = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
	if got := v.GetString("kafka.topic"); got != "user.events" {
		t.Fatalf("kafka topic = %q", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19102 {
		t.Fatalf("grpc server port = %d", got)
	}
	if got := v.GetInt("service.grpcPort"); got != 19102 {
		t.Fatalf("service grpc port = %d", got)
	}
	var grpcServer struct {
		Port int
	}
	if err := v.UnmarshalKey("grpc.server", &grpcServer); err != nil {
		t.Fatalf("unmarshal grpc server: %v", err)
	}
	if grpcServer.Port != 19102 {
		t.Fatalf("unmarshaled grpc server port = %d", grpcServer.Port)
	}
}

func TestApplyEnvOverridesRejectsInvalidGRPCPort(t *testing.T) {
	t.Setenv("BBS_USER_GRPC_SERVER_PORT", "not-a-port")
	if err := applyEnvOverrides(viper.New()); err == nil {
		t.Fatal("applyEnvOverrides() error = nil, want invalid port error")
	}
}

func TestSkipNacosRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("BBS_USER_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without an environment override")
	}
	t.Setenv("BBS_USER_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_USER_SKIP_NACOS=true")
	}
}
