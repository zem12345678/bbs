package config

import (
	"reflect"
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

func TestApplyEnvOverridesSetsRuntimeEndpoints(t *testing.T) {
	t.Setenv("BBS_USER_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092,,")
	t.Setenv("BBS_USER_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379")
	t.Setenv("BBS_USER_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")

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
	if got := v.GetInt("grpc.server.port"); got != 9102 {
		t.Fatalf("grpc server port = %d", got)
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
