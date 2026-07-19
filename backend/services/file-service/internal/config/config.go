package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/viper"
)

type nacosOptions struct {
	Addr        string `mapstructure:"addr"`
	Port        uint64 `mapstructure:"port"`
	NamespaceID string `mapstructure:"namespaceId"`
	DataID      string `mapstructure:"dataId"`
	GroupID     string `mapstructure:"groupId"`
}

func Load(path string) (*viper.Viper, error) {
	v := viper.New()
	configureEnv(v)
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var nacos nacosOptions
	if err := v.UnmarshalKey("nacos", &nacos); err != nil {
		return nil, fmt.Errorf("read nacos config: %w", err)
	}
	if nacos.enabled() {
		if err := mergeNacosConfig(v, nacos); err != nil {
			return nil, err
		}
	}
	applyEnvOverrides(v)
	setDefaults(v)
	return v, nil
}

func (o nacosOptions) enabled() bool {
	return strings.TrimSpace(o.Addr) != "" && o.Port > 0 && strings.TrimSpace(o.DataID) != ""
}

func mergeNacosConfig(v *viper.Viper, o nacosOptions) error {
	group := strings.TrimSpace(o.GroupID)
	if group == "" {
		group = "DEFAULT_GROUP"
	}
	client, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": []constant.ServerConfig{{IpAddr: o.Addr, Port: o.Port}},
		"clientConfig": constant.ClientConfig{
			NamespaceId:         o.NamespaceID,
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			LogDir:              "tmp/nacos/log",
			CacheDir:            "tmp/nacos/cache",
			LogLevel:            "warn",
		},
	})
	if err != nil {
		return fmt.Errorf("create nacos client: %w", err)
	}
	content, err := client.GetConfig(vo.ConfigParam{DataId: o.DataID, Group: group})
	if err != nil {
		return fmt.Errorf("load nacos config: %w", err)
	}
	if err := v.MergeConfig(bytes.NewBufferString(content)); err != nil {
		return fmt.Errorf("merge nacos config: %w", err)
	}
	return client.ListenConfig(vo.ConfigParam{
		DataId: o.DataID,
		Group:  group,
		OnChange: func(_, _, _, data string) {
			_ = v.MergeConfig(bytes.NewBufferString(data))
		},
	})
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_FILE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for key, aliases := range map[string][]string{
		"service.name":            {"BBS_FILE_SERVICE_NAME"},
		"service.grpcPort":        {"BBS_FILE_SERVICE_GRPC_PORT"},
		"postgres.dsn":            {"BBS_FILE_POSTGRES_DSN"},
		"upstreams.credit":        {"BBS_FILE_UPSTREAMS_CREDIT"},
		"upstreams.mall":          {"BBS_FILE_UPSTREAMS_MALL"},
		"grpc.server.port":        {"BBS_FILE_GRPC_SERVER_PORT", "BBS_FILE_SERVICE_GRPC_PORT"},
		"grpc.server.serviceName": {"BBS_FILE_GRPC_SERVER_SERVICE_NAME", "BBS_FILE_SERVICE_NAME"},
		"grpc.client.etcdAddr":    {"BBS_FILE_GRPC_CLIENT_ETCD_ADDR"},
		"grpc.server.etcdAddr":    {"BBS_FILE_GRPC_SERVER_ETCD_ADDR"},
	} {
		_ = v.BindEnv(append([]string{key}, aliases...)...)
	}
}

func applyEnvOverrides(v *viper.Viper) {
	for _, item := range []struct {
		key string
		env string
	}{
		{"grpc.server.etcdAddr", "BBS_FILE_GRPC_SERVER_ETCD_ADDR"},
		{"grpc.client.etcdAddr", "BBS_FILE_GRPC_CLIENT_ETCD_ADDR"},
	} {
		if value := strings.TrimSpace(os.Getenv(item.env)); value != "" {
			v.Set(item.key, splitCommaSeparated(value))
		}
	}
}

func setDefaults(v *viper.Viper) {
	setString(v, "service.name", "bbs-file-service")
	if v.GetInt("service.grpcPort") == 0 {
		v.Set("service.grpcPort", 9111)
	}
	setString(v, "postgres.dsn", "postgres://bbs_file_app:local_file_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_file")
	setString(v, "upstreams.credit", "bbs-credit-service")
	setString(v, "upstreams.mall", "bbs-mall-service")
	if v.GetInt("grpc.server.port") == 0 {
		v.Set("grpc.server.port", v.GetInt("service.grpcPort"))
	}
	setString(v, "grpc.server.serviceName", v.GetString("service.name"))
	if len(v.GetStringSlice("grpc.server.etcdAddr")) == 0 {
		v.Set("grpc.server.etcdAddr", []string{"127.0.0.1:2379"})
	}
	if len(v.GetStringSlice("grpc.client.etcdAddr")) == 0 {
		v.Set("grpc.client.etcdAddr", v.GetStringSlice("grpc.server.etcdAddr"))
	}
}

func setString(v *viper.Viper, key, fallback string) {
	if strings.TrimSpace(v.GetString(key)) == "" {
		v.Set(key, fallback)
	}
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
