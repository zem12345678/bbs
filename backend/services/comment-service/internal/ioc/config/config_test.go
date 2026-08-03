package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyEnvOverridesSetsRuntimeEndpoints(t *testing.T) {
	t.Setenv("BBS_COMMENT_MONGO_URI", "mongodb://env-mongo:27017/?authSource=admin")
	t.Setenv("BBS_COMMENT_MONGO_ENDPOINTS", "mongo-a:27017, mongo-b:27017,,")
	t.Setenv("BBS_COMMENT_MONGO_DATABASE", "env_comment")
	t.Setenv("BBS_COMMENT_MONGO_USERNAME", "env-user")
	t.Setenv("BBS_COMMENT_MONGO_PASSWORD", "env-pass")
	t.Setenv("BBS_COMMENT_MONGO_AUTH_DB", "env-auth")
	t.Setenv("BBS_COMMENT_MONGO_ENABLE_TRACE", "true")
	t.Setenv("BBS_COMMENT_KAFKA_BROKERS", "kafka-a:9092")
	t.Setenv("BBS_COMMENT_KAFKA_TOPIC", "env.comment.events")
	t.Setenv("BBS_COMMENT_KAFKA_USERNAME", "env-kafka-user")
	t.Setenv("BBS_COMMENT_KAFKA_PASSWORD", "env-kafka-pass")
	t.Setenv("BBS_COMMENT_KAFKA_SCRAM_ALGORITHM", "SHA256")
	t.Setenv("BBS_COMMENT_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379")
	t.Setenv("BBS_COMMENT_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")
	t.Setenv("BBS_COMMENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-comment-internal-token")
	t.Setenv("BBS_COMMENT_INTERNAL_AUTH_TOKEN", "legacy-comment-internal-token")
	t.Setenv("BBS_COMMENT_TRACE_ENV", "production")

	v := viper.New()
	if err := v.MergeConfigMap(map[string]interface{}{
		"mongo": map[string]interface{}{
			"username": "admin",
			"database": "bbs_comment",
		},
		"kafka": map[string]interface{}{"topic": "comment.events"},
		"grpc": map[string]interface{}{
			"server": map[string]interface{}{"port": 9104},
			"client": map[string]interface{}{"timeout": "10s"},
		},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("apply environment overrides: %v", err)
	}

	if got, want := v.GetStringSlice("mongo.endpoints"), []string{"mongo-a:27017", "mongo-b:27017"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mongo endpoints = %#v, want %#v", got, want)
	}
	if got := v.GetString("mongo.uri"); got != "mongodb://env-mongo:27017/?authSource=admin" {
		t.Fatalf("mongo uri = %q", got)
	}
	if got := v.GetString("mongo.database"); got != "env_comment" {
		t.Fatalf("mongo database = %q", got)
	}
	if got := v.GetString("mongo.username"); got != "env-user" {
		t.Fatalf("mongo username = %q", got)
	}
	if got := v.GetString("mongo.password"); got != "env-pass" {
		t.Fatalf("mongo password = %q", got)
	}
	if got := v.GetString("mongo.authDB"); got != "env-auth" {
		t.Fatalf("mongo authDB = %q", got)
	}
	if !v.GetBool("mongo.enableTrace") {
		t.Fatal("mongo enableTrace = false, want true")
	}
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka brokers = %#v, want %#v", got, want)
	}
	if got := v.GetString("kafka.topic"); got != "env.comment.events" {
		t.Fatalf("kafka topic = %q", got)
	}
	if got := v.GetString("kafka.username"); got != "env-kafka-user" {
		t.Fatalf("kafka username = %q", got)
	}
	if got := v.GetString("kafka.password"); got != "env-kafka-pass" {
		t.Fatalf("kafka password = %q", got)
	}
	if got := v.GetString("kafka.scram_algorithm"); got != "SHA256" {
		t.Fatalf("kafka scram algorithm = %q", got)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-comment-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 9104 {
		t.Fatalf("grpc server port = %d", got)
	}
}

func TestSkipNacosRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("BBS_COMMENT_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without an environment override")
	}
	t.Setenv("BBS_COMMENT_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_COMMENT_SKIP_NACOS=true")
	}
}

func TestApplyEnvOverridesReplacesNestedGRPCServerPort(t *testing.T) {
	t.Setenv("BBS_COMMENT_GRPC_SERVER_PORT", "19104")
	t.Setenv("BBS_COMMENT_SERVICE_GRPC_PORT", "19105")

	v := viper.New()
	if err := v.MergeConfigMap(map[string]interface{}{
		"grpc": map[string]interface{}{"server": map[string]interface{}{"port": 9104}},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("apply environment overrides: %v", err)
	}

	if got := v.GetInt("service.grpcPort"); got != 19104 {
		t.Fatalf("service.grpcPort = %d, want 19104", got)
	}
	var server struct{ Port int }
	if err := v.UnmarshalKey("grpc.server", &server); err != nil {
		t.Fatalf("unmarshal grpc.server: %v", err)
	}
	if server.Port != 19104 {
		t.Fatalf("grpc server port = %d, want 19104", server.Port)
	}

	t.Setenv("BBS_COMMENT_GRPC_SERVER_PORT", "invalid")
	if err := applyEnvOverrides(v); err == nil {
		t.Fatal("applyEnvOverrides() accepted an invalid gRPC port")
	}
}

func TestApplyEnvOverridesSupportsLegacyInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_COMMENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "")
	t.Setenv("BBS_COMMENT_INTERNAL_AUTH_TOKEN", "legacy-comment-internal-token")

	v := viper.New()
	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("apply environment overrides: %v", err)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "legacy-comment-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestApplyEnvOverridesSetsStatefulSetSnowflakeSettings(t *testing.T) {
	t.Setenv("BBS_COMMENT_SNOWFLAKE_INSTANCE_NAME", "bbs-comment-service-7")
	t.Setenv("BBS_COMMENT_SNOWFLAKE_WORKER_ID_RANGE_START", "448")
	t.Setenv("BBS_COMMENT_SNOWFLAKE_WORKER_ID_RANGE_SIZE", "192")
	v := viper.New()
	configureEnv(v)
	if err := applyEnvOverrides(v); err != nil {
		t.Fatal(err)
	}

	if got := v.GetString("snowflake.instanceName"); got != "bbs-comment-service-7" {
		t.Fatalf("snowflake.instanceName = %q", got)
	}
	if got := v.GetInt64("snowflake.workerIdRangeStart"); got != 448 {
		t.Fatalf("snowflake.workerIdRangeStart = %d", got)
	}
	if got := v.GetInt64("snowflake.workerIdRangeSize"); got != 192 {
		t.Fatalf("snowflake.workerIdRangeSize = %d", got)
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
	production.Set("grpc.server.internalAuthToken", "production-comment-token-with-at-least-32-bytes")
	if err := validate(production); err != nil {
		t.Fatalf("validate production config: %v", err)
	}
}
