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
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka brokers = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
	if got := v.GetString("mongo.username"); got != "admin" {
		t.Fatalf("mongo username = %q", got)
	}
	if got := v.GetString("mongo.database"); got != "bbs_comment" {
		t.Fatalf("mongo database = %q", got)
	}
	if got := v.GetString("kafka.topic"); got != "comment.events" {
		t.Fatalf("kafka topic = %q", got)
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
