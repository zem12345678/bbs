package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"user-service/pkg/uuid"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken           = "bbs-local-user-internal-token"
	minProductionInternalAuthTokenBytes = 32
	localDevMallInternalAuthToken       = "bbs-local-mall-internal-token"
	minProductionMallInternalAuthBytes  = 32
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
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	setDefaults(v)
	if err := validate(v); err != nil {
		return nil, err
	}
	v.Set("server.uuid", uuidstr)
	return v, err
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_USER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.grpcPort", "BBS_USER_SERVICE_GRPC_PORT", "BBS_USER_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_USER_GRPC_SERVER_PORT", "BBS_USER_GRPC_PORT", "BBS_USER_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_USER_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_USER_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "trace.env", "BBS_USER_TRACE_ENV")
	bindEnv(v, "upstreams.mall", "BBS_USER_UPSTREAMS_MALL")
	bindEnv(v, "upstreams.mallInternalAuthToken", "BBS_USER_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "mail.enabled", "BBS_USER_MAIL_ENABLED")
	bindEnv(v, "mail.smtpAddr", "BBS_USER_MAIL_SMTP_ADDR")
	bindEnv(v, "mail.username", "BBS_USER_MAIL_USERNAME")
	bindEnv(v, "mail.password", "BBS_USER_MAIL_PASSWORD")
	bindEnv(v, "mail.from", "BBS_USER_MAIL_FROM")
	bindEnv(v, "mail.frontendBaseURL", "BBS_USER_MAIL_FRONTEND_BASE_URL")
	bindEnv(v, "mail.tlsMode", "BBS_USER_MAIL_TLS_MODE")
	bindEnv(v, "mail.timeout", "BBS_USER_MAIL_TIMEOUT")
	bindEnv(v, "redis.url", "BBS_USER_REDIS_URL")
	bindEnv(v, "redis.password", "BBS_USER_REDIS_PASSWORD")
	bindEnv(v, "redis.dbNum", "BBS_USER_REDIS_DB_NUM")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func setDefaults(v *viper.Viper) {
	setStringDefault(v, "upstreams.mall", "bbs-mall-service")
	setStringDefault(v, "upstreams.mallInternalAuthToken", localDevMallInternalAuthToken)
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
}

func validate(v *viper.Viper) error {
	environment := strings.ToLower(strings.TrimSpace(v.GetString("trace.env")))
	if environment != "production" && environment != "prod" {
		return nil
	}
	token := strings.TrimSpace(v.GetString("grpc.server.internalAuthToken"))
	if token == "" || token == localDevInternalAuthToken {
		return fmt.Errorf("grpc.server.internalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < minProductionInternalAuthTokenBytes {
		return fmt.Errorf("grpc.server.internalAuthToken must be at least %d bytes in production", minProductionInternalAuthTokenBytes)
	}
	mallToken := strings.TrimSpace(v.GetString("upstreams.mallInternalAuthToken"))
	if mallToken == "" || mallToken == localDevMallInternalAuthToken {
		return fmt.Errorf("upstreams.mallInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(mallToken)) < minProductionMallInternalAuthBytes {
		return fmt.Errorf("upstreams.mallInternalAuthToken must be at least %d bytes in production", minProductionMallInternalAuthBytes)
	}
	return nil
}

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_USER_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func applyEnvOverrides(v *viper.Viper) error {
	overrides := map[string]interface{}{}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_KAFKA_BROKERS")); value != "" {
		overrides["kafka"] = map[string]interface{}{"brokers": splitCommaSeparated(value)}
	}
	grpcOverrides := map[string]interface{}{}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_GRPC_SERVER_ETCD_ADDR")); value != "" {
		grpcOverrides["server"] = map[string]interface{}{"etcdAddr": splitCommaSeparated(value)}
	}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		grpcOverrides["client"] = map[string]interface{}{"etcdAddr": splitCommaSeparated(value)}
	}
	if len(grpcOverrides) > 0 {
		overrides["grpc"] = grpcOverrides
	}
	if len(overrides) == 0 {
		return applyGRPCPortOverride(v)
	}
	if err := v.MergeConfigMap(overrides); err != nil {
		return err
	}
	return applyGRPCPortOverride(v)
}

func applyGRPCPortOverride(v *viper.Viper) error {
	value := firstEnv("BBS_USER_GRPC_SERVER_PORT", "BBS_USER_GRPC_PORT", "BBS_USER_SERVICE_GRPC_PORT")
	if value == "" {
		return nil
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parse user gRPC server port: %w", err)
	}
	if port <= 0 {
		return fmt.Errorf("user gRPC server port must be greater than zero: %d", port)
	}
	v.Set("service.grpcPort", port)
	v.Set("grpc.server.port", port)
	return nil
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
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

func setStringDefault(v *viper.Viper, key string, fallback string) {
	if strings.TrimSpace(v.GetString(key)) == "" {
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

var ProviderSet = wire.NewSet(New)
