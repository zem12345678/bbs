package main

import "testing"

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
