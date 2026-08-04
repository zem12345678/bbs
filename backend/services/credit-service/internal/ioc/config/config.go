package config

import (
	"bytes"
	"credit-service/pkg/uuid"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken           = "bbs-local-credit-internal-token"
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
	v := viper.New()
	configureEnv(v)
	v.AddConfigPath(".")
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, errors.Wrap(err, "read config file error")
	}
	fmt.Printf("use config file -> %s\n", v.ConfigFileUsed())

	if !skipNacos() {
		var nacosOptions Options
		if err := v.UnmarshalKey("nacos", &nacosOptions); err != nil {
			return nil, errors.Wrap(err, "unmarshal nacos option error")
		}
		if nacosOptions.enabled() {
			if err := readNacosConfig(v, nacosOptions); err != nil {
				return nil, err
			}
		}
	}

	applyEnvOverrides(v)
	if err := applyGRPCPortEnvOverride(v,
		"BBS_CREDIT_GRPC_SERVER_PORT",
		"BBS_CREDIT_SERVICE_GRPC_PORT",
	); err != nil {
		return nil, err
	}
	setDefaults(v)
	if err := validate(v); err != nil {
		return nil, err
	}
	if err := setHostUUID(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (o Options) enabled() bool {
	return strings.TrimSpace(o.Addr) != "" && o.Port != 0 && strings.TrimSpace(o.DataID) != ""
}

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_CREDIT_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func readNacosConfig(v *viper.Viper, o Options) error {
	group := stringDefault(o.GroupID, "DEFAULT_GROUP")
	configClient, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": []constant.ServerConfig{{IpAddr: o.Addr, Port: o.Port}},
		"clientConfig": constant.ClientConfig{
			NamespaceId:         o.NamespaceID,
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			LogDir:              "tmp/nacos/log",
			CacheDir:            "tmp/nacos/cache",
			LogLevel:            "debug",
		},
	})
	if err != nil {
		return err
	}
	content, err := configClient.GetConfig(vo.ConfigParam{
		DataId: o.DataID,
		Group:  group,
	})
	if err != nil {
		return err
	}
	if err := v.MergeConfig(bytes.NewBufferString(content)); err != nil {
		return errors.Wrap(err, "viper read nacos config error")
	}
	if err := configClient.ListenConfig(vo.ConfigParam{
		DataId: o.DataID,
		Group:  group,
		OnChange: func(namespace, group, dataID, data string) {
			_ = v.MergeConfig(bytes.NewBufferString(data))
		},
	}); err != nil {
		return errors.Wrap(err, "listenConfig nacos config error")
	}
	return nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_CREDIT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_CREDIT_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_CREDIT_SERVICE_GRPC_PORT")
	bindEnv(v, "app.name", "BBS_CREDIT_APP_NAME")
	bindEnv(v, "postgres.dsn", "BBS_CREDIT_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_CREDIT_POSTGRES_DEBUG")
	bindEnv(v, "redis.addr", "BBS_CREDIT_REDIS_ADDR")
	bindEnv(v, "redis.url", "BBS_CREDIT_REDIS_URL", "BBS_CREDIT_REDIS_ADDR")
	bindEnv(v, "redis.db", "BBS_CREDIT_REDIS_DB")
	bindEnv(v, "redis.dbNum", "BBS_CREDIT_REDIS_DB_NUM", "BBS_CREDIT_REDIS_DB")
	bindEnv(v, "redis.password", "BBS_CREDIT_REDIS_PASSWORD")
	bindEnv(v, "kafka.brokers", "BBS_CREDIT_KAFKA_BROKERS")
	bindEnv(v, "kafka.userTopic", "BBS_CREDIT_KAFKA_USER_TOPIC")
	bindEnv(v, "kafka.articleTopic", "BBS_CREDIT_KAFKA_ARTICLE_TOPIC")
	bindEnv(v, "kafka.commentTopic", "BBS_CREDIT_KAFKA_COMMENT_TOPIC")
	bindEnv(v, "kafka.reactionTopic", "BBS_CREDIT_KAFKA_REACTION_TOPIC")
	bindEnv(v, "kafka.userGroupId", "BBS_CREDIT_KAFKA_USER_GROUP_ID")
	bindEnv(v, "kafka.articleGroupId", "BBS_CREDIT_KAFKA_ARTICLE_GROUP_ID")
	bindEnv(v, "kafka.commentGroupId", "BBS_CREDIT_KAFKA_COMMENT_GROUP_ID")
	bindEnv(v, "kafka.reactionGroupId", "BBS_CREDIT_KAFKA_REACTION_GROUP_ID")
	bindEnv(v, "kafka.username", "BBS_CREDIT_KAFKA_USERNAME")
	bindEnv(v, "kafka.password", "BBS_CREDIT_KAFKA_PASSWORD")
	bindEnv(v, "kafka.scram_algorithm", "BBS_CREDIT_KAFKA_SCRAM_ALGORITHM")
	bindEnv(v, "grpc.server.port", "BBS_CREDIT_GRPC_SERVER_PORT", "BBS_CREDIT_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_CREDIT_GRPC_SERVER_SERVICE_NAME", "BBS_CREDIT_SERVICE_NAME")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_CREDIT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_CREDIT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "trace.grpcEndpoint", "BBS_CREDIT_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.env", "BBS_CREDIT_TRACE_ENV")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyGRPCPortEnvOverride(v *viper.Viper, names ...string) error {
	value := firstNonEmptyEnv(names...)
	if value == "" {
		return nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid gRPC port override %q", value)
	}
	v.Set("service.grpcPort", port)
	v.Set("grpc.server.port", port)
	return nil
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_CREDIT_POSTGRES_DSN")); value != "" {
		v.Set("postgres.dsn", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CREDIT_POSTGRES_DEBUG")); value != "" {
		v.Set("postgres.debug", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CREDIT_KAFKA_BROKERS")); value != "" {
		v.Set("kafka.brokers", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CREDIT_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CREDIT_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
	}
}

func setDefaults(v *viper.Viper) {
	serviceName := stringDefault(v.GetString("service.name"), "bbs-credit-service")
	servicePort := v.GetInt("service.grpcPort")
	if servicePort == 0 {
		servicePort = 9107
	}
	v.Set("service.name", serviceName)
	v.Set("service.grpcPort", servicePort)
	setStringDefault(v, "app.name", serviceName)

	setStringDefault(v, "log.filename", "logs/credit-service.log")
	setIntDefault(v, "log.maxSize", 100)
	setIntDefault(v, "log.maxBackups", 7)
	setIntDefault(v, "log.maxAge", 30)
	setStringDefault(v, "log.level", "info")
	if !v.IsSet("log.stdout") {
		v.Set("log.stdout", true)
	}

	setStringDefault(v, "postgres.dsn", "postgres://bbs_credit_app:local_credit_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_credit")
	redisURL := stringDefault(v.GetString("redis.url"), v.GetString("redis.addr"))
	setStringDefault(v, "redis.url", stringDefault(redisURL, "127.0.0.1:6379"))
	setStringDefault(v, "redis.addr", v.GetString("redis.url"))
	setIntDefault(v, "redis.maxIdle", 10)
	setIntDefault(v, "redis.maxActive", 100)
	setIntDefault(v, "redis.idleTimeout", 10)
	setIntDefault(v, "redis.timeout", 5)
	setStringDefault(v, "redis.network", "tcp")
	if len(v.GetStringSlice("kafka.brokers")) == 0 {
		v.Set("kafka.brokers", []string{"127.0.0.1:9092"})
	}
	setStringDefault(v, "kafka.userTopic", "user.events")
	setStringDefault(v, "kafka.articleTopic", "article.events")
	setStringDefault(v, "kafka.commentTopic", "comment.events")
	setStringDefault(v, "kafka.reactionTopic", "reaction.events")
	setStringDefault(v, "kafka.userGroupId", "bbs-credit-user-consumer")
	setStringDefault(v, "kafka.articleGroupId", "bbs-credit-article-consumer")
	setStringDefault(v, "kafka.commentGroupId", "bbs-credit-comment-consumer")
	setStringDefault(v, "kafka.reactionGroupId", "bbs-credit-reaction-consumer")

	if v.GetInt("grpc.server.port") == 0 {
		v.Set("grpc.server.port", servicePort)
	}
	setStringDefault(v, "grpc.server.serviceName", serviceName)
	if len(v.GetStringSlice("grpc.server.etcdAddr")) == 0 {
		v.Set("grpc.server.etcdAddr", []string{"127.0.0.1:2379"})
	}
	if v.GetDuration("grpc.server.timeout") <= 0 {
		v.Set("grpc.server.timeout", 10*time.Second)
	}
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
	if v.GetDuration("grpc.client.timeout") <= 0 {
		v.Set("grpc.client.timeout", 10*time.Second)
	}
	setStringDefault(v, "grpc.client.tag", "credit")
	setStringDefault(v, "grpc.client.serverName", serviceName)
	if len(v.GetStringSlice("grpc.client.etcdAddr")) == 0 {
		v.Set("grpc.client.etcdAddr", v.GetStringSlice("grpc.server.etcdAddr"))
	}
	if !v.IsSet("grpc.client.secure") {
		v.Set("grpc.client.secure", false)
	}

	setStringDefault(v, "trace.grpcEndpoint", "127.0.0.1:4317")
	setStringDefault(v, "trace.serviceName", serviceName)
	setStringDefault(v, "trace.version", "local")
	setStringDefault(v, "trace.env", "local")
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

func setHostUUID(v *viper.Viper) error {
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		uuidstr, err = uuid.NewUUID()
	}
	v.Set("server.uuid", uuidstr)
	return err
}

func setStringDefault(v *viper.Viper, key string, fallback string) {
	if strings.TrimSpace(v.GetString(key)) == "" {
		v.Set(key, fallback)
	}
}

func setIntDefault(v *viper.Viper, key string, fallback int) {
	if v.GetInt(key) == 0 {
		v.Set(key, fallback)
	}
}

func stringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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

var ProviderSet = wire.NewSet(New)
