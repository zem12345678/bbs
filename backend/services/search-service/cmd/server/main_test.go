package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	path := writeSearchConfigFile(t, `
service:
  name: file-search-service
  grpcPort: 9106
elasticsearch:
  addresses:
    - http://file-es:9200
  indices:
    articles: file_articles
    topics: file_topics
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
	t.Setenv("BBS_SEARCH_SERVICE_GRPC_PORT", "19106")
	t.Setenv("BBS_SEARCH_ELASTICSEARCH_ADDRESSES", "http://env-es-1:9200, http://env-es-2:9200")
	t.Setenv("BBS_SEARCH_ELASTICSEARCH_INDICES_ARTICLES", "env_articles")
	t.Setenv("BBS_SEARCH_ELASTICSEARCH_INDICES_TOPICS", "env_topics")
	t.Setenv("BBS_SEARCH_KAFKA_BROKERS", "env-kafka-1:9092, env-kafka-2:9092")
	t.Setenv("BBS_SEARCH_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_SEARCH_KAFKA_COMMENT_TOPIC", "env.comment.events")
	t.Setenv("BBS_SEARCH_KAFKA_REACTION_TOPIC", "env.reaction.events")
	t.Setenv("BBS_SEARCH_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_SEARCH_KAFKA_COMMENT_GROUP_ID", "env-comment-group")
	t.Setenv("BBS_SEARCH_KAFKA_REACTION_GROUP_ID", "env-reaction-group")

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.GRPCPort != 19106 {
		t.Fatalf("grpc port = %d, want 19106", cfg.Service.GRPCPort)
	}
	if len(cfg.Elasticsearch.Addresses) != 2 || cfg.Elasticsearch.Addresses[0] != "http://env-es-1:9200" || cfg.Elasticsearch.Addresses[1] != "http://env-es-2:9200" {
		t.Fatalf("elasticsearch addresses = %#v", cfg.Elasticsearch.Addresses)
	}
	if cfg.Elasticsearch.Indices.Articles != "env_articles" || cfg.Elasticsearch.Indices.Topics != "env_topics" {
		t.Fatalf("elasticsearch indices = %#v", cfg.Elasticsearch.Indices)
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
	path := writeSearchConfigFile(t, `{}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "search-service" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.GRPCPort != 9106 {
		t.Fatalf("grpc port = %d, want 9106", cfg.Service.GRPCPort)
	}
	if len(cfg.Elasticsearch.Addresses) != 1 || cfg.Elasticsearch.Addresses[0] != "http://127.0.0.1:9200" {
		t.Fatalf("elasticsearch addresses = %#v", cfg.Elasticsearch.Addresses)
	}
	if cfg.Elasticsearch.Indices.Articles != "bbs_articles" || cfg.Elasticsearch.Indices.Topics != "bbs_topics" {
		t.Fatalf("elasticsearch indices = %#v", cfg.Elasticsearch.Indices)
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

func writeSearchConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
