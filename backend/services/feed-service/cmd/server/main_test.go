package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	path := writeFeedConfigFile(t, `
service:
  name: file-feed-service
  grpcPort: 9113
redis:
  addr: file-redis:6379
  db: 0
  password: file-redis-password
kafka:
  brokers:
    - file-kafka:9092
  articleTopic: file.article.events
  commentTopic: file.comment.events
  reactionTopic: file.reaction.events
  articleGroupId: file-article-group
  commentGroupId: file-comment-group
  reactionGroupId: file-reaction-group
`)
	t.Setenv("BBS_FEED_SERVICE_GRPC_PORT", "19113")
	t.Setenv("BBS_FEED_REDIS_ADDR", "env-redis:6379")
	t.Setenv("BBS_FEED_REDIS_DB", "3")
	t.Setenv("BBS_FEED_REDIS_PASSWORD", "env-redis-password")
	t.Setenv("BBS_FEED_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_FEED_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_FEED_KAFKA_COMMENT_TOPIC", "env.comment.events")
	t.Setenv("BBS_FEED_KAFKA_REACTION_TOPIC", "env.reaction.events")
	t.Setenv("BBS_FEED_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_FEED_KAFKA_COMMENT_GROUP_ID", "env-comment-group")
	t.Setenv("BBS_FEED_KAFKA_REACTION_GROUP_ID", "env-reaction-group")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.GRPCPort != 19113 {
		t.Fatalf("grpc port = %d, want 19113", cfg.Service.GRPCPort)
	}
	if cfg.Redis.Addr != "env-redis:6379" || cfg.Redis.DB != 3 || cfg.Redis.Password != "env-redis-password" {
		t.Fatalf("redis config = %#v", cfg.Redis)
	}
	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "env-kafka-1:9092" || cfg.Kafka.Brokers[1] != "env-kafka-2:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.ArticleTopic != "env.article.events" || cfg.Kafka.CommentTopic != "env.comment.events" || cfg.Kafka.ReactionTopic != "env.reaction.events" {
		t.Fatalf("kafka topics = %#v", cfg.Kafka)
	}
	if cfg.Kafka.ArticleGroupID != "env-article-group" || cfg.Kafka.CommentGroupID != "env-comment-group" || cfg.Kafka.ReactionGroupID != "env-reaction-group" {
		t.Fatalf("kafka group ids = %#v", cfg.Kafka)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeFeedConfigFile(t, `{}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "feed-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9113 {
		t.Fatalf("grpc port = %d, want 9113", cfg.Service.GRPCPort)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Fatalf("redis addr = %q", cfg.Redis.Addr)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "127.0.0.1:9092" {
		t.Fatalf("kafka brokers = %#v", cfg.Kafka.Brokers)
	}
	if cfg.Kafka.ArticleTopic != "article.events" || cfg.Kafka.CommentTopic != "comment.events" || cfg.Kafka.ReactionTopic != "reaction.events" {
		t.Fatalf("kafka topics = %#v", cfg.Kafka)
	}
	if cfg.Kafka.ArticleGroupID == "" || cfg.Kafka.CommentGroupID == "" || cfg.Kafka.ReactionGroupID == "" {
		t.Fatalf("expected kafka group id defaults, got %#v", cfg.Kafka)
	}
}

func writeFeedConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
