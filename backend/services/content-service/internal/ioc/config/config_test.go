package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsContentUpstreams(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.comment"); got != "bbs-comment-service" {
		t.Fatalf("upstreams.comment = %q", got)
	}
	if got := v.GetString("upstreams.mall"); got != "bbs-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
	if got := v.GetString("upstreams.credit"); got != "bbs-credit-service" {
		t.Fatalf("upstreams.credit = %q", got)
	}
}

func TestConfigureEnvBindsMallUpstream(t *testing.T) {
	t.Setenv("BBS_CONTENT_UPSTREAMS_MALL", "file-mall-service")
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	if got := v.GetString("upstreams.mall"); got != "file-mall-service" {
		t.Fatalf("upstreams.mall = %q", got)
	}
}

func TestApplyEnvOverridesSetsEtcdEndpoints(t *testing.T) {
	t.Setenv("BBS_CONTENT_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379,,")
	t.Setenv("BBS_CONTENT_GRPC_CLIENT_ETCD_ADDR", "etcd-client:2379")

	v := viper.New()
	configureEnv(v)
	applyEnvOverrides(v)

	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("server etcd endpoints = %#v, want %#v", got, want)
	}
	if got, want := v.GetStringSlice("grpc.client.etcdAddr"), []string{"etcd-client:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("client etcd endpoints = %#v, want %#v", got, want)
	}
}

func TestApplyNacosEnvOverrides(t *testing.T) {
	t.Setenv("BBS_CONTENT_NACOS_ADDR", "nacos.example")
	t.Setenv("BBS_CONTENT_NACOS_DATA_ID", "content-service-local.yaml")

	v := viper.New()
	v.Set("nacos.addr", "127.0.0.1")
	v.Set("nacos.dataId", "bbs-content-service.yaml")
	applyNacosEnvOverrides(v)

	if got := v.GetString("nacos.addr"); got != "nacos.example" {
		t.Fatalf("nacos.addr = %q", got)
	}
	if got := v.GetString("nacos.dataId"); got != "content-service-local.yaml" {
		t.Fatalf("nacos.dataId = %q", got)
	}
}
