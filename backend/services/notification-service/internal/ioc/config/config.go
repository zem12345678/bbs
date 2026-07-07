package config

import (
	"notification-service/pkg/uuid"
	"bytes"
	"fmt"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Options struct {
	Addr        string `toml:"addr" json:"addr" yaml:"addr" env:"NACOS_ADDR"`
	Port        uint64 `toml:"port" json:"port" yaml:"port" env:"NACOS_PORT"`
	NamespaceID string `toml:"namespaceId" json:"namespaceId" yaml:"namespaceId" env:"NACOS_NAMESPACEID"`
	DataID      string `toml:"dataId" json:"dataId" yaml:"dataId" env:"NACOS_DATAID"`
	GroupID     string `toml:"groupId" json:"groupId" yaml:"password" env:"NACOS_GROUPID"`
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
	if err = v.UnmarshalKey("nacos", o); err != nil {
		return nil, errors.Wrap(err, "unmarshal redis option error")
	}

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
		Group:  o.GroupID})

	if err != nil {
		return nil, err
	}
	err = v.ReadConfig(bytes.NewBufferString(content))

	if err != nil {
		return nil, errors.Wrap(err, "viper read nacos config error")
	}

	err = configClient.ListenConfig(vo.ConfigParam{
		DataId: o.DataID,
		Group:  o.GroupID,
		OnChange: func(namespace, group, dataId, data string) {
			//获取配置
			_ = v.ReadConfig(bytes.NewBufferString(content))

		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "listenConfig nacos config error")
	}
	//go func(error, *viper.Viper) *viper.Viper {
	//	for {
	//		err = configClient.ListenConfig(vo.ConfigParam{
	//			DataId: o.DataID,
	//			Group:  o.GroupID,
	//			OnChange: func(namespace, group, dataId, data string) {
	//				//获取配置
	//				_ = v.ReadConfig(bytes.NewBufferString(content))
	//
	//			},
	//		})
	//	}
	//
	//}(err, v)
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	v.Set("server.uuid", uuidstr)
	fmt.Println(3333, v.GetEnvPrefix())
	return v, err
}

var ProviderSet = wire.NewSet(New)
