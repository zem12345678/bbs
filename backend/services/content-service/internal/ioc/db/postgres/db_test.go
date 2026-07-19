package datasource

import (
	"database/sql"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigureSQLPoolLimitsConnections(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://user:password@127.0.0.1:5432/bbs?sslmode=disable")
	if err != nil {
		t.Fatalf("open database handle: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	configureSQLPool(db, &Options{MaxOpenConns: 1})
	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections = %d, want 1", got)
	}
}

func TestMaxOpenConnectionsUsesBoundedDefault(t *testing.T) {
	if got := maxOpenConnections(&Options{}); got != defaultMaxOpenConns {
		t.Fatalf("max open connections = %d, want %d", got, defaultMaxOpenConns)
	}
}

func TestOptionsUnmarshalMaxOpenConns(t *testing.T) {
	v := viper.New()
	v.Set("postgres.max_open_conns", 1)

	var options Options
	if err := v.UnmarshalKey("postgres", &options); err != nil {
		t.Fatalf("unmarshal options: %v", err)
	}
	if options.MaxOpenConns != 1 {
		t.Fatalf("max open connections = %d, want 1", options.MaxOpenConns)
	}
}
