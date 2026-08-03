package config

import (
	"bytes"
	"fmt"
	"os"
	"search-service/pkg/uuid"
	"strconv"
	"strings"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken           = "bbs-local-search-internal-token"
	minProductionInternalAuthTokenBytes = 32
)

type Options struct {
	Addr        string `mapstructure:"addr" toml:"addr" json:"addr" yaml:"addr" env:"NACOS_ADDR"`
	Port        uint64 `mapstructure:"port" toml:"port" json:"port" yaml:"port" env:"NACOS_PORT"`
	NamespaceID string `mapstructure:"namespaceId" toml:"namespaceId" json:"namespaceId" yaml:"namespaceId" env:"NACOS_NAMESPACEID"`
	DataID      string `mapstructure:"dataId" toml:"dataId" json:"dataId" yaml:"dataId" env:"NACOS_DATAID"`
	GroupID     string `mapstructure:"groupId" toml:"groupId" json:"groupId" yaml:"groupId" env:"NACOS_GROUPID"`
}

func New(path string) (*viper.Viper, error) {
	var (
		err error
		v   = viper.New()
		o   = new(Options)
	)
	v.AddConfigPath(".")
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err == nil {
		fmt.Printf("use config file -> %s\n", v.ConfigFileUsed())
	} else {
		return nil, errors.Wrap(err, "read config file error")
	}
	if !skipNacos() {
		if err = v.UnmarshalKey("nacos", o); err != nil {
			return nil, errors.Wrap(err, "unmarshal nacos option error")
		}
		group := stringDefault(o.GroupID, "DEFAULT_GROUP")

		sc := []constant.ServerConfig{
			{
				IpAddr: o.Addr,
				Port:   o.Port,
			},
		}
		//客服端配置
		cc := constant.ClientConfig{
			NamespaceId:         o.NamespaceID, // 如果需要支持多namespace，我们可以场景多个client,它们有不同的NamespaceId
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			LogDir:              "tmp/nacos/log",
			CacheDir:            "tmp/nacos/cache",
			//RotateTime:          "1h",
			//MaxAge:              3,
			LogLevel: "debug",
		}

		configClient, err := clients.CreateConfigClient(map[string]interface{}{
			"serverConfigs": sc,
			"clientConfig":  cc,
		})
		if err != nil {
			return nil, err
		}
		//获取配置
		content, err := configClient.GetConfig(vo.ConfigParam{
			DataId: o.DataID,
			Group:  group})

		if err != nil {
			return nil, err
		}
		err = v.MergeConfig(bytes.NewBufferString(content))

		if err != nil {
			return nil, errors.Wrap(err, "viper read nacos config error")
		}
		applyEnvironmentOverrides(v)

		err = configClient.ListenConfig(vo.ConfigParam{
			DataId: o.DataID,
			Group:  group,
			OnChange: func(namespace, group, dataId, data string) {
				//获取配置
				_ = v.MergeConfig(bytes.NewBufferString(data))

			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "listenConfig nacos config error")
		}
	} else {
		applyEnvironmentOverrides(v)
	}
	setInternalAuthDefault(v)
	if err := validate(v); err != nil {
		return nil, err
	}
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	v.Set("server.uuid", uuidstr)
	return v, err
}

func stringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_SEARCH_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func applyEnvironmentOverrides(v *viper.Viper) {
	setStringEnv(v, "service.name", "BBS_SEARCH_SERVICE_NAME")
	setStringEnv(v, "app.name", "BBS_SEARCH_APP_NAME")
	setStringEnv(v, "grpc.server.serviceName", "BBS_SEARCH_GRPC_SERVER_SERVICE_NAME")
	port := 0
	for _, name := range []string{"BBS_SEARCH_GRPC_SERVER_PORT", "BBS_SEARCH_SERVICE_GRPC_PORT"} {
		value := strings.TrimSpace(os.Getenv(name))
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			port = parsed
			break
		}
	}
	if port > 0 {
		service := v.GetStringMap("service")
		service["grpcport"] = port
		v.Set("service", service)

		grpcServer := v.GetStringMap("grpc.server")
		grpcServer["port"] = port
		v.Set("grpc.server", grpcServer)
	}

	if addresses := strings.TrimSpace(os.Getenv("BBS_SEARCH_ELASTICSEARCH_ADDRESSES")); addresses != "" {
		values := splitCommaSeparated(addresses)
		v.Set("elasticsearch.addresses", values)
		v.Set("es.url", values)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_ELASTICSEARCH_INDICES_ARTICLES")); value != "" {
		v.Set("elasticsearch.indices.articles", value)
		v.Set("es.indices.articles", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_ELASTICSEARCH_INDICES_TOPICS")); value != "" {
		v.Set("elasticsearch.indices.topics", value)
		v.Set("es.indices.topics", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_ELASTICSEARCH_INDICES_USERS")); value != "" {
		v.Set("elasticsearch.indices.users", value)
		v.Set("es.indices.users", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_ELASTICSEARCH_ENABLE_DEBUG_LOGGER")); value != "" {
		v.Set("es.enable_debug_logger", value)
	}
	if brokers := strings.TrimSpace(os.Getenv("BBS_SEARCH_KAFKA_BROKERS")); brokers != "" {
		v.Set("kafka.brokers", splitCommaSeparated(brokers))
	}
	setStringEnv(v, "kafka.username", "BBS_SEARCH_KAFKA_USERNAME")
	setStringEnv(v, "kafka.password", "BBS_SEARCH_KAFKA_PASSWORD")
	setStringEnv(v, "kafka.scram_algorithm", "BBS_SEARCH_KAFKA_SCRAM_ALGORITHM")
	setStringEnv(v, "kafka.articleTopic", "BBS_SEARCH_KAFKA_ARTICLE_TOPIC")
	setStringEnv(v, "kafka.commentTopic", "BBS_SEARCH_KAFKA_COMMENT_TOPIC")
	setStringEnv(v, "kafka.reactionTopic", "BBS_SEARCH_KAFKA_REACTION_TOPIC")
	setStringEnv(v, "kafka.userTopic", "BBS_SEARCH_KAFKA_USER_TOPIC")
	setStringEnv(v, "kafka.groupId", "BBS_SEARCH_KAFKA_GROUP_ID")
	setStringEnv(v, "kafka.articleGroupId", "BBS_SEARCH_KAFKA_ARTICLE_GROUP_ID")
	setStringEnv(v, "kafka.commentGroupId", "BBS_SEARCH_KAFKA_COMMENT_GROUP_ID")
	setStringEnv(v, "kafka.reactionGroupId", "BBS_SEARCH_KAFKA_REACTION_GROUP_ID")
	setStringEnv(v, "kafka.userGroupId", "BBS_SEARCH_KAFKA_USER_GROUP_ID")
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_SEARCH_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
	}
	if value := firstNonEmptyEnv("BBS_SEARCH_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_SEARCH_INTERNAL_AUTH_TOKEN"); value != "" {
		v.Set("grpc.server.internalAuthToken", value)
	}
	setStringEnv(v, "trace.grpcEndpoint", "BBS_SEARCH_TRACE_GRPC_ENDPOINT")
	setStringEnv(v, "trace.serviceName", "BBS_SEARCH_TRACE_SERVICE_NAME")
	setStringEnv(v, "trace.version", "BBS_SEARCH_TRACE_VERSION")
	setStringEnv(v, "trace.env", "BBS_SEARCH_TRACE_ENV")
}

func setStringEnv(v *viper.Viper, key string, env string) {
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		v.Set(key, value)
	}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func setInternalAuthDefault(v *viper.Viper) {
	if strings.TrimSpace(v.GetString("grpc.server.internalAuthToken")) == "" {
		v.Set("grpc.server.internalAuthToken", localDevInternalAuthToken)
	}
}

func validate(v *viper.Viper) error {
	environment := strings.ToLower(strings.TrimSpace(v.GetString("trace.env")))
	if environment != "production" && environment != "prod" {
		return nil
	}
	return validateProductionInternalAuthToken(v.GetString("grpc.server.internalAuthToken"))
}

func validateProductionInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == localDevInternalAuthToken {
		return errors.New("grpc.server.internalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < minProductionInternalAuthTokenBytes {
		return fmt.Errorf("grpc.server.internalAuthToken must be at least %d bytes in production", minProductionInternalAuthTokenBytes)
	}
	return nil
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

var ProviderSet = wire.NewSet(New)
