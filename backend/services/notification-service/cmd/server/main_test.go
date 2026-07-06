package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	path := writeNotificationConfigFile(t, `
service:
  name: file-notification-service
  grpcPort: 9108
postgres:
  dsn: file-postgres-dsn
kafka:
  brokers:
    - file-kafka:9092
  userTopic: file.user.events
  articleTopic: file.article.events
  commentTopic: file.comment.events
  reactionTopic: file.reaction.events
  userGroupId: file-user-group
  articleGroupId: file-article-group
  commentGroupId: file-comment-group
  reactionGroupId: file-reaction-group
`)
	t.Setenv("BBS_NOTIFICATION_SERVICE_GRPC_PORT", "19108")
	t.Setenv("BBS_NOTIFICATION_POSTGRES_DSN", "env-postgres-dsn")
	t.Setenv("BBS_NOTIFICATION_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_NOTIFICATION_KAFKA_USER_TOPIC", "env.user.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_COMMENT_TOPIC", "env.comment.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_REACTION_TOPIC", "env.reaction.events")
	t.Setenv("BBS_NOTIFICATION_KAFKA_USER_GROUP_ID", "env-user-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_COMMENT_GROUP_ID", "env-comment-group")
	t.Setenv("BBS_NOTIFICATION_KAFKA_REACTION_GROUP_ID", "env-reaction-group")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.GRPCPort != 19108 {
		t.Fatalf("grpc port = %d, want 19108", cfg.Service.GRPCPort)
	}
	if cfg.Postgres.DSN != "env-postgres-dsn" {
		t.Fatalf("postgres dsn = %q", cfg.Postgres.DSN)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "env-kafka-1:9092" || cfg.Kafka.Brokers[1] != "env-kafka-2:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.UserTopic != "env.user.events" || cfg.Kafka.ArticleTopic != "env.article.events" || cfg.Kafka.CommentTopic != "env.comment.events" || cfg.Kafka.ReactionTopic != "env.reaction.events" {
		t.Fatalf("kafka topics = %#v", cfg.Kafka)
	}
	if cfg.Kafka.UserGroupID != "env-user-group" || cfg.Kafka.ArticleGroupID != "env-article-group" || cfg.Kafka.CommentGroupID != "env-comment-group" || cfg.Kafka.ReactionGroupID != "env-reaction-group" {
		t.Fatalf("kafka group ids = %#v", cfg.Kafka)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeNotificationConfigFile(t, `{}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "notification-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9108 {
		t.Fatalf("grpc port = %d, want 9108", cfg.Service.GRPCPort)
	}
	if cfg.Postgres.DSN == "" {
		t.Fatalf("expected postgres dsn default")
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.UserTopic != "user.events" || cfg.Kafka.ArticleTopic != "article.events" || cfg.Kafka.CommentTopic != "comment.events" || cfg.Kafka.ReactionTopic != "reaction.events" {
		t.Fatalf("kafka topics = %#v", cfg.Kafka)
	}
	if cfg.Kafka.UserGroupID == "" || cfg.Kafka.ArticleGroupID == "" || cfg.Kafka.CommentGroupID == "" || cfg.Kafka.ReactionGroupID == "" {
		t.Fatalf("expected kafka group id defaults, got %#v", cfg.Kafka)
	}
}

func writeNotificationConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
