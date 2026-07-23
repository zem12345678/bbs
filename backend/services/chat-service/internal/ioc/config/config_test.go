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
