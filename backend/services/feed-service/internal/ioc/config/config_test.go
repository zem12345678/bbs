package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_FEED_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_FEED_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_FEED_SKIP_NACOS=true")
	}
}

func TestApplyGRPCPortEnvOverride(t *testing.T) {
	t.Setenv("BBS_FEED_SERVICE_GRPC_PORT", "19113")

	v := viper.New()
	if err := applyGRPCPortEnvOverride(v,
		"BBS_FEED_GRPC_SERVER_PORT",
		"BBS_FEED_SERVICE_GRPC_PORT",
	); err != nil {
		t.Fatalf("applyGRPCPortEnvOverride() error = %v", err)
	}
	if got := v.GetInt("service.grpcPort"); got != 19113 {
		t.Fatalf("service.grpcPort = %d, want 19113", got)
	}
	if got := v.GetInt("grpc.server.port"); got != 19113 {
		t.Fatalf("grpc.server.port = %d, want 19113", got)
	}

	t.Setenv("BBS_FEED_SERVICE_GRPC_PORT", "0")
	if err := applyGRPCPortEnvOverride(v, "BBS_FEED_SERVICE_GRPC_PORT"); err == nil {
		t.Fatal("applyGRPCPortEnvOverride() accepted an invalid port")
	}
}

func TestApplyEnvOverridesSetsRuntimeConfig(t *testing.T) {
	t.Setenv("BBS_FEED_REDIS_ADDR", "redis:6379")
	t.Setenv("BBS_FEED_REDIS_DB", "7")
	t.Setenv("BBS_FEED_REDIS_PASSWORD", "redis-pass")
	t.Setenv("BBS_FEED_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092")
	t.Setenv("BBS_FEED_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_FEED_KAFKA_COMMENT_TOPIC", "env.comment.events")
	t.Setenv("BBS_FEED_KAFKA_REACTION_TOPIC", "env.reaction.events")
	t.Setenv("BBS_FEED_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_FEED_KAFKA_COMMENT_GROUP_ID", "env-comment-group")
	t.Setenv("BBS_FEED_KAFKA_REACTION_GROUP_ID", "env-reaction-group")
	t.Setenv("BBS_FEED_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_FEED_TRACE_ENV", "production")

	v := viper.New()
	applyEnvOverrides(v)

	if got := v.GetString("redis.url"); got != "redis:6379" {
		t.Fatalf("redis.url = %q", got)
	}
	if got := v.GetInt("redis.dbNum"); got != 7 {
		t.Fatalf("redis.dbNum = %d", got)
	}
	if got := v.GetString("redis.password"); got != "redis-pass" {
		t.Fatalf("redis.password = %q", got)
	}
	if got, want := v.GetStringSlice("kafka.brokers"), []string{"kafka-a:9092", "kafka-b:9092"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("kafka.brokers = %#v, want %#v", got, want)
	}
	if got := v.GetString("kafka.articleTopic"); got != "env.article.events" {
		t.Fatalf("kafka.articleTopic = %q", got)
	}
	if got := v.GetString("kafka.commentGroupId"); got != "env-comment-group" {
		t.Fatalf("kafka.commentGroupId = %q", got)
	}
	if got, want := v.GetStringSlice("grpc.server.etcdAddr"), []string{"etcd-a:2379", "etcd-b:2379"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("grpc.server.etcdAddr = %#v, want %#v", got, want)
	}
	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
}

func TestConfigureEnvBindsInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_FEED_GRPC_SERVER_INTERNAL_AUTH_TOKEN", " configured-feed-token ")
	v := viper.New()

	configureEnv(v)
	setInternalAuthDefault(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != " configured-feed-token " {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestValidateRejectsDefaultInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setInternalAuthDefault(v)
	v.Set("trace.env", "production")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "internalAuthToken") {
		t.Fatalf("validate error = %v, want production internal auth token error", err)
	}
}

func TestValidateRejectsShortInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setInternalAuthDefault(v)
	v.Set("trace.env", "prod")
	v.Set("grpc.server.internalAuthToken", "too-short")

	err := validate(v)
	if err == nil || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("validate error = %v, want short internal auth token error", err)
	}
}

func TestValidateAcceptsConfiguredInternalAuthTokenInProduction(t *testing.T) {
	v := viper.New()
	setInternalAuthDefault(v)
	v.Set("trace.env", "production")
	v.Set("grpc.server.internalAuthToken", "production-feed-internal-token-with-32-bytes")

	if err := validate(v); err != nil {
		t.Fatalf("validate configured internal auth token: %v", err)
	}
}
