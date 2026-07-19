package main

import (
	"net"
	"testing"
)

func TestNewPostgresPoolConfigLimitsConnections(t *testing.T) {
	config, err := newPostgresPoolConfig("postgres://user:password@127.0.0.1:5432/bbs?sslmode=disable", 1)
	if err != nil {
		t.Fatalf("new pool config: %v", err)
	}
	if config.MaxConns != 1 {
		t.Fatalf("max connections = %d, want 1", config.MaxConns)
	}
	if config.MinConns != 0 {
		t.Fatalf("min connections = %d, want 0", config.MinConns)
	}
	if config.AfterRelease != nil {
		t.Fatal("pool should retain its bounded idle connection")
	}
}

func TestIsIntranetIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{ip: "192.168.31.164", want: true},
		{ip: "10.0.0.8", want: true},
		{ip: "172.30.144.1", want: true},
		{ip: "198.18.0.1", want: false},
		{ip: "169.254.1.1", want: false},
		{ip: "127.0.0.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := isIntranetIPv4(net.ParseIP(tt.ip)); got != tt.want {
				t.Fatalf("isIntranetIPv4(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
