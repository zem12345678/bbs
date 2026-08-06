package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// yamlConfig mirrors the shape of configs/config.yaml closely enough to prove that
// applyEnvOverrides keeps sibling keys and still outranks the raw env string.
const envOverrideYAML = `
kafka:
  brokers:
    - file-a:9092
  topic: user-events
  username: kafka-user
grpc:
  server:
    port: 9102
    etcdAddr:
      - 127.0.0.1:2379
    serviceName: bbs-user-service
    timeout: 10s
`

func newEnvOverrideViper(t *testing.T) *viper.Viper {
	t.Helper()
	v := viper.New()
	configureEnv(v)
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(envOverrideYAML)); err != nil {
		t.Fatalf("read config: %v", err)
	}
	return v
}

// TestApplyEnvOverridesSplitsKafkaBrokers guards the regression where the CSV split
// landed in viper's config layer and AutomaticEnv served the raw string instead.
func TestApplyEnvOverridesSplitsKafkaBrokers(t *testing.T) {
	t.Setenv("BBS_USER_KAFKA_BROKERS", "kafka-a:9092,kafka-b:9092")
	v := newEnvOverrideViper(t)

	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("applyEnvOverrides: %v", err)
	}

	want := []string{"kafka-a:9092", "kafka-b:9092"}
	if got := v.GetStringSlice("kafka.brokers"); !reflect.DeepEqual(got, want) {
		t.Fatalf("flat kafka.brokers = %#v, want %#v", got, want)
	}

	var section struct {
		Brokers  []string `mapstructure:"brokers"`
		Topic    string   `mapstructure:"topic"`
		Username string   `mapstructure:"username"`
	}
	if err := v.UnmarshalKey("kafka", &section); err != nil {
		t.Fatalf("unmarshal kafka: %v", err)
	}
	if !reflect.DeepEqual(section.Brokers, want) {
		t.Fatalf("kafka.brokers = %#v, want %#v", section.Brokers, want)
	}
	if section.Topic != "user-events" {
		t.Fatalf("kafka.topic = %q, want %q", section.Topic, "user-events")
	}
	if section.Username != "kafka-user" {
		t.Fatalf("kafka.username = %q, want %q", section.Username, "kafka-user")
	}
}

// TestApplyEnvOverridesKeepsGRPCServerSiblings proves that overriding only the port
// leaves etcdAddr, serviceName and timeout from the config file intact.
func TestApplyEnvOverridesKeepsGRPCServerSiblings(t *testing.T) {
	t.Setenv("BBS_USER_GRPC_SERVER_PORT", "29102")
	v := newEnvOverrideViper(t)

	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("applyEnvOverrides: %v", err)
	}

	var server struct {
		Port        int      `mapstructure:"port"`
		EtcdAddr    []string `mapstructure:"etcdAddr"`
		ServiceName string   `mapstructure:"serviceName"`
		Timeout     string   `mapstructure:"timeout"`
	}
	if err := v.UnmarshalKey("grpc.server", &server); err != nil {
		t.Fatalf("unmarshal grpc.server: %v", err)
	}
	if server.Port != 29102 {
		t.Fatalf("grpc.server.port = %d, want 29102", server.Port)
	}
	if want := []string{"127.0.0.1:2379"}; !reflect.DeepEqual(server.EtcdAddr, want) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", server.EtcdAddr, want)
	}
	if server.ServiceName != "bbs-user-service" {
		t.Fatalf("grpc.server.serviceName = %q", server.ServiceName)
	}
	if server.Timeout != "10s" {
		t.Fatalf("grpc.server.timeout = %q, want %q", server.Timeout, "10s")
	}
	if got := v.GetInt("service.grpcPort"); got != 29102 {
		t.Fatalf("service.grpcPort = %d, want 29102", got)
	}
}
