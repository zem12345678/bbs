package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppliesEnvironmentOverrides(t *testing.T) {
	path := writeConfigFile(t, `
service:
  name: file-reaction-service
  grpcPort: 9105
postgres:
  dsn: file-postgres-dsn
redis:
  addr: file-redis:6379
  db: 0
  password: file-redis-password
kafka:
  brokers:
    - file-kafka:9092
  topic: file.reaction.events
reaction:
  rebuildCacheOnStart: false
`)
	t.Setenv("BBS_REACTION_SERVICE_GRPC_PORT", "19105")
	t.Setenv("BBS_REACTION_POSTGRES_DSN", "env-postgres-dsn")
	t.Setenv("BBS_REACTION_REDIS_ADDR", "env-redis:6379")
	t.Setenv("BBS_REACTION_REDIS_DB", "5")
	t.Setenv("BBS_REACTION_REDIS_PASSWORD", "env-redis-password")
	t.Setenv("BBS_REACTION_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_REACTION_KAFKA_TOPIC", "env.reaction.events")
	t.Setenv("BBS_REACTION_REBUILD_CACHE_ON_START", "true")

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if cfg.Service.GRPCPort != 19105 {
		t.Fatalf("grpc port = %d, want 19105", cfg.Service.GRPCPort)
	}
	if cfg.Postgres.DSN != "env-postgres-dsn" {
		t.Fatalf("postgres dsn = %q", cfg.Postgres.DSN)
	}
	if cfg.Redis.Addr != "env-redis:6379" || cfg.Redis.DB != 5 || cfg.Redis.Password != "env-redis-password" {
		t.Fatalf("redis config = %#v", cfg.Redis)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "env-kafka-1:9092" || cfg.Kafka.Brokers[1] != "env-kafka-2:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "env.reaction.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
	}
	if !cfg.Reaction.RebuildCacheOnStart {
		t.Fatalf("expected rebuild cache on start to be true")
	}
}

func TestNewAppliesDefaults(t *testing.T) {
	path := writeConfigFile(t, `{}`)

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if cfg.Service.Name != "reaction-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9105 {
		t.Fatalf("grpc port = %d, want 9105", cfg.Service.GRPCPort)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Fatalf("redis addr = %q", cfg.Redis.Addr)
	}
	if cfg.Postgres.DSN == "" {
		t.Fatalf("expected postgres dsn default")
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "reaction.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
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
