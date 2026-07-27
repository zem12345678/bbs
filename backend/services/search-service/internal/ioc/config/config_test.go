package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestApplyEnvironmentOverridesSetsSearchServerPort(t *testing.T) {
	type serviceOptions struct {
		Name     string
		GRPCPort int
	}
	type grpcServerOptions struct {
		Port        int
		EtcdAddr    []string
		ServiceName string
	}

	tests := []struct {
		name        string
		serverPort  string
		servicePort string
		want        int
	}{
		{
			name:       "grpc server port",
			serverPort: "19106",
			want:       19106,
		},
		{
			name:        "service grpc port",
			servicePort: "19107",
			want:        19107,
		},
		{
			name:        "server port takes precedence",
			serverPort:  "19108",
			servicePort: "19109",
			want:        19108,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BBS_SEARCH_GRPC_SERVER_PORT", tt.serverPort)
			t.Setenv("BBS_SEARCH_SERVICE_GRPC_PORT", tt.servicePort)

			v := viper.New()
			v.SetConfigType("yaml")
			if err := v.ReadConfig(strings.NewReader(`
service:
  name: bbs-search-service
  grpcPort: 9106
grpc:
  server:
    port: 9106
    etcdAddr:
      - 127.0.0.1:2379
    serviceName: bbs-search-service
`)); err != nil {
				t.Fatalf("read config: %v", err)
			}

			applyEnvironmentOverrides(v)

			var service serviceOptions
			if err := v.UnmarshalKey("service", &service); err != nil {
				t.Fatalf("unmarshal service: %v", err)
			}
			if service.Name != "bbs-search-service" || service.GRPCPort != tt.want {
				t.Fatalf("service = %#v, want preserved name and port %d", service, tt.want)
			}

			var grpcServer grpcServerOptions
			if err := v.UnmarshalKey("grpc.server", &grpcServer); err != nil {
				t.Fatalf("unmarshal grpc server: %v", err)
			}
			if grpcServer.Port != tt.want {
				t.Fatalf("grpc.server.port = %d, want %d", grpcServer.Port, tt.want)
			}
			if grpcServer.ServiceName != "bbs-search-service" || len(grpcServer.EtcdAddr) != 1 || grpcServer.EtcdAddr[0] != "127.0.0.1:2379" {
				t.Fatalf("grpc server settings were not preserved: %#v", grpcServer)
			}
		})
	}
}

func TestApplyEnvironmentOverridesSetsRuntimeConfigWithoutPort(t *testing.T) {
	t.Setenv("BBS_SEARCH_ELASTICSEARCH_ADDRESSES", "http://es-a:9200, http://es-b:9200")
	t.Setenv("BBS_SEARCH_ELASTICSEARCH_INDICES_ARTICLES", "env_articles")
	t.Setenv("BBS_SEARCH_ELASTICSEARCH_INDICES_TOPICS", "env_topics")
	t.Setenv("BBS_SEARCH_KAFKA_BROKERS", "kafka-a:9092, kafka-b:9092")
	t.Setenv("BBS_SEARCH_KAFKA_ARTICLE_TOPIC", "env.article.events")
	t.Setenv("BBS_SEARCH_KAFKA_COMMENT_TOPIC", "env.comment.events")
	t.Setenv("BBS_SEARCH_KAFKA_REACTION_TOPIC", "env.reaction.events")
	t.Setenv("BBS_SEARCH_KAFKA_ARTICLE_GROUP_ID", "env-article-group")
	t.Setenv("BBS_SEARCH_KAFKA_COMMENT_GROUP_ID", "env-comment-group")
	t.Setenv("BBS_SEARCH_KAFKA_REACTION_GROUP_ID", "env-reaction-group")
	t.Setenv("BBS_SEARCH_GRPC_SERVER_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_SEARCH_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "env-search-internal-token")
	t.Setenv("BBS_SEARCH_INTERNAL_AUTH_TOKEN", "legacy-search-internal-token")
	t.Setenv("BBS_SEARCH_TRACE_ENV", "production")

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(`
es:
  url:
    - http://127.0.0.1:9200
  indices:
    articles: bbs_articles
    topics: bbs_topics
kafka:
  brokers:
    - 127.0.0.1:9092
  articleTopic: article.events
grpc:
  server:
    etcdAddr:
      - 127.0.0.1:2379
trace:
  env: local
`)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	applyEnvironmentOverrides(v)

	if got, want := v.GetStringSlice("es.url"), []string{"http://es-a:9200", "http://es-b:9200"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("es.url = %#v, want %#v", got, want)
	}
	if got := v.GetString("es.indices.articles"); got != "env_articles" {
		t.Fatalf("es.indices.articles = %q", got)
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
	if got := v.GetString("grpc.server.internalAuthToken"); got != "env-search-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
	if got := v.GetString("trace.env"); got != "production" {
		t.Fatalf("trace.env = %q", got)
	}
}

func TestApplyEnvironmentOverridesSupportsLegacyInternalAuthToken(t *testing.T) {
	t.Setenv("BBS_SEARCH_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "")
	t.Setenv("BBS_SEARCH_INTERNAL_AUTH_TOKEN", "legacy-search-internal-token")

	v := viper.New()
	applyEnvironmentOverrides(v)

	if got := v.GetString("grpc.server.internalAuthToken"); got != "legacy-search-internal-token" {
		t.Fatalf("grpc.server.internalAuthToken = %q", got)
	}
}

func TestInternalAuthDefaultAndProductionValidation(t *testing.T) {
	local := viper.New()
	local.Set("trace.env", "local")
	setInternalAuthDefault(local)
	if got := local.GetString("grpc.server.internalAuthToken"); got != localDevInternalAuthToken {
		t.Fatalf("local internal auth token = %q", got)
	}
	if err := validate(local); err != nil {
		t.Fatalf("validate local config: %v", err)
	}

	for _, environment := range []string{"production", "prod"} {
		for _, tc := range []struct {
			name  string
			token string
		}{
			{name: "empty", token: ""},
			{name: "default", token: localDevInternalAuthToken},
			{name: "short", token: "too-short"},
		} {
			t.Run(environment+"/"+tc.name, func(t *testing.T) {
				v := viper.New()
				v.Set("trace.env", environment)
				v.Set("grpc.server.internalAuthToken", tc.token)
				if err := validate(v); err == nil {
					t.Fatal("validate production config error = nil")
				}
			})
		}
	}

	production := viper.New()
	production.Set("trace.env", "production")
	production.Set("grpc.server.internalAuthToken", "production-search-token-with-at-least-32-bytes")
	if err := validate(production); err != nil {
		t.Fatalf("validate production config: %v", err)
	}
}

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_SEARCH_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_SEARCH_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_SEARCH_SKIP_NACOS=true")
	}
}
