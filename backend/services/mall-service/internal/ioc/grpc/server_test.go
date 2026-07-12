package grpc

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestNormalizeServerOptionsDefaultsDiscoveryAndTimeout(t *testing.T) {
	v := viper.New()
	v.Set("service.name", "mall-service")
	v.Set("service.grpcPort", 19115)
	v.Set("grpc.client.timeout", 7*time.Second)
	v.Set("grpc.client.etcdAddr", []string{"127.0.0.1:2379"})

	var options ServerOptions
	normalizeServerOptions(&options, v, "bbs-mall-service", "mall-service")

	if options.ServiceName != "bbs-mall-service" {
		t.Fatalf("ServiceName = %q, want bbs-mall-service", options.ServiceName)
	}
	if options.Port != 19115 {
		t.Fatalf("Port = %d, want 19115", options.Port)
	}
	if !reflect.DeepEqual(options.EtcdAddr, []string{"127.0.0.1:2379"}) {
		t.Fatalf("EtcdAddr = %v, want [127.0.0.1:2379]", options.EtcdAddr)
	}
	if options.Timeout != 7*time.Second {
		t.Fatalf("Timeout = %v, want 7s", options.Timeout)
	}
}

func TestNormalizeServerOptionsUsesLocalDiscoveryDefault(t *testing.T) {
	v := viper.New()

	var options ServerOptions
	normalizeServerOptions(&options, v, "bbs-mall-service", "mall-service")

	if !reflect.DeepEqual(options.EtcdAddr, []string{"127.0.0.1:2379"}) {
		t.Fatalf("EtcdAddr = %v, want [127.0.0.1:2379]", options.EtcdAddr)
	}
	if options.Timeout != 10*time.Second {
		t.Fatalf("Timeout = %v, want 10s", options.Timeout)
	}
}
