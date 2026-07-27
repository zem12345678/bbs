package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsProvidesRuntimeConfig(t *testing.T) {
	v := viper.New()

	setDefaults(v)

	if got := v.GetString("service.name"); got != "bbs-credit-service" {
		t.Fatalf("service.name = %q, want bbs-credit-service", got)
	}
	if got := v.GetInt("service.grpcPort"); got != 9107 {
		t.Fatalf("service.grpcPort = %d, want 9107", got)
	}
	if got := v.GetString("postgres.dsn"); got == "" {
		t.Fatal("postgres.dsn should default to a local DSN")
	}
	if got := v.GetStringSlice("kafka.brokers"); !reflect.DeepEqual(got, []string{"127.0.0.1:9092"}) {
		t.Fatalf("kafka.brokers = %#v, want local broker", got)
	}
	if got := v.GetString("kafka.articleTopic"); got != "article.events" {
		t.Fatalf("kafka.articleTopic = %q, want article.events", got)
	}
	if got := v.GetString("kafka.reactionGroupId"); got != "bbs-credit-reaction-consumer" {
		t.Fatalf("kafka.reactionGroupId = %q, want bbs-credit-reaction-consumer", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 9107 {
		t.Fatalf("grpc.server.port = %d, want 9107", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("grpc.server.internalAuthToken = %q, want local development token", got)
	}
	if got := v.GetString("trace.serviceName"); got != "bbs-credit-service" {
		t.Fatalf("trace.serviceName = %q, want bbs-credit-service", got)
	}
}

func TestConfigureEnvBindsPostgresDSN(t *testing.T) {
	t.Setenv("BBS_CREDIT_POSTGRES_DSN", "postgres://credit.example/bbs?sslmode=disable")
	v := viper.New()

	configureEnv(v)
	setDefaults(v)

	if got := v.GetString("postgres.dsn"); got != "postgres://credit.example/bbs?sslmode=disable" {
		t.Fatalf("postgres.dsn = %q, want env override", got)
	}
}

func TestConfigureEnvBindsInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_CREDIT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", " configured-credit-token ")
	v := viper.New()

	configureEnv(v)
	setDefaults(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != " configured-credit-token " {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
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
	v.Set("grpc.server.internalAuthToken", "production-credit-internal-token-with-32-bytes")

	if err := validate(v); err != nil {
		t.Fatalf("validate configured internal auth token: %v", err)
	}
}

func TestApplyEnvOverridesSplitsKafkaBrokers(t *testing.T) {
	t.Setenv("BBS_CREDIT_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092,,")
	t.Setenv("BBS_CREDIT_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_CREDIT_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")
	v := viper.New()

	configureEnv(v)
	applyEnvOverrides(v)
	setDefaults(v)

	want := []string{"kafka-a:9092", "kafka-b:9092"}
	if got := v.GetStringSlice("kafka.brokers"); !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka.brokers = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
}

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_CREDIT_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_CREDIT_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_CREDIT_SKIP_NACOS=true")
	}
}

func TestApplyGRPCPortEnvOverrideReplacesNestedServerPort(t *testing.T) {
	t.Setenv("BBS_CREDIT_GRPC_SERVER_PORT", "19107")
	t.Setenv("BBS_CREDIT_SERVICE_GRPC_PORT", "19108")

	v := viper.New()
	if err := v.MergeConfigMap(map[string]interface{}{
		"grpc": map[string]interface{}{"server": map[string]interface{}{"port": 9107}},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := applyGRPCPortEnvOverride(v,
		"BBS_CREDIT_GRPC_SERVER_PORT",
		"BBS_CREDIT_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}

	if got := v.GetInt("service.grpcPort"); got != 19107 {
		t.Fatalf("service.grpcPort = %d, want 19107", got)
	}
	var server struct{ Port int }
	if err := v.UnmarshalKey("grpc.server", &server); err != nil {
		t.Fatalf("unmarshal grpc.server: %v", err)
	}
	if server.Port != 19107 {
		t.Fatalf("grpc server port = %d, want 19107", server.Port)
	}

	t.Setenv("BBS_CREDIT_GRPC_SERVER_PORT", "70000")
	if err := applyGRPCPortEnvOverride(v, "BBS_CREDIT_GRPC_SERVER_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}
