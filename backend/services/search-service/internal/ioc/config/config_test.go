package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyEnvironmentOverridesSetsSearchServerPort(t *testing.T) {
	type serviceOptions struct {
		Name     string
		GRPCPort int
	}
	type grpcServerOptions struct {
		Port        int
		EtcdAddr    []string
		ServiceName string
	}

	tests := []struct {
		name        string
		serverPort  string
		servicePort string
		want        int
	}{
		{
			name:       "grpc server port",
			serverPort: "19106",
			want:       19106,
		},
		{
			name:        "service grpc port",
			servicePort: "19107",
			want:        19107,
		},
		{
			name:        "server port takes precedence",
			serverPort:  "19108",
			servicePort: "19109",
			want:        19108,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BBS_SEARCH_GRPC_SERVER_PORT", tt.serverPort)
			t.Setenv("BBS_SEARCH_SERVICE_GRPC_PORT", tt.servicePort)

			v := viper.New()
			v.SetConfigType("yaml")
			if err := v.ReadConfig(strings.NewReader(`
service:
  name: bbs-search-service
  grpcPort: 9106
grpc:
  server:
    port: 9106
    etcdAddr:
      - 127.0.0.1:2379
    serviceName: bbs-search-service
`)); err != nil {
				t.Fatalf("read config: %v", err)
			}

			applyEnvironmentOverrides(v)

			var service serviceOptions
			if err := v.UnmarshalKey("service", &service); err != nil {
				t.Fatalf("unmarshal service: %v", err)
			}
			if service.Name != "bbs-search-service" || service.GRPCPort != tt.want {
				t.Fatalf("service = %#v, want preserved name and port %d", service, tt.want)
			}

			var grpcServer grpcServerOptions
			if err := v.UnmarshalKey("grpc.server", &grpcServer); err != nil {
				t.Fatalf("unmarshal grpc server: %v", err)
			}
			if grpcServer.Port != tt.want {
				t.Fatalf("grpc.server.port = %d, want %d", grpcServer.Port, tt.want)
			}
			if grpcServer.ServiceName != "bbs-search-service" || len(grpcServer.EtcdAddr) != 1 || grpcServer.EtcdAddr[0] != "127.0.0.1:2379" {
				t.Fatalf("grpc server settings were not preserved: %#v", grpcServer)
			}
		})
	}
}
