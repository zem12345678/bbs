package server

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

type config struct {
	Service struct {
		Name     string
		GRPCPort int
	}
	GRPC struct {
		Server struct {
			Port        int
			ServiceName string
			EtcdAddr    []string
		}
	}
	Elasticsearch struct {
		Addresses []string
		Indices   struct {
			Articles          string
			Topics            string
			Users             string
			AccountTombstones string
		}
	}
	Kafka struct {
		Brokers         []string
		ArticleTopic    string
		CommentTopic    string
		ReactionTopic   string
		UserTopic       string
		GroupID         string
		ArticleGroupID  string
		CommentGroupID  string
		ReactionGroupID string
		UserGroupID     string
	}
}

func loadConfig(path string) (*config, error) {
	v := viper.New()
	configureEnv(v)
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	applyEnvOverrides(v)
	var cfg config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "bbs-search-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = cfg.GRPC.Server.Port
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9106
	}
	if cfg.GRPC.Server.Port == 0 {
		cfg.GRPC.Server.Port = cfg.Service.GRPCPort
	}
	if cfg.GRPC.Server.ServiceName == "" {
		cfg.GRPC.Server.ServiceName = cfg.Service.Name
	}
	if len(cfg.GRPC.Server.EtcdAddr) == 0 {
		cfg.GRPC.Server.EtcdAddr = []string{"127.0.0.1:2379"}
	}
	if len(cfg.Elasticsearch.Addresses) == 0 {
		cfg.Elasticsearch.Addresses = []string{"http://127.0.0.1:9200"}
	}
	if cfg.Elasticsearch.Indices.Articles == "" {
		cfg.Elasticsearch.Indices.Articles = "bbs_articles"
	}
	if cfg.Elasticsearch.Indices.Topics == "" {
		cfg.Elasticsearch.Indices.Topics = "bbs_topics"
	}
	if cfg.Elasticsearch.Indices.Users == "" {
		cfg.Elasticsearch.Indices.Users = "bbs_users_v2"
	}
	if cfg.Elasticsearch.Indices.AccountTombstones == "" {
		cfg.Elasticsearch.Indices.AccountTombstones = "bbs_search_account_tombstones_v1"
	}
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"127.0.0.1:9092"}
	}
	if cfg.Kafka.ArticleTopic == "" {
		cfg.Kafka.ArticleTopic = "article.events"
	}
	if cfg.Kafka.CommentTopic == "" {
		cfg.Kafka.CommentTopic = "comment.events"
	}
	if cfg.Kafka.ReactionTopic == "" {
		cfg.Kafka.ReactionTopic = "reaction.events"
	}
	if cfg.Kafka.UserTopic == "" {
		cfg.Kafka.UserTopic = "user.events"
	}
	if cfg.Kafka.ArticleGroupID == "" {
		cfg.Kafka.ArticleGroupID = cfg.Kafka.GroupID
	}
	if cfg.Kafka.ArticleGroupID == "" {
		cfg.Kafka.ArticleGroupID = "bbs-search-indexer"
	}
	if cfg.Kafka.CommentGroupID == "" {
		cfg.Kafka.CommentGroupID = "bbs-search-comment-counter"
	}
	if cfg.Kafka.ReactionGroupID == "" {
		cfg.Kafka.ReactionGroupID = "bbs-search-reaction-counter"
	}
	if cfg.Kafka.UserGroupID == "" {
		cfg.Kafka.UserGroupID = "bbs-search-user-indexer"
	}
	return &cfg, nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_SEARCH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_SEARCH_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_SEARCH_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_SEARCH_GRPC_SERVER_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_SEARCH_GRPC_SERVER_SERVICE_NAME")
	bindEnv(v, "grpc.server.etcdAddr", "BBS_SEARCH_GRPC_SERVER_ETCD_ADDR")
	bindEnv(v, "elasticsearch.addresses", "BBS_SEARCH_ELASTICSEARCH_ADDRESSES")
	bindEnv(v, "elasticsearch.indices.articles", "BBS_SEARCH_ELASTICSEARCH_INDICES_ARTICLES")
	bindEnv(v, "elasticsearch.indices.topics", "BBS_SEARCH_ELASTICSEARCH_INDICES_TOPICS")
	bindEnv(v, "elasticsearch.indices.users", "BBS_SEARCH_ELASTICSEARCH_INDICES_USERS")
	bindEnv(v, "elasticsearch.indices.accountTombstones", "BBS_SEARCH_ELASTICSEARCH_INDICES_ACCOUNT_TOMBSTONES")
	bindEnv(v, "kafka.brokers", "BBS_SEARCH_KAFKA_BROKERS")
	bindEnv(v, "kafka.articleTopic", "BBS_SEARCH_KAFKA_ARTICLE_TOPIC")
	bindEnv(v, "kafka.commentTopic", "BBS_SEARCH_KAFKA_COMMENT_TOPIC")
	bindEnv(v, "kafka.reactionTopic", "BBS_SEARCH_KAFKA_REACTION_TOPIC")
	bindEnv(v, "kafka.userTopic", "BBS_SEARCH_KAFKA_USER_TOPIC")
	bindEnv(v, "kafka.groupId", "BBS_SEARCH_KAFKA_GROUP_ID")
	bindEnv(v, "kafka.articleGroupId", "BBS_SEARCH_KAFKA_ARTICLE_GROUP_ID")
	bindEnv(v, "kafka.commentGroupId", "BBS_SEARCH_KAFKA_COMMENT_GROUP_ID")
	bindEnv(v, "kafka.reactionGroupId", "BBS_SEARCH_KAFKA_REACTION_GROUP_ID")
	bindEnv(v, "kafka.userGroupId", "BBS_SEARCH_KAFKA_USER_GROUP_ID")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_ELASTICSEARCH_ADDRESSES")); value != "" {
		v.Set("elasticsearch.addresses", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_KAFKA_BROKERS")); value != "" {
		v.Set("kafka.brokers", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
	}
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
