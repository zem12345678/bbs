package datasource

import "testing"

func TestNewPoolConfigAllowsNestedOrderReads(t *testing.T) {
	config, err := newPoolConfig(&Options{
		Dsn:          "postgres://user:password@127.0.0.1:5432/bbs?sslmode=disable",
		MaxOpenConns: 2,
	})
	if err != nil {
		t.Fatalf("new pool config: %v", err)
	}
	if config.MaxConns != 2 {
		t.Fatalf("max connections = %d, want 2", config.MaxConns)
	}
}
