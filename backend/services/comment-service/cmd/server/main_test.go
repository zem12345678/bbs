package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	path := writeCommentConfigFile(t, `
service:
  name: file-comment-service
  grpcPort: 9104
mongo:
  uri: mongodb://file-mongo:27017
  database: file_comment
kafka:
  brokers:
    - file-kafka:9092
  topic: file.comment.events
snowflake:
  workerId: 4
`)
	t.Setenv("BBS_COMMENT_SERVICE_GRPC_PORT", "19104")
	t.Setenv("BBS_COMMENT_MONGO_URI", "mongodb://env-mongo:27017")
	t.Setenv("BBS_COMMENT_MONGO_DATABASE", "env_comment")
	t.Setenv("BBS_COMMENT_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_COMMENT_KAFKA_TOPIC", "env.comment.events")
	t.Setenv("BBS_COMMENT_SNOWFLAKE_WORKER_ID", "44")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.GRPCPort != 19104 {
		t.Fatalf("grpc port = %d, want 19104", cfg.Service.GRPCPort)
	}
	if cfg.Mongo.URI != "mongodb://env-mongo:27017" {
		t.Fatalf("mongo uri = %q", cfg.Mongo.URI)
	}
	if cfg.Mongo.Database != "env_comment" {
		t.Fatalf("mongo database = %q", cfg.Mongo.Database)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "env-kafka-1:9092" || cfg.Kafka.Brokers[1] != "env-kafka-2:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "env.comment.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
	}
	if cfg.Snowflake.WorkerID != 44 {
		t.Fatalf("worker id = %d", cfg.Snowflake.WorkerID)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeCommentConfigFile(t, `{}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "comment-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9104 {
		t.Fatalf("grpc port = %d, want 9104", cfg.Service.GRPCPort)
	}
	if cfg.Mongo.URI != "mongodb://127.0.0.1:27017" {
		t.Fatalf("mongo uri = %q", cfg.Mongo.URI)
	}
	if cfg.Mongo.Database != "bbs_comment" {
		t.Fatalf("mongo database = %q", cfg.Mongo.Database)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.Topic != "comment.events" {
		t.Fatalf("kafka topic = %q", cfg.Kafka.Topic)
	}
	if cfg.Snowflake.WorkerID != 4 {
		t.Fatalf("worker id = %d", cfg.Snowflake.WorkerID)
	}
}

func writeCommentConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
