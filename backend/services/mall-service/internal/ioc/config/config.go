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
	applyEnvOverrides(v)
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

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_MALL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.grpcPort", "BBS_MALL_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_MALL_GRPC_SERVER_PORT")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if port := firstNonEmpty(os.Getenv("BBS_MALL_GRPC_SERVER_PORT"), os.Getenv("BBS_MALL_SERVICE_GRPC_PORT")); port != "" {
		v.Set("service.grpcPort", port)
		v.Set("grpc.server.port", port)
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

var ProviderSet = wire.NewSet(New)
