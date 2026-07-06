package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewAppliesEnvironmentOverrides(t *testing.T) {
	path := writeConfigFile(t, `
service:
  name: file-content-service
  grpcPort: 9103
postgres:
  dsn: file-postgres-dsn
  debug: false
redis:
  addr: file-redis:6379
  db: 0
  password: file-redis-password
kafka:
  brokers:
    - file-kafka:9092
  topic: file.article.events
cache:
  ttl: 5m
snowflake:
  workerId: 3
`)
	t.Setenv("BBS_CONTENT_SERVICE_GRPC_PORT", "19103")
	t.Setenv("BBS_CONTENT_POSTGRES_DSN", "env-postgres-dsn")
	t.Setenv("BBS_CONTENT_POSTGRES_DEBUG", "true")
	t.Setenv("BBS_CONTENT_REDIS_ADDR", "env-redis:6379")
	t.Setenv("BBS_CONTENT_REDIS_DB", "2")
	t.Setenv("BBS_CONTENT_REDIS_PASSWORD", "env-redis-password")
	t.Setenv("BBS_CONTENT_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_CONTENT_KAFKA_TOPIC", "env.article.events")
	t.Setenv("BBS_CONTENT_CACHE_TTL", "15m")
	t.Setenv("BBS_CONTENT_SNOWFLAKE_WORKER_ID", "33")

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if cfg.Service.GRPCPort != 19103 {
		t.Fatalf("grpc port = %d, want 19103", cfg.Service.GRPCPort)
	}
	if cfg.Postgres.DSN != "env-postgres-dsn" {
		t.Fatalf("postgres dsn = %q", cfg.Postgres.DSN)
	}
	if !cfg.Postgres.Debug {
		t.Fatalf("postgres debug should be true")
	}
	if cfg.Redis.Addr != "env-redis:6379" || cfg.Redis.DB != 2 || cfg.Redis.Password != "env-redis-password" {
		t.Fatalf("redis config = %#v", cfg.Redis)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "env-kafka-1:9092" || cfg.Kafka.Brokers[1] != "env-kafka-2:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "env.article.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
	}
	if cfg.Cache.TTL != 15*time.Minute {
		t.Fatalf("cache ttl = %s, want 15m", cfg.Cache.TTL)
	}
	if cfg.Snowflake.WorkerID != 33 {
		t.Fatalf("worker id = %d", cfg.Snowflake.WorkerID)
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	path := writeConfigFile(t, `{}`)

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if cfg.Service.Name != "content-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9103 {
		t.Fatalf("grpc port = %d, want 9103", cfg.Service.GRPCPort)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Fatalf("redis addr = %q", cfg.Redis.Addr)
	}
	if cfg.Cache.TTL != 5*time.Minute {
		t.Fatalf("cache ttl = %s, want 5m", cfg.Cache.TTL)
	}
	if cfg.Snowflake.WorkerID != 3 {
		t.Fatalf("worker id = %d", cfg.Snowflake.WorkerID)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
