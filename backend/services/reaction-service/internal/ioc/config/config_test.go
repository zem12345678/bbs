package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_REACTION_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_REACTION_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_REACTION_SKIP_NACOS=true")
	}
}

func TestApplyGRPCPortEnvOverrideReplacesNestedServerPort(t *testing.T) {
	t.Setenv("BBS_REACTION_GRPC_SERVER_PORT", "19105")
	t.Setenv("BBS_REACTION_SERVICE_GRPC_PORT", "19106")

	v := viper.New()
	if err := v.MergeConfigMap(map[string]interface{}{
		"grpc": map[string]interface{}{"server": map[string]interface{}{"port": 9105}},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := applyGRPCPortEnvOverride(v,
		"BBS_REACTION_GRPC_SERVER_PORT",
		"BBS_REACTION_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}

	if got := v.GetInt("service.grpcPort"); got != 19105 {
		t.Fatalf("service.grpcPort = %d, want 19105", got)
	}
	var server struct{ Port int }
	if err := v.UnmarshalKey("grpc.server", &server); err != nil {
		t.Fatalf("unmarshal grpc.server: %v", err)
	}
	if server.Port != 19105 {
		t.Fatalf("grpc server port = %d, want 19105", server.Port)
	}

	t.Setenv("BBS_REACTION_GRPC_SERVER_PORT", "0")
	if err := applyGRPCPortEnvOverride(v, "BBS_REACTION_GRPC_SERVER_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}

func TestApplyEnvOverridesSetsRuntimeConfig(t *testing.T) {
	t.Setenv("BBS_REACTION_POSTGRES_DSN", "postgres://reaction")
	t.Setenv("BBS_REACTION_REDIS_ADDR", "redis:6379")
	t.Setenv("BBS_REACTION_REDIS_DB", "5")
	t.Setenv("BBS_REACTION_REDIS_PASSWORD", "redis-pass")
	t.Setenv("BBS_REACTION_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092")
	t.Setenv("BBS_REACTION_KAFKA_TOPIC", "env.reaction.events")
	t.Setenv("BBS_REACTION_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_REACTION_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-reaction-internal-token")
	t.Setenv("BBS_REACTION_INTERNAL_AUTH_TOKEN", "legacy-reaction-internal-token")
	t.Setenv("BBS_REACTION_TRACE_ENV", "production")

	v := viper.New()
	applyEnvOverrides(v)

	if got := v.GetString("postgres.dsn"); got != "postgres://reaction" {
		t.Fatalf("postgres.dsn = %q", got)
	}
	if got := v.GetString("redis.url"); got != "redis:6379" {
		t.Fatalf("redis.url = %q", got)
	}
	if got := v.GetInt("redis.dbNum"); got != 5 {
		t.Fatalf("redis.dbNum = %d", got)
	}
	if got := v.GetString("redis.password"); got != "redis-pass" {
		t.Fatalf("redis.password = %q", got)
	}
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092", "kafka-b:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka.brokers = %#v, want %#v", got, want)
	}
	if got := v.GetString("kafka.topic"); got != "env.reaction.events" {
		t.Fatalf("kafka.topic = %q", got)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", got, want)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-reaction-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
}

func TestApplyEnvOverridesSupportsLegacyInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_REACTION_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "")
	t.Setenv("BBS_REACTION_INTERNAL_AUTH_TOKEN", "legacy-reaction-internal-token")

	v := viper.New()
	applyEnvOverrides(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != "legacy-reaction-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
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

	for _, environment := range []string{"production", "prod"} {
		for _, tc := range []struct {
			name  string
			token string
		}{
			{name: "empty", token: ""},
			{name: "default", token: localDevInternalAuthToken},
			{name: "short", token: "too-short"},
		} {
			t.Run(environment+"/"+tc.name, func(t *testing.T) {
				v := viper.New()
				v.Set("trace.env", environment)
				v.Set("grpc.server.internalAuthToken", tc.token)
				if err := validate(v); err == nil {
					t.Fatal("validate production config error = nil")
				}
			})
		}
	}

	production := viper.New()
	production.Set("trace.env", "production")
	production.Set("grpc.server.internalAuthToken", "production-reaction-token-with-at-least-32-bytes")
	if err := validate(production); err != nil {
		t.Fatalf("validate production config: %v", err)
	}
}
