package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyEnvOverridesSetsRuntimeEndpoints(t *testing.T) {
	t.Setenv("BBS_COMMENT_MONGO_ENDPOINTS", "mongo-a:27017, mongo-b:27017,,")
	t.Setenv("BBS_COMMENT_KAFKA_BROKERS", "kafka-a:9092")
	t.Setenv("BBS_COMMENT_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379")
	t.Setenv("BBS_COMMENT_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")

	v := viper.New()
	applyEnvOverrides(v)

	if got, want := v.GetStringSlice("mongo.endpoints"), []string{"mongo-a:27017", "mongo-b:27017"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mongo endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka brokers = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
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
