package config

import (
	"admin/pkg/uuid"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
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
			_ = v.MergeConfig(bytes.NewBufferString(data))
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "listenConfig nacos config error")
	}
	applyEnvOverrides(v)
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	setDefaults(v)
	v.Set("server.uuid", uuidstr)
	return v, err
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_ADMIN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "auth.jwtSecret", "BBS_ADMIN_AUTH_JWT_SECRET")
	bindEnv(v, "auth.jwtTtl", "BBS_ADMIN_AUTH_JWT_TTL")
	bindEnv(v, "auth.defaultAdminPassword", "BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD")
	bindEnv(v, "auth.secretEncryptionKey", "BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY")
	bindEnv(v, "upstreams.user", "BBS_ADMIN_UPSTREAMS_USER")
	bindEnv(v, "upstreams.reaction", "BBS_ADMIN_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.content", "BBS_ADMIN_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.comment", "BBS_ADMIN_UPSTREAMS_COMMENT")
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_ADMIN_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_ADMIN_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
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

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func setDefaults(v *viper.Viper) {
	setStringDefault(v, "auth.jwtSecret", "bbs-admin-local-dev-secret")
	setStringDefault(v, "auth.jwtTtl", "168h")
	setStringDefault(v, "auth.defaultAdminPassword", "Admin123!")
	setStringDefault(v, "auth.secretEncryptionKey", "bbs-admin-local-setting-secret")
	setStringDefault(v, "upstreams.user", "bbs-user-service")
	setStringDefault(v, "upstreams.reaction", "bbs-reaction-service")
	setStringDefault(v, "upstreams.content", "bbs-content-service")
	setStringDefault(v, "upstreams.comment", "bbs-comment-service")
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
