package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// envOverrideYAML mirrors the shape of configs/config.yaml closely enough to prove
// that env overrides keep sibling keys and still outrank the raw env string.
const envOverrideYAML = `
postgres:
  dsn: file-dsn
  debug: false
  maxOpenConns: 40
grpc:
  server:
    port: 9101
    etcdAddr:
      - 127.0.0.1:2379
    serviceName: bbs-admin-service
    timeout: 10s
  client:
    etcdAddr:
      - 127.0.0.1:2379
    timeout: 5s
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

// TestApplyEnvOverridesSplitsEtcdAddr guards the regression where a CSV split landed
// in viper's config layer and AutomaticEnv served the raw string on a flat read.
func TestApplyEnvOverridesSplitsEtcdAddr(t *testing.T) {
	t.Setenv("BBS_ADMIN_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379,etcd-b:2379")
	t.Setenv("BBS_ADMIN_GRPC_CLIENT_ETCD_ADDR", "etcd-c:2379,etcd-d:2379")
	v := newEnvOverrideViper(t)

	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("applyEnvOverrides: %v", err)
	}

	wantServer := []string{"etcd-a:2379", "etcd-b:2379"}
	if got := v.GetStringSlice("grpc.server.etcdAddr"); !reflect.DeepEqual(got, wantServer) {
		t.Fatalf("flat grpc.server.etcdAddr = %#v, want %#v", got, wantServer)
	}
	wantClient := []string{"etcd-c:2379", "etcd-d:2379"}
	if got := v.GetStringSlice("grpc.client.etcdAddr"); !reflect.DeepEqual(got, wantClient) {
		t.Fatalf("flat grpc.client.etcdAddr = %#v, want %#v", got, wantClient)
	}

	var section struct {
		Server struct {
			Port        int      `mapstructure:"port"`
			EtcdAddr    []string `mapstructure:"etcdAddr"`
			ServiceName string   `mapstructure:"serviceName"`
			Timeout     string   `mapstructure:"timeout"`
		} `mapstructure:"server"`
		Client struct {
			EtcdAddr []string `mapstructure:"etcdAddr"`
			Timeout  string   `mapstructure:"timeout"`
		} `mapstructure:"client"`
	}
	if err := v.UnmarshalKey("grpc", &section); err != nil {
		t.Fatalf("unmarshal grpc: %v", err)
	}
	if !reflect.DeepEqual(section.Server.EtcdAddr, wantServer) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", section.Server.EtcdAddr, wantServer)
	}
	if !reflect.DeepEqual(section.Client.EtcdAddr, wantClient) {
		t.Fatalf("grpc.client.etcdAddr = %#v, want %#v", section.Client.EtcdAddr, wantClient)
	}
	if section.Server.Port != 9101 {
		t.Fatalf("grpc.server.port = %d, want 9101", section.Server.Port)
	}
	if section.Server.ServiceName != "bbs-admin-service" {
		t.Fatalf("grpc.server.serviceName = %q", section.Server.ServiceName)
	}
	if section.Server.Timeout != "10s" {
		t.Fatalf("grpc.server.timeout = %q, want %q", section.Server.Timeout, "10s")
	}
	if section.Client.Timeout != "5s" {
		t.Fatalf("grpc.client.timeout = %q, want %q", section.Client.Timeout, "5s")
	}
}

// TestApplyEnvOverridesKeepsPostgresSiblings proves that overriding only the DSN
// leaves the remaining postgres settings from the config file intact.
func TestApplyEnvOverridesKeepsPostgresSiblings(t *testing.T) {
	t.Setenv("BBS_ADMIN_POSTGRES_DSN", "env-dsn")
	v := newEnvOverrideViper(t)

	if err := applyEnvOverrides(v); err != nil {
		t.Fatalf("applyEnvOverrides: %v", err)
	}

	var pg struct {
		DSN          string `mapstructure:"dsn"`
		MaxOpenConns int    `mapstructure:"maxOpenConns"`
	}
	if err := v.UnmarshalKey("postgres", &pg); err != nil {
		t.Fatalf("unmarshal postgres: %v", err)
	}
	if pg.DSN != "env-dsn" {
		t.Fatalf("postgres.dsn = %q, want %q", pg.DSN, "env-dsn")
	}
	if pg.MaxOpenConns != 40 {
		t.Fatalf("postgres.maxOpenConns = %d, want 40", pg.MaxOpenConns)
	}
}

// TestApplyGRPCPortEnvOverrideKeepsServerSiblings proves that publishing the port
// override no longer hides etcdAddr and the other grpc.server settings.
func TestApplyGRPCPortEnvOverrideKeepsServerSiblings(t *testing.T) {
	t.Setenv("BBS_ADMIN_GRPC_SERVER_PORT", "29101")
	v := newEnvOverrideViper(t)

	if err := applyGRPCPortEnvOverride(v, "BBS_ADMIN_GRPC_SERVER_PORT", "BBS_ADMIN_SERVICE_GRPC_PORT"); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride: %v", err)
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
	if server.Port != 29101 {
		t.Fatalf("grpc.server.port = %d, want 29101", server.Port)
	}
	if want := []string{"127.0.0.1:2379"}; !reflect.DeepEqual(server.EtcdAddr, want) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", server.EtcdAddr, want)
	}
	if server.ServiceName != "bbs-admin-service" {
		t.Fatalf("grpc.server.serviceName = %q", server.ServiceName)
	}
	if server.Timeout != "10s" {
		t.Fatalf("grpc.server.timeout = %q, want %q", server.Timeout, "10s")
	}
	if got := v.GetInt("service.grpcPort"); got != 29101 {
		t.Fatalf("service.grpcPort = %d, want 29101", got)
	}
}
