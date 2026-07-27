package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_NOTIFICATION_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_NOTIFICATION_SKIP_NACOS=true")
	}
}

func TestApplyGRPCPortEnvOverride(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_PORT", "19108")

	v := viper.New()
	if err := applyGRPCPortEnvOverride(v,
		"BBS_NOTIFICATION_GRPC_SERVER_PORT",
		"BBS_NOTIFICATION_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}
	if got := v.GetInt("service.grpcPort"); got != 19108 {
		t.Fatalf("service.grpcPort = %d, want 19108", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19108 {
		t.Fatalf("grpc.server.port = %d, want 19108", got)
	}

	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_PORT", "70000")
	if err := applyGRPCPortEnvOverride(v, "BBS_NOTIFICATION_GRPC_SERVER_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}

func TestApplyEnvOverridesSetsRuntimeConfig(t *testing.T) {
	t.Setenv("BBS_NOTIFICATION_POSTGRES_DSN", "postgres://notification")
	t.Setenv("BBS_NOTIFICATION_POSTGRES_DEBUG", "true")
	t.Setenv("BBS_NOTIFICATION_POSTGRES_MAX_OPEN_CONNS", "8")
	t.Setenv("BBS_NOTIFICATION_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092")
	t.Setenv("BBS_NOTIFICATION_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_USER_TOPIC", "env.user.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_MALL_TOPIC", "env.mall.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_USER_GROUP_ID", "env-user-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_MALL_GROUP_ID", "env-mall-group")
	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_NOTIFICATION_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-notification-internal-token")
	t.Setenv("BBS_NOTIFICATION_TRACE_ENV", "production")

	v := viper.New()
	applyEnvOverrides(v)

	if got := v.GetString("postgres.dsn"); got != "postgres://notification" {
		t.Fatalf("postgres.dsn = %q", got)
	}
	if !v.GetBool("postgres.debug") {
		t.Fatal("postgres.debug = false, want true")
	}
	if got := v.GetInt("postgres.max_open_conns"); got != 8 {
		t.Fatalf("postgres.max_open_conns = %d", got)
	}
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092", "kafka-b:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka.brokers = %#v, want %#v", got, want)
	}
	if got := v.GetString("kafka.userTopic"); got != "env.user.events" {
		t.Fatalf("kafka.userTopic = %q", got)
	}
	if got := v.GetString("kafka.mallGroupId"); got != "env-mall-group" {
		t.Fatalf("kafka.mallGroupId = %q", got)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", got, want)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-notification-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
}

func TestInternalAuthDefaultAndProductionValidation(t *testing.T) {
	local := viper.New()
	local.Set("trace.env", "local")
	setInternalAuthDefault(local)
	if got := local.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("local internal auth token = %q", got)
	}
	if err := validate(local); err != nil {
		t.Fatalf("validate local config: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "default", token: localDevInternalAuthToken},
		{name: "short", token: "too-short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			v.Set("trace.env", "production")
			v.Set("grpc.server.internalAuthToken", tc.token)
			if err := validate(v); err == nil {
				t.Fatal("validate production config error = nil")
			}
		})
	}

	production := viper.New()
	production.Set("trace.env", "prod")
	production.Set("grpc.server.internalAuthToken", "production-notification-token-with-at-least-32-bytes")
	if err := validate(production); err != nil {
		t.Fatalf("validate production config: %v", err)
	}
}
