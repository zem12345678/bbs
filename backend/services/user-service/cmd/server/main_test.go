package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	path := writeUserConfigFile(t, `
service:
  name: file-user-service
  grpcPort: 9102
postgres:
  dsn: file-postgres-dsn
  debug: false
kafka:
  brokers:
    - file-kafka:9092
  topic: file.user.events
snowflake:
  workerId: 2
jwt:
  secret: file-jwt
  ttl: 168h
password:
  minLength: 8
`)
	t.Setenv("BBS_USER_SERVICE_GRPC_PORT", "19102")
	t.Setenv("BBS_USER_POSTGRES_DSN", "env-postgres-dsn")
	t.Setenv("BBS_USER_POSTGRES_DEBUG", "true")
	t.Setenv("BBS_USER_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_USER_KAFKA_TOPIC", "env.user.events")
	t.Setenv("BBS_USER_SNOWFLAKE_WORKER_ID", "22")
	t.Setenv("BBS_USER_JWT_SECRET", "env-jwt")
	t.Setenv("BBS_USER_JWT_TTL", "24h")
	t.Setenv("BBS_USER_PASSWORD_MIN_LENGTH", "12")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.GRPCPort != 19102 {
		t.Fatalf("grpc port = %d, want 19102", cfg.Service.GRPCPort)
	}
	if cfg.Postgres.DSN != "env-postgres-dsn" {
		t.Fatalf("postgres dsn = %q", cfg.Postgres.DSN)
	}
	if !cfg.Postgres.Debug {
		t.Fatalf("postgres debug should be true")
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "env-kafka-1:9092" || cfg.Kafka.Brokers[1] != "env-kafka-2:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "env.user.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
	}
	if cfg.Snowflake.WorkerID != 22 {
		t.Fatalf("worker id = %d", cfg.Snowflake.WorkerID)
	}
	if cfg.JWT.Secret != "env-jwt" {
		t.Fatalf("jwt secret = %q", cfg.JWT.Secret)
	}
	if cfg.JWT.TTL != 24*time.Hour {
		t.Fatalf("jwt ttl = %s, want 24h", cfg.JWT.TTL)
	}
	if cfg.Password.MinLength != 12 {
		t.Fatalf("password min length = %d", cfg.Password.MinLength)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeUserConfigFile(t, `{}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "bbs-user-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9102 {
		t.Fatalf("grpc port = %d, want 9102", cfg.Service.GRPCPort)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "user.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
	}
	if cfg.Snowflake.WorkerID != 2 {
		t.Fatalf("worker id = %d", cfg.Snowflake.WorkerID)
	}
	if cfg.JWT.Secret == "" || cfg.JWT.TTL != 7*24*time.Hour {
		t.Fatalf("jwt defaults secret=%q ttl=%s", cfg.JWT.Secret, cfg.JWT.TTL)
	}
	if cfg.Password.MinLength != 8 {
		t.Fatalf("password min length = %d", cfg.Password.MinLength)
	}
}

func TestLoadConfigRejectsDefaultJWTSecretInProduction(t *testing.T) {
	path := writeUserConfigFile(t, "trace:\n  env: production\n")

	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with default JWT secret to fail")
	}
}

func writeUserConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
