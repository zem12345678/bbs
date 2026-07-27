package config

import (
	"bytes"
	"comment-service/pkg/uuid"
	"fmt"
	"os"
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
	localDevInternalAuthToken           = "bbs-local-comment-internal-token"
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
	configureEnv(v)
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
	}
	if err = applyEnvOverrides(v); err != nil {
		return nil, errors.Wrap(err, "apply environment overrides")
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
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_COMMENT_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_COMMENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnv(v, "service.name", "BBS_COMMENT_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_COMMENT_SERVICE_GRPC_PORT")
	bindEnv(v, "app.name", "BBS_COMMENT_APP_NAME", "BBS_COMMENT_SERVICE_NAME")
	bindEnv(v, "mongo.uri", "BBS_COMMENT_MONGO_URI")
	bindEnv(v, "mongo.username", "BBS_COMMENT_MONGO_USERNAME")
	bindEnv(v, "mongo.password", "BBS_COMMENT_MONGO_PASSWORD")
	bindEnv(v, "mongo.endpoints", "BBS_COMMENT_MONGO_ENDPOINTS")
	bindEnv(v, "mongo.authDB", "BBS_COMMENT_MONGO_AUTH_DB")
	bindEnv(v, "mongo.database", "BBS_COMMENT_MONGO_DATABASE")
	bindEnv(v, "mongo.enableTrace", "BBS_COMMENT_MONGO_ENABLE_TRACE")
	bindEnv(v, "kafka.brokers", "BBS_COMMENT_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_COMMENT_KAFKA_TOPIC")
	bindEnv(v, "kafka.username", "BBS_COMMENT_KAFKA_USERNAME")
	bindEnv(v, "kafka.password", "BBS_COMMENT_KAFKA_PASSWORD")
	bindEnv(v, "kafka.scram_algorithm", "BBS_COMMENT_KAFKA_SCRAM_ALGORITHM")
	bindEnv(v, "snowflake.workerId", "BBS_COMMENT_SNOWFLAKE_WORKER_ID")
	bindEnv(v, "grpc.server.port", "BBS_COMMENT_GRPC_SERVER_PORT", "BBS_COMMENT_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_COMMENT_GRPC_SERVER_SERVICE_NAME", "BBS_COMMENT_SERVICE_NAME")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_COMMENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_COMMENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "grpc.server.etcdAddr", "BBS_COMMENT_GRPC_SERVER_ETCD_ADDR")
	bindEnv(v, "grpc.client.etcdAddr", "BBS_COMMENT_GRPC_CLIENT_ETCD_ADDR")
	bindEnv(v, "trace.grpcEndpoint", "BBS_COMMENT_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.serviceName", "BBS_COMMENT_TRACE_SERVICE_NAME", "BBS_COMMENT_SERVICE_NAME")
	bindEnv(v, "trace.version", "BBS_COMMENT_TRACE_VERSION")
	bindEnv(v, "trace.env", "BBS_COMMENT_TRACE_ENV")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) error {
	overrides := map[string]interface{}{}
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_MONGO_URI")); value != "" {
		overrides["mongo"] = map[string]interface{}{"uri": value}
	}
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_MONGO_ENDPOINTS")); value != "" {
		mergeOverride(overrides, "mongo", "endpoints", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_MONGO_DATABASE")); value != "" {
		mergeOverride(overrides, "mongo", "database", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_KAFKA_BROKERS")); value != "" {
		overrides["kafka"] = map[string]interface{}{"brokers": splitCommaSeparated(value)}
	}
	grpcOverrides := map[string]interface{}{}
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_GRPC_SERVER_ETCD_ADDR")); value != "" {
		grpcOverrides["server"] = map[string]interface{}{"etcdAddr": splitCommaSeparated(value)}
	}
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		grpcOverrides["client"] = map[string]interface{}{"etcdAddr": splitCommaSeparated(value)}
	}
	if len(grpcOverrides) > 0 {
		overrides["grpc"] = grpcOverrides
	}
	if len(overrides) > 0 {
		if err := v.MergeConfigMap(overrides); err != nil {
			return err
		}
	}
	setStringEnv(v, "service.name", "BBS_COMMENT_SERVICE_NAME")
	setStringEnv(v, "app.name", "BBS_COMMENT_APP_NAME")
	setStringEnv(v, "mongo.username", "BBS_COMMENT_MONGO_USERNAME")
	setStringEnv(v, "mongo.password", "BBS_COMMENT_MONGO_PASSWORD")
	setStringEnv(v, "mongo.authDB", "BBS_COMMENT_MONGO_AUTH_DB")
	setStringEnv(v, "mongo.enableTrace", "BBS_COMMENT_MONGO_ENABLE_TRACE")
	setStringEnv(v, "kafka.topic", "BBS_COMMENT_KAFKA_TOPIC")
	setStringEnv(v, "kafka.username", "BBS_COMMENT_KAFKA_USERNAME")
	setStringEnv(v, "kafka.password", "BBS_COMMENT_KAFKA_PASSWORD")
	setStringEnv(v, "kafka.scram_algorithm", "BBS_COMMENT_KAFKA_SCRAM_ALGORITHM")
	setStringEnv(v, "snowflake.workerId", "BBS_COMMENT_SNOWFLAKE_WORKER_ID")
	if value := firstNonEmptyEnv("BBS_COMMENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_COMMENT_INTERNAL_AUTH_TOKEN"); value != "" {
		v.Set("grpc.server.internalAuthToken", value)
	}
	setStringEnv(v, "trace.grpcEndpoint", "BBS_COMMENT_TRACE_GRPC_ENDPOINT")
	setStringEnv(v, "trace.serviceName", "BBS_COMMENT_TRACE_SERVICE_NAME")
	setStringEnv(v, "trace.version", "BBS_COMMENT_TRACE_VERSION")
	setStringEnv(v, "trace.env", "BBS_COMMENT_TRACE_ENV")
	return applyGRPCPortEnvOverride(v,
		"BBS_COMMENT_GRPC_SERVER_PORT",
		"BBS_COMMENT_SERVICE_GRPC_PORT",
	)
}

func mergeOverride(overrides map[string]interface{}, section string, key string, value interface{}) {
	existing, _ := overrides[section].(map[string]interface{})
	if existing == nil {
		existing = map[string]interface{}{}
		overrides[section] = existing
	}
	existing[key] = value
}

func setStringEnv(v *viper.Viper, key string, env string) {
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		v.Set(key, value)
	}
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
