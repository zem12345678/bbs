package config

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestSetDefaultsBuildsChatProviderConfig(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	if got := v.GetString("service.name"); got != "bbs-chat-service" {
		t.Fatalf("service.name = %q, want bbs-chat-service", got)
	}
	if got := v.GetInt("service.grpcPort"); got != 9116 {
		t.Fatalf("service.grpcPort = %d, want 9116", got)
	}
	if got := v.GetString("redis.url"); got != "127.0.0.1:6379" {
		t.Fatalf("redis.url = %q, want local redis", got)
	}
	if got := v.GetInt("redis.dbNum"); got != 0 {
		t.Fatalf("redis.dbNum = %d, want 0", got)
	}
	if got := v.GetStringSlice("kafka.producerOptions.brokers"); !reflect.DeepEqual(got, []string{"127.0.0.1:9092"}) {
		t.Fatalf("producer brokers = %#v", got)
	}
	if got := v.GetString("kafka.consumerOptions.groupId"); got != "bbs-chat-realtime" {
		t.Fatalf("consumer group = %q", got)
	}
	if got := v.GetDuration("outbox.leaseDuration"); got != time.Minute {
		t.Fatalf("outbox lease = %s, want 1m", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("internal auth token = %q, want local development token", got)
	}
	if got := v.GetDuration("grpc.server.rateLimit.interval"); got != time.Second {
		t.Fatalf("grpc rate limit interval = %s, want 1s", got)
	}
	if got := v.GetInt("grpc.server.rateLimit.rate"); got != 1000 {
		t.Fatalf("grpc rate limit rate = %d, want 1000", got)
	}
	if err := validate(v); err != nil {
		t.Fatalf("defaults should validate: %v", err)
	}
}

func TestApplyEnvOverridesSupportsCSVAndAliases(t *testing.T) {
	t.Setenv("BBS_CHAT_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092,,")
	t.Setenv("BBS_CHAT_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_CHAT_REDIS_ADDR", "redis.example:6380")
	t.Setenv("BBS_CHAT_REDIS_DB", "4")
	t.Setenv("BBS_CHAT_KAFKA_REALTIME_GROUP_ID", "chat-realtime-v2")
	t.Setenv("BBS_CHAT_KAFKA_PRODUCER_USERNAME", "producer-user")
	t.Setenv("BBS_CHAT_KAFKA_PRODUCER_PASSWORD", "producer-pass")
	t.Setenv("BBS_CHAT_KAFKA_PRODUCER_SCRAM_ALGORITHM", "SHA256")
	t.Setenv("BBS_CHAT_INTERNAL_AUTH_TOKEN", "env-internal-token")
	t.Setenv("BBS_CHAT_GRPC_SERVER_TLS_ENABLED", "true")
	t.Setenv("BBS_CHAT_GRPC_SERVER_RATE_LIMIT_INTERVAL", "2s")
	t.Setenv("BBS_CHAT_GRPC_SERVER_RATE_LIMIT_RATE", "42")

	v := viper.New()
	configureEnv(v)
	if err := applyEnvOverrides(v); err != nil {
		t.Fatal(err)
	}
	setDefaults(v)

	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092", "kafka-b:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka brokers = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("etcd addresses = %#v, want %#v", got, want)
	}
	if got := v.GetString("redis.url"); got != "redis.example:6380" {
		t.Fatalf("redis.url = %q", got)
	}
	if got := v.GetInt("redis.dbNum"); got != 4 {
		t.Fatalf("redis.dbNum = %d", got)
	}
	if got := v.GetString("kafka.consumerOptions.groupId"); got != "chat-realtime-v2" {
		t.Fatalf("consumer group = %q", got)
	}
	if got := v.GetString("kafka.producerOptions.scram_algorithm"); got != "SHA256" {
		t.Fatalf("producer SASL algorithm = %q", got)
	}
	if got := v.GetString("kafka.consumerOptions.username"); got != "" {
		t.Fatalf("consumer username unexpectedly inherited producer credentials: %q", got)
	}
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-internal-token" {
		t.Fatalf("internal auth token = %q", got)
	}
	if got := v.GetBool("grpc.server.tls.enabled"); !got {
		t.Fatalf("grpc.server.tls.enabled = %t", got)
	}
	if got := v.GetDuration("grpc.server.rateLimit.interval"); got != 2*time.Second {
		t.Fatalf("grpc rate limit interval = %s", got)
	}
	if got := v.GetInt("grpc.server.rateLimit.rate"); got != 42 {
		t.Fatalf("grpc rate limit rate = %d", got)
	}
}

func TestValidateRejectsUnsafeConfiguration(t *testing.T) {
	tests := []struct {
		name string
		edit func(*viper.Viper)
		want string
	}{
		{
			name: "postgres pool",
			edit: func(v *viper.Viper) { v.Set("postgres.max_open_conns", 0) },
			want: "max_open_conns",
		},
		{
			name: "sasl pair",
			edit: func(v *viper.Viper) { v.Set("kafka.username", "only-user") },
			want: "username and password",
		},
		{
			name: "producer topic",
			edit: func(v *viper.Viper) { v.Set("kafka.producerOptions.topic", "other.events") },
			want: "producer topic",
		},
		{
			name: "consumer topics",
			edit: func(v *viper.Viper) { v.Set("kafka.consumerOptions.topics", []string{"chat.events", "other.events"}) },
			want: "consumer topics",
		},
		{
			name: "consumer group",
			edit: func(v *viper.Viper) { v.Set("kafka.consumerOptions.groupId", "other-group") },
			want: "consumer group",
		},
		{
			name: "unsupported grpc TLS",
			edit: func(v *viper.Viper) { v.Set("grpc.client.secure", true) },
			want: "TLS",
		},
		{
			name: "grpc rate limit",
			edit: func(v *viper.Viper) { v.Set("grpc.server.rateLimit.rate", 0) },
			want: "rate limit",
		},
		{
			name: "snowflake range",
			edit: func(v *viper.Viper) { v.Set("snowflake.workerId", 1024) },
			want: "worker id",
		},
		{
			name: "outbox lease",
			edit: func(v *viper.Viper) { v.Set("outbox.leaseDuration", 40*time.Second) },
			want: "lease duration",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := viper.New()
			setDefaults(v)
			test.edit(v)
			err := validate(v)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateRequiresExplicitSnowflakeWorkerIDInProduction(t *testing.T) {
	t.Setenv("BBS_CHAT_SNOWFLAKE_WORKER_ID", "")
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "BBS_CHAT_SNOWFLAKE_WORKER_ID") {
		t.Fatalf("validate error = %v, want explicit production worker ID error", err)
	}
}

func TestValidateAcceptsExplicitSnowflakeWorkerIDInProduction(t *testing.T) {
	t.Setenv("BBS_CHAT_SNOWFLAKE_WORKER_ID", "17")
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "prod")
	v.Set("snowflake.workerId", 17)
	v.Set("grpc.server.internalAuthToken", "production-chat-internal-token-with-32-bytes")
	setProductionServerTLS(v)

	if err := validate(v); err != nil {
		t.Fatalf("explicit production worker ID should validate: %v", err)
	}
}

func TestValidateRejectsPlaintextGRPCServerInProduction(t *testing.T) {
	t.Setenv("BBS_CHAT_SNOWFLAKE_WORKER_ID", "17")
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("snowflake.workerId", 17)
	v.Set("grpc.server.internalAuthToken", "production-chat-internal-token-with-32-bytes")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "tls.enabled") {
		t.Fatalf("validate error = %v, want production TLS error", err)
	}
}

func TestValidateRejectsDefaultInternalAuthTokenInProduction(t *testing.T) {
	t.Setenv("BBS_CHAT_SNOWFLAKE_WORKER_ID", "17")
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("snowflake.workerId", 17)

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "internalAuthToken") {
		t.Fatalf("validate error = %v, want production internal auth token error", err)
	}
}

func TestValidateRejectsShortInternalAuthTokenInProduction(t *testing.T) {
	t.Setenv("BBS_CHAT_SNOWFLAKE_WORKER_ID", "17")
	v := viper.New()
	setDefaults(v)
	v.Set("trace.env", "production")
	v.Set("snowflake.workerId", 17)
	v.Set("grpc.server.internalAuthToken", "too-short")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("validate error = %v, want short internal auth token error", err)
	}
}

func setProductionServerTLS(v *viper.Viper) {
	v.Set("grpc.server.tls.enabled", true)
	v.Set("grpc.server.tls.certFile", "/var/run/secrets/bbs/tls.crt")
	v.Set("grpc.server.tls.keyFile", "/var/run/secrets/bbs/tls.key")
	v.Set("grpc.server.tls.clientCAFile", "/var/run/secrets/bbs/ca.crt")
}

func TestNewLoadsLocalConfigWhenNacosIsSkipped(t *testing.T) {
	t.Setenv("BBS_CHAT_SKIP_NACOS", "true")
	v, err := New("../../../configs/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("service.name"); got != "bbs-chat-service" {
		t.Fatalf("service.name = %q", got)
	}
	if got := v.GetString("server.uuid"); got == "" {
		t.Fatal("server.uuid should be populated")
	}
}

func TestNewRejectsInvalidNacosPortOverride(t *testing.T) {
	t.Setenv("BBS_CHAT_SKIP_NACOS", "true")
	t.Setenv("BBS_CHAT_NACOS_PORT", "invalid")

	_, err := New("../../../configs/config.yaml")
	if err == nil || !strings.Contains(err.Error(), "BBS_CHAT_NACOS_PORT") {
		t.Fatalf("New() error = %v, want invalid Nacos port", err)
	}
}

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_CHAT_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_CHAT_SKIP_NACOS", "yes")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_CHAT_SKIP_NACOS=yes")
	}
}

func TestApplyNacosEnvOverridesRejectsInvalidPort(t *testing.T) {
	tests := []string{"0", "invalid", "65536"}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Setenv("BBS_CHAT_NACOS_PORT", value)
			if err := applyNacosEnvOverrides(viper.New()); err == nil || !strings.Contains(err.Error(), "BBS_CHAT_NACOS_PORT") {
				t.Fatalf("applyNacosEnvOverrides() error = %v, want invalid Nacos port", err)
			}
		})
	}
}

func TestApplyNacosEnvOverridesAcceptsValidPort(t *testing.T) {
	t.Setenv("BBS_CHAT_NACOS_PORT", "18848")
	v := viper.New()
	if err := applyNacosEnvOverrides(v); err != nil {
		t.Fatal(err)
	}
	if got := v.GetUint64("nacos.port"); got != 18848 {
		t.Fatalf("nacos.port = %d, want 18848", got)
	}
}

func TestApplyEnvOverridesRejectsInvalidGRPCServerTLSEnabled(t *testing.T) {
	t.Setenv("BBS_CHAT_GRPC_SERVER_TLS_ENABLED", "maybe")
	v := viper.New()
	configureEnv(v)

	err := applyEnvOverrides(v)
	if err == nil || !strings.Contains(err.Error(), "BBS_CHAT_GRPC_SERVER_TLS_ENABLED") {
		t.Fatalf("applyEnvOverrides() error = %v, want invalid grpc server tls env", err)
	}
}
