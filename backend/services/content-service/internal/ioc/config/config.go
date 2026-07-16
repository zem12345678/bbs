package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"content-service/pkg/uuid"

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
	GroupID     string `toml:"groupId" json:"groupId" yaml:"groupId" env:"NACOS_GROUPID"`
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

	var nacosOptions Options
	if err := v.UnmarshalKey("nacos", &nacosOptions); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos option error")
	}
	if nacosOptions.enabled() {
		if err := readNacosConfig(v, nacosOptions); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(v)
	setDefaults(v)
	if err := setHostUUID(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (o Options) enabled() bool {
	return strings.TrimSpace(o.Addr) != "" && o.Port != 0 && strings.TrimSpace(o.DataID) != ""
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
	content, err := configClient.GetConfig(vo.ConfigParam{DataId: o.DataID, Group: group})
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
	v.SetEnvPrefix("BBS_CONTENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_CONTENT_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_CONTENT_SERVICE_GRPC_PORT")
	bindEnv(v, "app.name", "BBS_CONTENT_APP_NAME")
	bindEnv(v, "postgres.dsn", "BBS_CONTENT_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_CONTENT_POSTGRES_DEBUG")
	bindEnv(v, "redis.addr", "BBS_CONTENT_REDIS_ADDR")
	bindEnv(v, "redis.url", "BBS_CONTENT_REDIS_URL", "BBS_CONTENT_REDIS_ADDR")
	bindEnv(v, "redis.db", "BBS_CONTENT_REDIS_DB")
	bindEnv(v, "redis.dbNum", "BBS_CONTENT_REDIS_DB_NUM", "BBS_CONTENT_REDIS_DB")
	bindEnv(v, "redis.password", "BBS_CONTENT_REDIS_PASSWORD")
	bindEnv(v, "kafka.brokers", "BBS_CONTENT_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_CONTENT_KAFKA_TOPIC")
	bindEnv(v, "upstreams.comment", "BBS_CONTENT_UPSTREAMS_COMMENT")
	bindEnv(v, "upstreams.credit", "BBS_CONTENT_UPSTREAMS_CREDIT")
	bindEnv(v, "cache.ttl", "BBS_CONTENT_CACHE_TTL")
	bindEnv(v, "snowflake.workerId", "BBS_CONTENT_SNOWFLAKE_WORKER_ID")
	bindEnv(v, "grpc.server.port", "BBS_CONTENT_GRPC_SERVER_PORT", "BBS_CONTENT_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_CONTENT_GRPC_SERVER_SERVICE_NAME", "BBS_CONTENT_SERVICE_NAME")
	bindEnv(v, "grpc.client.timeout", "BBS_CONTENT_GRPC_CLIENT_TIMEOUT")
	bindEnv(v, "grpc.client.tag", "BBS_CONTENT_GRPC_CLIENT_TAG")
	bindEnv(v, "grpc.client.serverName", "BBS_CONTENT_GRPC_CLIENT_SERVER_NAME", "BBS_CONTENT_SERVICE_NAME")
	bindEnv(v, "trace.grpcEndpoint", "BBS_CONTENT_TRACE_GRPC_ENDPOINT")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_KAFKA_BROKERS")); value != "" {
		v.Set("kafka.brokers", splitCommaSeparated(value))
	}
}

func setDefaults(v *viper.Viper) {
	serviceName := stringDefault(v.GetString("service.name"), "bbs-content-service")
	servicePort := v.GetInt("service.grpcPort")
	if servicePort == 0 {
		servicePort = 9103
	}
	v.Set("service.name", serviceName)
	v.Set("service.grpcPort", servicePort)
	setStringDefault(v, "app.name", serviceName)

	setStringDefault(v, "log.filename", "logs/content-service.log")
	setIntDefault(v, "log.maxSize", 100)
	setIntDefault(v, "log.maxBackups", 7)
	setIntDefault(v, "log.maxAge", 30)
	setStringDefault(v, "log.level", "info")
	if !v.IsSet("log.stdout") {
		v.Set("log.stdout", true)
	}

	setStringDefault(v, "postgres.dsn", "postgres://bbs_content_app:local_content_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_content")

	redisURL := stringDefault(v.GetString("redis.url"), v.GetString("redis.addr"))
	setStringDefault(v, "redis.url", stringDefault(redisURL, "127.0.0.1:6379"))
	if !v.IsSet("redis.dbNum") {
		v.Set("redis.dbNum", v.GetInt("redis.db"))
	}
	setIntDefault(v, "redis.maxIdle", 10)
	setIntDefault(v, "redis.maxActive", 100)
	setIntDefault(v, "redis.idleTimeout", 10)
	setIntDefault(v, "redis.timeout", 5)
	setStringDefault(v, "redis.network", "tcp")

	if len(v.GetStringSlice("kafka.brokers")) == 0 {
		v.Set("kafka.brokers", []string{"127.0.0.1:9092"})
	}
	setStringDefault(v, "kafka.topic", "article.events")
	setStringDefault(v, "upstreams.comment", "bbs-comment-service")
	setStringDefault(v, "upstreams.credit", "bbs-credit-service")
	if v.GetDuration("cache.ttl") <= 0 {
		v.Set("cache.ttl", 5*time.Minute)
	}
	setIntDefault(v, "snowflake.workerId", 3)

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
	if v.GetDuration("grpc.client.timeout") <= 0 {
		v.Set("grpc.client.timeout", 10*time.Second)
	}
	setStringDefault(v, "grpc.client.tag", "content")
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
