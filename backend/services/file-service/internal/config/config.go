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

const (
	localDevMallInternalAuthToken              = "bbs-local-mall-internal-token"
	minProductionMallInternalAuthTokenBytes    = 32
	localDevCreditInternalAuthToken            = "bbs-local-credit-internal-token"
	minProductionCreditInternalAuthTokenBytes  = 32
	localDevContentInternalAuthToken           = "bbs-local-content-internal-token"
	minProductionContentInternalAuthTokenBytes = 32
	localDevFileInternalAuthToken              = "bbs-local-file-internal-token"
	minProductionFileInternalAuthTokenBytes    = 32
	defaultFileCapacityBytes                   = int64(100 << 20)
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

	if !skipNacos() {
		var nacos nacosOptions
		if err := v.UnmarshalKey("nacos", &nacos); err != nil {
			return nil, fmt.Errorf("read nacos config: %w", err)
		}
		if nacos.enabled() {
			if err := mergeNacosConfig(v, nacos); err != nil {
				return nil, err
			}
		}
	}
	applyEnvOverrides(v)
	setDefaults(v)
	if err := validate(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (o nacosOptions) enabled() bool {
	return strings.TrimSpace(o.Addr) != "" && o.Port > 0 && strings.TrimSpace(o.DataID) != ""
}

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_FILE_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
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
		"service.name":                       {"BBS_FILE_SERVICE_NAME"},
		"service.grpcPort":                   {"BBS_FILE_SERVICE_GRPC_PORT"},
		"postgres.dsn":                       {"BBS_FILE_POSTGRES_DSN"},
		"files.capacityBytes":                {"BBS_FILE_FILES_CAPACITY_BYTES"},
		"upstreams.credit":                   {"BBS_FILE_UPSTREAMS_CREDIT"},
		"upstreams.creditInternalAuthToken":  {"BBS_FILE_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN"},
		"upstreams.content":                  {"BBS_FILE_UPSTREAMS_CONTENT"},
		"upstreams.contentInternalAuthToken": {"BBS_FILE_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN"},
		"upstreams.mall":                     {"BBS_FILE_UPSTREAMS_MALL"},
		"upstreams.mallInternalAuthToken":    {"BBS_FILE_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN"},
		"trace.env":                          {"BBS_FILE_TRACE_ENV"},
		"grpc.server.port":                   {"BBS_FILE_GRPC_SERVER_PORT", "BBS_FILE_SERVICE_GRPC_PORT"},
		"grpc.server.serviceName":            {"BBS_FILE_GRPC_SERVER_SERVICE_NAME", "BBS_FILE_SERVICE_NAME"},
		"grpc.server.internalAuthToken":      {"BBS_FILE_GRPC_SERVER_INTERNAL_AUTH_TOKEN"},
		"grpc.client.etcdAddr":               {"BBS_FILE_GRPC_CLIENT_ETCD_ADDR"},
		"grpc.server.etcdAddr":               {"BBS_FILE_GRPC_SERVER_ETCD_ADDR"},
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
	if v.GetInt64("files.capacityBytes") <= 0 {
		v.Set("files.capacityBytes", defaultFileCapacityBytes)
	}
	setString(v, "upstreams.credit", "bbs-credit-service")
	setString(v, "upstreams.creditInternalAuthToken", localDevCreditInternalAuthToken)
	setString(v, "upstreams.content", "bbs-content-service")
	setString(v, "upstreams.contentInternalAuthToken", localDevContentInternalAuthToken)
	setString(v, "upstreams.mall", "bbs-mall-service")
	setString(v, "upstreams.mallInternalAuthToken", localDevMallInternalAuthToken)
	if v.GetInt("grpc.server.port") == 0 {
		v.Set("grpc.server.port", v.GetInt("service.grpcPort"))
	}
	setString(v, "grpc.server.serviceName", v.GetString("service.name"))
	setString(v, "grpc.server.internalAuthToken", localDevFileInternalAuthToken)
	if len(v.GetStringSlice("grpc.server.etcdAddr")) == 0 {
		v.Set("grpc.server.etcdAddr", []string{"127.0.0.1:2379"})
	}
	if len(v.GetStringSlice("grpc.client.etcdAddr")) == 0 {
		v.Set("grpc.client.etcdAddr", v.GetStringSlice("grpc.server.etcdAddr"))
	}
}

func validate(v *viper.Viper) error {
	switch strings.ToLower(strings.TrimSpace(v.GetString("trace.env"))) {
	case "prod", "production":
		token := strings.TrimSpace(v.GetString("upstreams.mallInternalAuthToken"))
		if token == "" || token == localDevMallInternalAuthToken {
			return fmt.Errorf("upstreams.mallInternalAuthToken must be set to a non-default value in production")
		}
		if len([]byte(token)) < minProductionMallInternalAuthTokenBytes {
			return fmt.Errorf("upstreams.mallInternalAuthToken must be at least %d bytes in production", minProductionMallInternalAuthTokenBytes)
		}
		token = strings.TrimSpace(v.GetString("upstreams.creditInternalAuthToken"))
		if token == "" || token == localDevCreditInternalAuthToken {
			return fmt.Errorf("upstreams.creditInternalAuthToken must be set to a non-default value in production")
		}
		if len([]byte(token)) < minProductionCreditInternalAuthTokenBytes {
			return fmt.Errorf("upstreams.creditInternalAuthToken must be at least %d bytes in production", minProductionCreditInternalAuthTokenBytes)
		}
		token = strings.TrimSpace(v.GetString("grpc.server.internalAuthToken"))
		if token == "" || token == localDevFileInternalAuthToken {
			return fmt.Errorf("grpc.server.internalAuthToken must be set to a non-default value in production")
		}
		if len([]byte(token)) < minProductionFileInternalAuthTokenBytes {
			return fmt.Errorf("grpc.server.internalAuthToken must be at least %d bytes in production", minProductionFileInternalAuthTokenBytes)
		}
		token = strings.TrimSpace(v.GetString("upstreams.contentInternalAuthToken"))
		if token == "" || token == localDevContentInternalAuthToken {
			return fmt.Errorf("upstreams.contentInternalAuthToken must be set to a non-default value in production")
		}
		if len([]byte(token)) < minProductionContentInternalAuthTokenBytes {
			return fmt.Errorf("upstreams.contentInternalAuthToken must be at least %d bytes in production", minProductionContentInternalAuthTokenBytes)
		}
		for _, key := range []string{"storage.bucket", "storage.accessKey", "storage.secretKey"} {
			if strings.TrimSpace(v.GetString(key)) == "" {
				return fmt.Errorf("%s is required in production", key)
			}
		}
	}
	return nil
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
