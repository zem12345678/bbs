package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"content-service/pkg/snowflake"
	"content-service/pkg/uuid"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken                  = "bbs-local-content-internal-token"
	minProductionInternalAuthTokenBytes        = 32
	localDevCommentInternalAuthToken           = "bbs-local-comment-internal-token"
	minProductionCommentInternalAuthTokenBytes = 32
	localDevMallInternalAuthToken              = "bbs-local-mall-internal-token"
	minProductionMallInternalAuthTokenBytes    = 32
	localDevCreditInternalAuthToken            = "bbs-local-credit-internal-token"
	minProductionCreditInternalAuthTokenBytes  = 32
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
	applyNacosEnvOverrides(v)

	var nacosOptions Options
	if err := v.UnmarshalKey("nacos", &nacosOptions); err != nil {
		return nil, errors.Wrap(err, "unmarshal nacos option error")
	}
	if !skipNacos() && nacosOptions.enabled() {
		if err := readNacosConfig(v, nacosOptions); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(v)
	if err := applyGRPCPortEnvOverride(v,
		"BBS_CONTENT_GRPC_SERVER_PORT",
		"BBS_CONTENT_SERVICE_GRPC_PORT",
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
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_CONTENT_SKIP_NACOS"))) {
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
	bindEnv(v, "upstreams.commentInternalAuthToken", "BBS_CONTENT_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.mall", "BBS_CONTENT_UPSTREAMS_MALL")
	bindEnv(v, "upstreams.mallInternalAuthToken", "BBS_CONTENT_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.credit", "BBS_CONTENT_UPSTREAMS_CREDIT")
	bindEnv(v, "upstreams.creditInternalAuthToken", "BBS_CONTENT_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "cache.ttl", "BBS_CONTENT_CACHE_TTL")
	bindEnv(v, "outbox.owner", "BBS_CONTENT_OUTBOX_OWNER")
	bindEnv(v, "outbox.batchSize", "BBS_CONTENT_OUTBOX_BATCH_SIZE")
	bindEnv(v, "outbox.leaseDuration", "BBS_CONTENT_OUTBOX_LEASE_DURATION")
	bindEnv(v, "outbox.interval", "BBS_CONTENT_OUTBOX_INTERVAL")
	bindEnv(v, "outbox.retryDelay", "BBS_CONTENT_OUTBOX_RETRY_DELAY")
	bindEnv(v, "snowflake.workerId", "BBS_CONTENT_SNOWFLAKE_WORKER_ID")
	bindEnv(v, "snowflake.workerIdRangeStart", "BBS_CONTENT_SNOWFLAKE_WORKER_ID_RANGE_START")
	bindEnv(v, "snowflake.workerIdRangeSize", "BBS_CONTENT_SNOWFLAKE_WORKER_ID_RANGE_SIZE")
	bindEnv(v, "snowflake.instanceName", "BBS_CONTENT_SNOWFLAKE_INSTANCE_NAME")
	bindEnv(v, "grpc.server.port", "BBS_CONTENT_GRPC_SERVER_PORT", "BBS_CONTENT_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_CONTENT_GRPC_SERVER_SERVICE_NAME", "BBS_CONTENT_SERVICE_NAME")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_CONTENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_CONTENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "grpc.client.timeout", "BBS_CONTENT_GRPC_CLIENT_TIMEOUT")
	bindEnv(v, "grpc.client.tag", "BBS_CONTENT_GRPC_CLIENT_TAG")
	bindEnv(v, "grpc.client.serverName", "BBS_CONTENT_GRPC_CLIENT_SERVER_NAME", "BBS_CONTENT_SERVICE_NAME")
	bindEnv(v, "trace.grpcEndpoint", "BBS_CONTENT_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.env", "BBS_CONTENT_TRACE_ENV")
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
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_POSTGRES_DSN")); value != "" {
		v.Set("postgres.dsn", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_POSTGRES_DEBUG")); value != "" {
		v.Set("postgres.debug", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_KAFKA_BROKERS")); value != "" {
		v.Set("kafka.brokers", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
	}
	if value := firstNonEmptyEnv("BBS_CONTENT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_CONTENT_INTERNAL_AUTH_TOKEN"); value != "" {
		v.Set("grpc.server.internalAuthToken", value)
	}
}

func applyNacosEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_NACOS_ADDR")); value != "" {
		v.Set("nacos.addr", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_NACOS_DATA_ID")); value != "" {
		v.Set("nacos.dataId", value)
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
	setStringDefault(v, "upstreams.commentInternalAuthToken", localDevCommentInternalAuthToken)
	setStringDefault(v, "upstreams.mall", "bbs-mall-service")
	setStringDefault(v, "upstreams.mallInternalAuthToken", localDevMallInternalAuthToken)
	setStringDefault(v, "upstreams.credit", "bbs-credit-service")
	setStringDefault(v, "upstreams.creditInternalAuthToken", localDevCreditInternalAuthToken)
	if v.GetDuration("cache.ttl") <= 0 {
		v.Set("cache.ttl", 5*time.Minute)
	}
	setStringDefault(v, "outbox.owner", serviceName)
	setIntDefault(v, "outbox.batchSize", 20)
	if v.GetDuration("outbox.leaseDuration") <= 0 {
		v.Set("outbox.leaseDuration", 30*time.Second)
	}
	if v.GetDuration("outbox.interval") <= 0 {
		v.Set("outbox.interval", time.Second)
	}
	if v.GetDuration("outbox.retryDelay") <= 0 {
		v.Set("outbox.retryDelay", 3*time.Second)
	}
	setIntDefault(v, "snowflake.workerId", 3)

	if v.GetInt("grpc.server.port") == 0 {
		v.Set("grpc.server.port", servicePort)
	}
	setStringDefault(v, "grpc.server.serviceName", serviceName)
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
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

func validate(v *viper.Viper) error {
	if _, err := snowflake.ResolveWorkerID(
		v.GetInt64("snowflake.workerId"),
		v.GetInt64("snowflake.workerIdRangeStart"),
		v.GetInt64("snowflake.workerIdRangeSize"),
		v.GetString("snowflake.instanceName"),
	); err != nil {
		return fmt.Errorf("content snowflake worker ID: %w", err)
	}
	if !isProductionEnvironment(v.GetString("trace.env")) {
		return nil
	}
	token := strings.TrimSpace(v.GetString("upstreams.mallInternalAuthToken"))
	if token == "" || token == localDevMallInternalAuthToken {
		return fmt.Errorf("upstreams.mallInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < minProductionMallInternalAuthTokenBytes {
		return fmt.Errorf("upstreams.mallInternalAuthToken must be at least %d bytes in production", minProductionMallInternalAuthTokenBytes)
	}
	creditToken := strings.TrimSpace(v.GetString("upstreams.creditInternalAuthToken"))
	if creditToken == "" || creditToken == localDevCreditInternalAuthToken {
		return fmt.Errorf("upstreams.creditInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(creditToken)) < minProductionCreditInternalAuthTokenBytes {
		return fmt.Errorf("upstreams.creditInternalAuthToken must be at least %d bytes in production", minProductionCreditInternalAuthTokenBytes)
	}
	commentToken := strings.TrimSpace(v.GetString("upstreams.commentInternalAuthToken"))
	if commentToken == "" || commentToken == localDevCommentInternalAuthToken {
		return fmt.Errorf("upstreams.commentInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(commentToken)) < minProductionCommentInternalAuthTokenBytes {
		return fmt.Errorf("upstreams.commentInternalAuthToken must be at least %d bytes in production", minProductionCommentInternalAuthTokenBytes)
	}
	internalAuthToken := strings.TrimSpace(v.GetString("grpc.server.internalAuthToken"))
	if internalAuthToken == "" || internalAuthToken == localDevInternalAuthToken {
		return fmt.Errorf("grpc.server.internalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(internalAuthToken)) < minProductionInternalAuthTokenBytes {
		return fmt.Errorf("grpc.server.internalAuthToken must be at least %d bytes in production", minProductionInternalAuthTokenBytes)
	}
	return nil
}

func isProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return true
	default:
		return false
	}
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
