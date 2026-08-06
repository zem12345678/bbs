package config

import (
	"bytes"
	"fmt"
	"mall-service/pkg/uuid"
	"os"
	"strings"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken           = "bbs-local-mall-internal-token"
	localDevCreditInternalAuthToken     = "bbs-local-credit-internal-token"
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
	configureEnv(v)
	if err := v.ReadInConfig(); err == nil {
		fmt.Printf("use config file -> %s\n", v.ConfigFileUsed())
	} else {
		return nil, errors.Wrap(err, "read config file error")
	}
	if !skipNacos() {
		if err = v.UnmarshalKey("nacos", o); err != nil {
			return nil, errors.Wrap(err, "unmarshal redis option error")
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
	applyEnvOverrides(v)
	setDefaults(v)
	if err := validate(v); err != nil {
		return nil, err
	}
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	setNestedConfigValue(v, "server.uuid", uuidstr)
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
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_MALL_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_MALL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_MALL_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_MALL_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_MALL_GRPC_SERVER_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_MALL_GRPC_SERVER_SERVICE_NAME", "BBS_MALL_SERVICE_NAME")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_MALL_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_MALL_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "postgres.dsn", "BBS_MALL_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_MALL_POSTGRES_DEBUG")
	bindEnv(v, "trace.env", "BBS_MALL_TRACE_ENV")
	bindEnv(v, "upstreams.credit", "BBS_MALL_UPSTREAMS_CREDIT")
	bindEnv(v, "upstreams.creditInternalAuthToken", "BBS_MALL_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func setDefaults(v *viper.Viper) {
	setStringDefault(v, "upstreams.credit", "bbs-credit-service")
	setStringDefault(v, "upstreams.creditInternalAuthToken", localDevCreditInternalAuthToken)
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
}

func validate(v *viper.Viper) error {
	switch strings.ToLower(strings.TrimSpace(v.GetString("trace.env"))) {
	case "prod", "production":
		token := strings.TrimSpace(v.GetString("grpc.server.internalAuthToken"))
		if token == "" || token == localDevInternalAuthToken {
			return errors.New("grpc.server.internalAuthToken must be set to a non-default value in production")
		}
		if len([]byte(token)) < minProductionInternalAuthTokenBytes {
			return fmt.Errorf("grpc.server.internalAuthToken must be at least %d bytes in production", minProductionInternalAuthTokenBytes)
		}
		creditToken := strings.TrimSpace(v.GetString("upstreams.creditInternalAuthToken"))
		if creditToken == "" || creditToken == localDevCreditInternalAuthToken {
			return errors.New("upstreams.creditInternalAuthToken must be set to a non-default value in production")
		}
		if len([]byte(creditToken)) < minProductionInternalAuthTokenBytes {
			return fmt.Errorf("upstreams.creditInternalAuthToken must be at least %d bytes in production", minProductionInternalAuthTokenBytes)
		}
	}
	return nil
}

func setStringDefault(v *viper.Viper, key string, fallback string) {
	if strings.TrimSpace(v.GetString(key)) == "" {
		setNestedConfigValue(v, key, fallback)
	}
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_MALL_POSTGRES_DSN")); value != "" {
		setNestedConfigValue(v, "postgres.dsn", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_MALL_POSTGRES_DEBUG")); value != "" {
		setNestedConfigValue(v, "postgres.debug", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_MALL_GRPC_SERVER_ETCD_ADDR")); value != "" {
		setNestedConfigValue(v, "grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_MALL_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		setNestedConfigValue(v, "grpc.client.etcdAddr", splitCommaSeparated(value))
	}
	if port := firstNonEmpty(os.Getenv("BBS_MALL_GRPC_SERVER_PORT"), os.Getenv("BBS_MALL_SERVICE_GRPC_PORT")); port != "" {
		setNestedConfigValue(v, "service.grpcPort", port)
		setNestedConfigValue(v, "grpc.server.port", port)
	}
	if name := firstNonEmpty(os.Getenv("BBS_MALL_GRPC_SERVER_SERVICE_NAME"), os.Getenv("BBS_MALL_SERVICE_NAME")); name != "" {
		setNestedConfigValue(v, "service.name", name)
		setNestedConfigValue(v, "grpc.server.serviceName", name)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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

// setNestedConfigValue writes value at a dotted key without dropping sibling keys.
//
// viper's Set publishes the value in the override layer, and that layer stores it as a
// partial nested map. A whole-subtree read such as UnmarshalKey("grpc.server", &o) finds
// the override subtree first and returns only the keys present there, silently discarding
// siblings that came from the config file, so writing a single leaf through Set would break
// unrelated settings. MergeConfigMap keeps siblings but writes to the config layer, which
// AutomaticEnv/BindEnv outrank, so a CSV list value would lose to the raw env string.
//
// Snapshot the whole top-level subtree through AllKeys/Get so every sibling keeps its fully
// resolved value (including env-provided ones), apply the new leaf, then republish the entire
// root in the override layer. Siblings survive and the write still wins over env bindings.
func setNestedConfigValue(v *viper.Viper, key string, value interface{}) {
	parts := strings.Split(strings.ToLower(key), ".")
	if len(parts) == 1 {
		v.Set(parts[0], value)
		return
	}
	root := parts[0]
	prefix := root + "."

	tree := map[string]interface{}{}
	for _, full := range v.AllKeys() {
		if !strings.HasPrefix(full, prefix) {
			continue
		}
		assignNestedConfigValue(tree, strings.Split(strings.TrimPrefix(full, prefix), "."), v.Get(full))
	}
	assignNestedConfigValue(tree, parts[1:], value)
	v.Set(root, tree)
}

// assignNestedConfigValue writes value into tree at path, creating intermediate maps.
func assignNestedConfigValue(tree map[string]interface{}, path []string, value interface{}) {
	node := tree
	for _, segment := range path[:len(path)-1] {
		next, ok := node[segment].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			node[segment] = next
		}
		node = next
	}
	node[path[len(path)-1]] = value
}
