package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"chat-service/pkg/snowflake"
	"chat-service/pkg/uuid"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken           = "bbs-local-chat-internal-token"
	minProductionInternalAuthTokenBytes = 32
)

// Options describes the Nacos config endpoint used by the service.
type Options struct {
	Addr        string `mapstructure:"addr" toml:"addr" json:"addr" yaml:"addr" env:"NACOS_ADDR"`
	Port        uint64 `mapstructure:"port" toml:"port" json:"port" yaml:"port" env:"NACOS_PORT"`
	NamespaceID string `mapstructure:"namespaceId" toml:"namespaceId" json:"namespaceId" yaml:"namespaceId" env:"NACOS_NAMESPACEID"`
	DataID      string `mapstructure:"dataId" toml:"dataId" json:"dataId" yaml:"dataId" env:"NACOS_DATAID"`
	GroupID     string `mapstructure:"groupId" toml:"groupId" json:"groupId" yaml:"groupId" env:"NACOS_GROUPID"`
}

func (o Options) enabled() bool {
	return strings.TrimSpace(o.Addr) != "" && o.Port != 0 && strings.TrimSpace(o.DataID) != ""
}

// New loads an immutable startup configuration from the local file, optionally
// overlays Nacos, applies environment overrides, and returns the Viper
// instance consumed by the IoC providers.
func New(path string) (*viper.Viper, error) {
	v := viper.New()
	configureEnv(v)
	v.AddConfigPath(".")
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, errors.Wrap(err, "read config file error")
	}
	fmt.Printf("use config file -> %s\n", v.ConfigFileUsed())

	// Nacos endpoint settings are allowed to come from the environment so a
	// deployment can select its namespace/data ID without changing the image.
	if err := applyNacosEnvOverrides(v); err != nil {
		return nil, err
	}
	if !skipNacos() {
		var nacosOptions Options
		if err := v.UnmarshalKey("nacos", &nacosOptions); err != nil {
			return nil, errors.Wrap(err, "unmarshal nacos option error")
		}
		if nacosOptions.enabled() {
			if err := readNacosConfig(v, nacosOptions); err != nil {
				return nil, err
			}
		}
	}

	if err := applyEnvOverrides(v); err != nil {
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
		return errors.Wrap(err, "create nacos config client error")
	}
	defer configClient.CloseClient()

	content, err := configClient.GetConfig(vo.ConfigParam{DataId: o.DataID, Group: group})
	if err != nil {
		return errors.Wrap(err, "get nacos config error")
	}
	if strings.TrimSpace(content) != "" {
		if err := v.MergeConfig(bytes.NewBufferString(content)); err != nil {
			return errors.Wrap(err, "merge nacos config error")
		}
	}
	return nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_CHAT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_CHAT_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_CHAT_SERVICE_GRPC_PORT", "BBS_CHAT_GRPC_PORT")
	bindEnv(v, "app.name", "BBS_CHAT_APP_NAME")

	bindEnv(v, "log.filename", "BBS_CHAT_LOG_FILENAME")
	bindEnv(v, "log.maxSize", "BBS_CHAT_LOG_MAX_SIZE")
	bindEnv(v, "log.maxBackups", "BBS_CHAT_LOG_MAX_BACKUPS")
	bindEnv(v, "log.maxAge", "BBS_CHAT_LOG_MAX_AGE")
	bindEnv(v, "log.level", "BBS_CHAT_LOG_LEVEL")
	bindEnv(v, "log.stdout", "BBS_CHAT_LOG_STDOUT")

	bindEnv(v, "postgres.dsn", "BBS_CHAT_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_CHAT_POSTGRES_DEBUG")
	bindEnv(v, "postgres.max_open_conns", "BBS_CHAT_POSTGRES_MAX_OPEN_CONNS", "BBS_CHAT_POSTGRES_MAX_OPEN_CONNECTIONS")

	bindEnv(v, "redis.addr", "BBS_CHAT_REDIS_ADDR")
	bindEnv(v, "redis.url", "BBS_CHAT_REDIS_URL", "BBS_CHAT_REDIS_ADDR")
	bindEnv(v, "redis.db", "BBS_CHAT_REDIS_DB")
	bindEnv(v, "redis.dbNum", "BBS_CHAT_REDIS_DB_NUM", "BBS_CHAT_REDIS_DB")
	bindEnv(v, "redis.password", "BBS_CHAT_REDIS_PASSWORD")
	bindEnv(v, "redis.maxIdle", "BBS_CHAT_REDIS_MAX_IDLE")
	bindEnv(v, "redis.maxActive", "BBS_CHAT_REDIS_MAX_ACTIVE")
	bindEnv(v, "redis.idleTimeout", "BBS_CHAT_REDIS_IDLE_TIMEOUT")
	bindEnv(v, "redis.timeout", "BBS_CHAT_REDIS_TIMEOUT")
	bindEnv(v, "redis.network", "BBS_CHAT_REDIS_NETWORK")

	bindEnv(v, "kafka.brokers", "BBS_CHAT_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_CHAT_KAFKA_TOPIC")
	bindEnv(v, "kafka.topics", "BBS_CHAT_KAFKA_TOPICS")
	bindEnv(v, "kafka.groupId", "BBS_CHAT_KAFKA_GROUP_ID", "BBS_CHAT_KAFKA_GROUPID")
	bindEnv(v, "kafka.realtimeGroupId", "BBS_CHAT_KAFKA_REALTIME_GROUP_ID")
	bindEnv(v, "kafka.username", "BBS_CHAT_KAFKA_USERNAME")
	bindEnv(v, "kafka.password", "BBS_CHAT_KAFKA_PASSWORD")
	bindEnv(v, "kafka.scram_algorithm", "BBS_CHAT_KAFKA_SCRAM_ALGORITHM")
	bindEnv(v, "kafka.scramAlgorithm", "BBS_CHAT_KAFKA_SCRAM_ALGORITHM")
	bindKafkaNestedEnv(v, "producerOptions", "BBS_CHAT_KAFKA_PRODUCER")
	bindKafkaNestedEnv(v, "consumerOptions", "BBS_CHAT_KAFKA_CONSUMER")

	bindEnv(v, "outbox.owner", "BBS_CHAT_OUTBOX_OWNER")
	bindEnv(v, "outbox.batchSize", "BBS_CHAT_OUTBOX_BATCH_SIZE")
	bindEnv(v, "outbox.leaseDuration", "BBS_CHAT_OUTBOX_LEASE_DURATION")
	bindEnv(v, "outbox.interval", "BBS_CHAT_OUTBOX_INTERVAL")
	bindEnv(v, "outbox.retryDelay", "BBS_CHAT_OUTBOX_RETRY_DELAY")
	bindEnv(v, "outbox.publishTimeout", "BBS_CHAT_OUTBOX_PUBLISH_TIMEOUT")

	bindEnv(v, "snowflake.workerId", "BBS_CHAT_SNOWFLAKE_WORKER_ID")
	bindEnv(v, "snowflake.workerIdRangeStart", "BBS_CHAT_SNOWFLAKE_WORKER_ID_RANGE_START")
	bindEnv(v, "snowflake.workerIdRangeSize", "BBS_CHAT_SNOWFLAKE_WORKER_ID_RANGE_SIZE")
	bindEnv(v, "snowflake.instanceName", "BBS_CHAT_SNOWFLAKE_INSTANCE_NAME")
	bindEnv(v, "grpc.server.host", "BBS_CHAT_GRPC_SERVER_HOST", "BBS_CHAT_GRPC_HOST")
	bindEnv(v, "grpc.server.port", "BBS_CHAT_GRPC_SERVER_PORT", "BBS_CHAT_GRPC_PORT", "BBS_CHAT_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.advertiseHost", "BBS_CHAT_GRPC_SERVER_ADVERTISE_HOST", "BBS_CHAT_GRPC_ADVERTISE_HOST")
	bindEnv(v, "grpc.server.etcdAddr", "BBS_CHAT_GRPC_SERVER_ETCD_ADDR", "BBS_CHAT_ETCD_ADDR")
	bindEnv(v, "grpc.server.serviceName", "BBS_CHAT_GRPC_SERVER_SERVICE_NAME", "BBS_CHAT_GRPC_SERVICE_NAME", "BBS_CHAT_SERVICE_NAME")
	bindEnv(v, "grpc.server.timeout", "BBS_CHAT_GRPC_SERVER_TIMEOUT")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_CHAT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_CHAT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "grpc.server.tls.enabled", "BBS_CHAT_GRPC_SERVER_TLS_ENABLED")
	bindEnv(v, "grpc.server.tls.certFile", "BBS_CHAT_GRPC_SERVER_TLS_CERT_FILE")
	bindEnv(v, "grpc.server.tls.keyFile", "BBS_CHAT_GRPC_SERVER_TLS_KEY_FILE")
	bindEnv(v, "grpc.server.tls.clientCAFile", "BBS_CHAT_GRPC_SERVER_TLS_CLIENT_CA_FILE")
	bindEnv(v, "grpc.server.rateLimit.interval", "BBS_CHAT_GRPC_SERVER_RATE_LIMIT_INTERVAL")
	bindEnv(v, "grpc.server.rateLimit.rate", "BBS_CHAT_GRPC_SERVER_RATE_LIMIT_RATE")
	bindEnv(v, "grpc.client.timeout", "BBS_CHAT_GRPC_CLIENT_TIMEOUT")
	bindEnv(v, "grpc.client.tag", "BBS_CHAT_GRPC_CLIENT_TAG")
	bindEnv(v, "grpc.client.etcdAddr", "BBS_CHAT_GRPC_CLIENT_ETCD_ADDR")
	bindEnv(v, "grpc.client.serverName", "BBS_CHAT_GRPC_CLIENT_SERVER_NAME")
	bindEnv(v, "grpc.client.secure", "BBS_CHAT_GRPC_CLIENT_SECURE")

	bindEnv(v, "trace.grpcEndpoint", "BBS_CHAT_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.serviceName", "BBS_CHAT_TRACE_SERVICE_NAME")
	bindEnv(v, "trace.version", "BBS_CHAT_TRACE_VERSION")
	bindEnv(v, "trace.env", "BBS_CHAT_TRACE_ENV")

	bindEnv(v, "nacos.addr", "BBS_CHAT_NACOS_ADDR")
	bindEnv(v, "nacos.port", "BBS_CHAT_NACOS_PORT")
	bindEnv(v, "nacos.namespaceId", "BBS_CHAT_NACOS_NAMESPACE_ID", "BBS_CHAT_NACOS_NAMESPACEID")
	bindEnv(v, "nacos.dataId", "BBS_CHAT_NACOS_DATA_ID", "BBS_CHAT_NACOS_DATAID")
	bindEnv(v, "nacos.groupId", "BBS_CHAT_NACOS_GROUP_ID", "BBS_CHAT_NACOS_GROUPID")
}

func bindKafkaNestedEnv(v *viper.Viper, section, prefix string) {
	bindEnv(v, "kafka."+section+".brokers", prefix+"_BROKERS")
	bindEnv(v, "kafka."+section+".topics", prefix+"_TOPICS")
	bindEnv(v, "kafka."+section+".topic", prefix+"_TOPIC")
	bindEnv(v, "kafka."+section+".groupId", prefix+"_GROUP_ID", prefix+"_GROUPID")
	bindEnv(v, "kafka."+section+".username", prefix+"_USERNAME")
	bindEnv(v, "kafka."+section+".password", prefix+"_PASSWORD")
	bindEnv(v, "kafka."+section+".scram_algorithm", prefix+"_SCRAM_ALGORITHM")
	bindEnv(v, "kafka."+section+".scramAlgorithm", prefix+"_SCRAM_ALGORITHM")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

// applyNacosEnvOverrides is intentionally separate from the service config
// overrides: these values must be applied before the Nacos client is created.
func applyNacosEnvOverrides(v *viper.Viper) error {
	setStringFromEnv(v, "nacos.addr", "BBS_CHAT_NACOS_ADDR")
	setStringFromEnv(v, "nacos.namespaceId", "BBS_CHAT_NACOS_NAMESPACE_ID", "BBS_CHAT_NACOS_NAMESPACEID")
	setStringFromEnv(v, "nacos.dataId", "BBS_CHAT_NACOS_DATA_ID", "BBS_CHAT_NACOS_DATAID")
	setStringFromEnv(v, "nacos.groupId", "BBS_CHAT_NACOS_GROUP_ID", "BBS_CHAT_NACOS_GROUPID")
	if value := firstEnv("BBS_CHAT_NACOS_PORT"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 16)
		if err != nil || parsed == 0 {
			return fmt.Errorf("invalid BBS_CHAT_NACOS_PORT %q", value)
		}
		setNestedConfigValue(v, "nacos.port", parsed)
	}
	return nil
}

// applyEnvOverrides gives list values CSV semantics and reports malformed
// scalar values instead of silently replacing them with defaults.
func applyEnvOverrides(v *viper.Viper) error {
	stringOverrides := map[string][]string{
		"service.name":                  {"BBS_CHAT_SERVICE_NAME"},
		"app.name":                      {"BBS_CHAT_APP_NAME"},
		"log.filename":                  {"BBS_CHAT_LOG_FILENAME"},
		"log.level":                     {"BBS_CHAT_LOG_LEVEL"},
		"postgres.dsn":                  {"BBS_CHAT_POSTGRES_DSN"},
		"redis.addr":                    {"BBS_CHAT_REDIS_ADDR"},
		"redis.url":                     {"BBS_CHAT_REDIS_URL", "BBS_CHAT_REDIS_ADDR"},
		"redis.password":                {"BBS_CHAT_REDIS_PASSWORD"},
		"redis.network":                 {"BBS_CHAT_REDIS_NETWORK"},
		"kafka.topic":                   {"BBS_CHAT_KAFKA_TOPIC"},
		"kafka.groupId":                 {"BBS_CHAT_KAFKA_GROUP_ID", "BBS_CHAT_KAFKA_GROUPID"},
		"kafka.realtimeGroupId":         {"BBS_CHAT_KAFKA_REALTIME_GROUP_ID"},
		"kafka.username":                {"BBS_CHAT_KAFKA_USERNAME"},
		"kafka.password":                {"BBS_CHAT_KAFKA_PASSWORD"},
		"kafka.scram_algorithm":         {"BBS_CHAT_KAFKA_SCRAM_ALGORITHM"},
		"kafka.scramAlgorithm":          {"BBS_CHAT_KAFKA_SCRAM_ALGORITHM"},
		"outbox.owner":                  {"BBS_CHAT_OUTBOX_OWNER"},
		"outbox.leaseDuration":          {"BBS_CHAT_OUTBOX_LEASE_DURATION"},
		"outbox.interval":               {"BBS_CHAT_OUTBOX_INTERVAL"},
		"outbox.retryDelay":             {"BBS_CHAT_OUTBOX_RETRY_DELAY"},
		"outbox.publishTimeout":         {"BBS_CHAT_OUTBOX_PUBLISH_TIMEOUT"},
		"grpc.server.host":              {"BBS_CHAT_GRPC_SERVER_HOST", "BBS_CHAT_GRPC_HOST"},
		"grpc.server.advertiseHost":     {"BBS_CHAT_GRPC_SERVER_ADVERTISE_HOST", "BBS_CHAT_GRPC_ADVERTISE_HOST"},
		"grpc.server.serviceName":       {"BBS_CHAT_GRPC_SERVER_SERVICE_NAME", "BBS_CHAT_GRPC_SERVICE_NAME", "BBS_CHAT_SERVICE_NAME"},
		"grpc.server.internalAuthToken": {"BBS_CHAT_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_CHAT_INTERNAL_AUTH_TOKEN"},
		"trace.grpcEndpoint":            {"BBS_CHAT_TRACE_GRPC_ENDPOINT"},
		"trace.serviceName":             {"BBS_CHAT_TRACE_SERVICE_NAME"},
		"trace.version":                 {"BBS_CHAT_TRACE_VERSION"},
		"trace.env":                     {"BBS_CHAT_TRACE_ENV"},
		"snowflake.instanceName":        {"BBS_CHAT_SNOWFLAKE_INSTANCE_NAME"},
	}
	for key, envs := range stringOverrides {
		setStringFromEnv(v, key, envs...)
	}

	listOverrides := map[string][]string{
		"grpc.server.etcdAddr":          {"BBS_CHAT_GRPC_SERVER_ETCD_ADDR", "BBS_CHAT_ETCD_ADDR"},
		"grpc.client.etcdAddr":          {"BBS_CHAT_GRPC_CLIENT_ETCD_ADDR"},
		"kafka.brokers":                 {"BBS_CHAT_KAFKA_BROKERS"},
		"kafka.topics":                  {"BBS_CHAT_KAFKA_TOPICS"},
		"kafka.producerOptions.brokers": {"BBS_CHAT_KAFKA_PRODUCER_BROKERS"},
		"kafka.producerOptions.topics":  {"BBS_CHAT_KAFKA_PRODUCER_TOPICS"},
		"kafka.consumerOptions.brokers": {"BBS_CHAT_KAFKA_CONSUMER_BROKERS"},
		"kafka.consumerOptions.topics":  {"BBS_CHAT_KAFKA_CONSUMER_TOPICS"},
	}
	for key, envs := range listOverrides {
		if value := firstEnv(envs...); value != "" {
			setNestedConfigValue(v, key, splitCommaSeparated(value))
		}
	}

	integerOverrides := map[string][]string{
		"service.grpcPort":             {"BBS_CHAT_SERVICE_GRPC_PORT", "BBS_CHAT_GRPC_PORT"},
		"log.maxSize":                  {"BBS_CHAT_LOG_MAX_SIZE"},
		"log.maxBackups":               {"BBS_CHAT_LOG_MAX_BACKUPS"},
		"log.maxAge":                   {"BBS_CHAT_LOG_MAX_AGE"},
		"postgres.max_open_conns":      {"BBS_CHAT_POSTGRES_MAX_OPEN_CONNS", "BBS_CHAT_POSTGRES_MAX_OPEN_CONNECTIONS"},
		"redis.db":                     {"BBS_CHAT_REDIS_DB"},
		"redis.dbNum":                  {"BBS_CHAT_REDIS_DB_NUM", "BBS_CHAT_REDIS_DB"},
		"redis.maxIdle":                {"BBS_CHAT_REDIS_MAX_IDLE"},
		"redis.maxActive":              {"BBS_CHAT_REDIS_MAX_ACTIVE"},
		"redis.idleTimeout":            {"BBS_CHAT_REDIS_IDLE_TIMEOUT"},
		"redis.timeout":                {"BBS_CHAT_REDIS_TIMEOUT"},
		"outbox.batchSize":             {"BBS_CHAT_OUTBOX_BATCH_SIZE"},
		"snowflake.workerId":           {"BBS_CHAT_SNOWFLAKE_WORKER_ID"},
		"snowflake.workerIdRangeStart": {"BBS_CHAT_SNOWFLAKE_WORKER_ID_RANGE_START"},
		"snowflake.workerIdRangeSize":  {"BBS_CHAT_SNOWFLAKE_WORKER_ID_RANGE_SIZE"},
		"grpc.server.port":             {"BBS_CHAT_GRPC_SERVER_PORT", "BBS_CHAT_GRPC_PORT", "BBS_CHAT_SERVICE_GRPC_PORT"},
		"grpc.server.rateLimit.rate":   {"BBS_CHAT_GRPC_SERVER_RATE_LIMIT_RATE"},
	}
	for key, envs := range integerOverrides {
		if value := firstEnv(envs...); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("parse %s: %w", strings.Join(envs, "/"), err)
			}
			setNestedConfigValue(v, key, parsed)
		}
	}

	boolOverrides := map[string][]string{
		"log.stdout":              {"BBS_CHAT_LOG_STDOUT"},
		"postgres.debug":          {"BBS_CHAT_POSTGRES_DEBUG"},
		"grpc.client.secure":      {"BBS_CHAT_GRPC_CLIENT_SECURE"},
		"grpc.server.tls.enabled": {"BBS_CHAT_GRPC_SERVER_TLS_ENABLED"},
	}
	for key, envs := range boolOverrides {
		if value := firstEnv(envs...); value != "" {
			parsed, err := parseBool(value)
			if err != nil {
				return fmt.Errorf("parse %s: %w", strings.Join(envs, "/"), err)
			}
			setNestedConfigValue(v, key, parsed)
		}
	}

	durationOverrides := map[string][]string{
		"outbox.leaseDuration":           {"BBS_CHAT_OUTBOX_LEASE_DURATION"},
		"outbox.interval":                {"BBS_CHAT_OUTBOX_INTERVAL"},
		"outbox.retryDelay":              {"BBS_CHAT_OUTBOX_RETRY_DELAY"},
		"outbox.publishTimeout":          {"BBS_CHAT_OUTBOX_PUBLISH_TIMEOUT"},
		"grpc.server.timeout":            {"BBS_CHAT_GRPC_SERVER_TIMEOUT"},
		"grpc.server.rateLimit.interval": {"BBS_CHAT_GRPC_SERVER_RATE_LIMIT_INTERVAL"},
		"grpc.client.timeout":            {"BBS_CHAT_GRPC_CLIENT_TIMEOUT"},
	}
	for key, envs := range durationOverrides {
		if value := firstEnv(envs...); value != "" {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("parse %s: %w", strings.Join(envs, "/"), err)
			}
			setNestedConfigValue(v, key, parsed)
		}
	}

	// Nested Kafka values are kept independent so producer and consumer may
	// use different credentials, topics, or broker lists when required.
	applyKafkaNestedStringEnv(v, "producerOptions", "BBS_CHAT_KAFKA_PRODUCER")
	applyKafkaNestedStringEnv(v, "consumerOptions", "BBS_CHAT_KAFKA_CONSUMER")
	return nil
}

func applyKafkaNestedStringEnv(v *viper.Viper, section, prefix string) {
	for key, envs := range map[string][]string{
		"topic":           {prefix + "_TOPIC"},
		"groupId":         {prefix + "_GROUP_ID", prefix + "_GROUPID"},
		"username":        {prefix + "_USERNAME"},
		"password":        {prefix + "_PASSWORD"},
		"scram_algorithm": {prefix + "_SCRAM_ALGORITHM"},
		"scramAlgorithm":  {prefix + "_SCRAM_ALGORITHM"},
	} {
		setStringFromEnv(v, "kafka."+section+"."+key, envs...)
	}
}

func setDefaults(v *viper.Viper) {
	serviceName := stringDefault(v.GetString("service.name"), "bbs-chat-service")
	setStringDefault(v, "service.name", serviceName)
	if !v.IsSet("service.grpcPort") || v.GetInt("service.grpcPort") <= 0 {
		setNestedConfigValue(v, "service.grpcPort", 9116)
	}
	setStringDefault(v, "app.name", serviceName)

	setStringDefault(v, "log.filename", "logs/chat-service.log")
	setIntDefault(v, "log.maxSize", 100)
	setIntDefault(v, "log.maxBackups", 7)
	setIntDefault(v, "log.maxAge", 30)
	setStringDefault(v, "log.level", "info")
	if !v.IsSet("log.stdout") {
		setNestedConfigValue(v, "log.stdout", true)
	}

	setStringDefault(v, "postgres.dsn", "postgres://bbs_chat_app:local_chat_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_chat")
	if !v.IsSet("postgres.max_open_conns") {
		setNestedConfigValue(v, "postgres.max_open_conns", 8)
	}
	if !v.IsSet("postgres.debug") {
		setNestedConfigValue(v, "postgres.debug", false)
	}

	redisURL := stringDefault(v.GetString("redis.url"), v.GetString("redis.addr"))
	setStringDefault(v, "redis.url", stringDefault(redisURL, "127.0.0.1:6379"))
	setStringDefault(v, "redis.addr", v.GetString("redis.url"))
	if !v.IsSet("redis.dbNum") {
		setNestedConfigValue(v, "redis.dbNum", v.GetInt("redis.db"))
	}
	setNestedConfigValue(v, "redis.db", v.GetInt("redis.dbNum"))
	setStringDefault(v, "redis.password", "")
	setIntDefault(v, "redis.maxIdle", 10)
	setIntDefault(v, "redis.maxActive", 100)
	setIntDefault(v, "redis.idleTimeout", 10)
	setIntDefault(v, "redis.timeout", 5)
	setStringDefault(v, "redis.network", "tcp")

	normalizeKafka(v)

	setStringDefault(v, "outbox.owner", serviceName)
	setIntDefault(v, "outbox.batchSize", 20)
	setDurationDefault(v, "outbox.leaseDuration", time.Minute)
	setDurationDefault(v, "outbox.interval", time.Second)
	setDurationDefault(v, "outbox.retryDelay", 3*time.Second)
	setDurationDefault(v, "outbox.publishTimeout", 2*time.Second)

	if !v.IsSet("snowflake.workerId") {
		setNestedConfigValue(v, "snowflake.workerId", 16)
	}

	if !v.IsSet("grpc.server.port") || v.GetInt("grpc.server.port") <= 0 {
		setNestedConfigValue(v, "grpc.server.port", v.GetInt("service.grpcPort"))
	}
	setStringDefault(v, "grpc.server.host", "0.0.0.0")
	setStringDefault(v, "grpc.server.serviceName", serviceName)
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
	if len(stringSlice(v.Get("grpc.server.etcdAddr"))) == 0 {
		setNestedConfigValue(v, "grpc.server.etcdAddr", []string{"127.0.0.1:2379"})
	}
	setDurationDefault(v, "grpc.server.timeout", 10*time.Second)
	setDurationDefault(v, "grpc.server.rateLimit.interval", time.Second)
	setIntDefault(v, "grpc.server.rateLimit.rate", 1000)
	if !v.IsSet("grpc.server.tls.enabled") {
		setNestedConfigValue(v, "grpc.server.tls.enabled", false)
	}
	setDurationDefault(v, "grpc.client.timeout", 10*time.Second)
	setStringDefault(v, "grpc.client.tag", "chat")
	setStringDefault(v, "grpc.client.serverName", serviceName)
	if len(stringSlice(v.Get("grpc.client.etcdAddr"))) == 0 {
		setNestedConfigValue(v, "grpc.client.etcdAddr", stringSlice(v.Get("grpc.server.etcdAddr")))
	}
	if !v.IsSet("grpc.client.secure") {
		setNestedConfigValue(v, "grpc.client.secure", false)
	}

	setStringDefault(v, "trace.grpcEndpoint", "127.0.0.1:4317")
	setStringDefault(v, "trace.serviceName", serviceName)
	setStringDefault(v, "trace.version", "local")
	setStringDefault(v, "trace.env", "local")
}

func normalizeKafka(v *viper.Viper) {
	brokers := stringSlice(v.Get("kafka.brokers"))
	producerBrokers := stringSlice(v.Get("kafka.producerOptions.brokers"))
	consumerBrokers := stringSlice(v.Get("kafka.consumerOptions.brokers"))
	if len(brokers) == 0 {
		switch {
		case len(producerBrokers) > 0:
			brokers = producerBrokers
		case len(consumerBrokers) > 0:
			brokers = consumerBrokers
		default:
			brokers = []string{"127.0.0.1:9092"}
		}
	}
	setNestedConfigValue(v, "kafka.brokers", brokers)
	if len(producerBrokers) == 0 {
		producerBrokers = brokers
	}
	if len(consumerBrokers) == 0 {
		consumerBrokers = brokers
	}
	setNestedConfigValue(v, "kafka.producerOptions.brokers", producerBrokers)
	setNestedConfigValue(v, "kafka.consumerOptions.brokers", consumerBrokers)

	topic := stringDefault(v.GetString("kafka.topic"), v.GetString("kafka.producerOptions.topic"))
	if topic == "" {
		topics := stringSlice(v.Get("kafka.consumerOptions.topics"))
		if len(topics) > 0 {
			topic = topics[0]
		}
	}
	topic = stringDefault(topic, "chat.events")
	setNestedConfigValue(v, "kafka.topic", topic)
	setNestedConfigValue(v, "kafka.producerOptions.topic", stringDefault(v.GetString("kafka.producerOptions.topic"), topic))

	topics := stringSlice(v.Get("kafka.topics"))
	if len(topics) == 0 {
		topics = stringSlice(v.Get("kafka.consumerOptions.topics"))
	}
	if len(topics) == 0 {
		topics = []string{topic}
	}
	setNestedConfigValue(v, "kafka.topics", topics)
	setNestedConfigValue(v, "kafka.consumerOptions.topics", topics)

	groupID := stringDefault(v.GetString("kafka.realtimeGroupId"), v.GetString("kafka.groupId"))
	groupID = stringDefault(groupID, v.GetString("kafka.consumerOptions.groupId"))
	groupID = stringDefault(groupID, "bbs-chat-realtime")
	setNestedConfigValue(v, "kafka.realtimeGroupId", groupID)
	setNestedConfigValue(v, "kafka.groupId", groupID)
	setNestedConfigValue(v, "kafka.consumerOptions.groupId", stringDefault(v.GetString("kafka.consumerOptions.groupId"), groupID))

	username := strings.TrimSpace(v.GetString("kafka.username"))
	password := strings.TrimSpace(v.GetString("kafka.password"))
	algorithm := firstNonEmpty(v.GetString("kafka.scram_algorithm"), v.GetString("kafka.scramAlgorithm"))
	if algorithm != "" {
		setNestedConfigValue(v, "kafka.scram_algorithm", algorithm)
		setNestedConfigValue(v, "kafka.scramAlgorithm", algorithm)
	}
	setKafkaCredentialDefaults(v, "producerOptions", username, password, algorithm)
	setKafkaCredentialDefaults(v, "consumerOptions", username, password, algorithm)
}

func setKafkaCredentialDefaults(v *viper.Viper, section, username, password, algorithm string) {
	prefix := "kafka." + section + "."
	if strings.TrimSpace(v.GetString(prefix+"username")) == "" && username != "" {
		setNestedConfigValue(v, prefix+"username", username)
	}
	if strings.TrimSpace(v.GetString(prefix+"password")) == "" && password != "" {
		setNestedConfigValue(v, prefix+"password", password)
	}
	if firstNonEmpty(v.GetString(prefix+"scram_algorithm"), v.GetString(prefix+"scramAlgorithm")) == "" && algorithm != "" {
		setNestedConfigValue(v, prefix+"scram_algorithm", algorithm)
		setNestedConfigValue(v, prefix+"scramAlgorithm", algorithm)
	}
	if strings.TrimSpace(v.GetString(prefix+"username")) != "" && strings.TrimSpace(v.GetString(prefix+"password")) != "" && firstNonEmpty(v.GetString(prefix+"scram_algorithm"), v.GetString(prefix+"scramAlgorithm")) == "" {
		setNestedConfigValue(v, prefix+"scram_algorithm", "SHA512")
		setNestedConfigValue(v, prefix+"scramAlgorithm", "SHA512")
	}
}

func validate(v *viper.Viper) error {
	if strings.TrimSpace(v.GetString("postgres.dsn")) == "" {
		return errors.New("chat postgres dsn is required")
	}
	if v.GetInt("postgres.max_open_conns") <= 0 {
		return errors.New("chat postgres max_open_conns must be positive")
	}

	if err := validateStringList("chat grpc server etcd addresses", stringSlice(v.Get("grpc.server.etcdAddr"))); err != nil {
		return err
	}
	if v.GetBool("grpc.client.secure") {
		return errors.New("chat grpc client TLS is not configured; grpc.client.secure must be false")
	}
	if v.GetDuration("grpc.server.rateLimit.interval") <= 0 || v.GetInt("grpc.server.rateLimit.rate") <= 0 {
		return errors.New("chat grpc server rate limit must be positive")
	}
	if err := validateStringList("chat kafka brokers", stringSlice(v.Get("kafka.brokers"))); err != nil {
		return err
	}
	if strings.TrimSpace(v.GetString("kafka.topic")) == "" {
		return errors.New("chat kafka topic is required")
	}
	if strings.TrimSpace(v.GetString("kafka.realtimeGroupId")) == "" || strings.TrimSpace(v.GetString("kafka.groupId")) == "" {
		return errors.New("chat kafka group is required")
	}
	if err := validateStringList("chat kafka topics", stringSlice(v.Get("kafka.topics"))); err != nil {
		return err
	}
	if err := validateKafkaTopology(v); err != nil {
		return err
	}

	for _, section := range []string{"kafka", "kafka.producerOptions", "kafka.consumerOptions"} {
		if err := validateSASL(v, section); err != nil {
			return err
		}
	}

	if _, err := snowflake.ResolveWorkerID(
		v.GetInt64("snowflake.workerId"),
		v.GetInt64("snowflake.workerIdRangeStart"),
		v.GetInt64("snowflake.workerIdRangeSize"),
		v.GetString("snowflake.instanceName"),
	); err != nil {
		return fmt.Errorf("chat snowflake worker ID: %w", err)
	}
	if isProductionEnvironment(v.GetString("trace.env")) && strings.TrimSpace(v.GetString("snowflake.instanceName")) == "" && strings.TrimSpace(os.Getenv("BBS_CHAT_SNOWFLAKE_WORKER_ID")) == "" {
		return errors.New("BBS_CHAT_SNOWFLAKE_WORKER_ID or BBS_CHAT_SNOWFLAKE_INSTANCE_NAME must be set in production")
	}
	if isProductionEnvironment(v.GetString("trace.env")) {
		if err := validateProductionInternalAuthToken(v.GetString("grpc.server.internalAuthToken")); err != nil {
			return err
		}
		if err := validateProductionServerTLS(v); err != nil {
			return err
		}
	}

	batchSize := v.GetInt("outbox.batchSize")
	leaseDuration := v.GetDuration("outbox.leaseDuration")
	interval := v.GetDuration("outbox.interval")
	publishTimeout := v.GetDuration("outbox.publishTimeout")
	if batchSize <= 0 {
		return errors.New("chat outbox batch size must be positive")
	}
	if leaseDuration <= 0 || interval <= 0 || publishTimeout <= 0 {
		return errors.New("chat outbox durations must be positive")
	}
	minimumLease := time.Duration(batchSize)*publishTimeout + interval
	if leaseDuration <= minimumLease {
		return fmt.Errorf("chat outbox lease duration must exceed batch publish window %s", minimumLease)
	}
	return nil
}

// validateKafkaTopology keeps the outbox writer and realtime dispatcher on
// the single chat event topic defined by the service architecture.
func validateKafkaTopology(v *viper.Viper) error {
	topic := strings.TrimSpace(v.GetString("kafka.topic"))
	if producerTopic := strings.TrimSpace(v.GetString("kafka.producerOptions.topic")); producerTopic != topic {
		return fmt.Errorf("chat kafka producer topic %q must match kafka.topic %q", producerTopic, topic)
	}

	consumerTopics := stringSlice(v.Get("kafka.consumerOptions.topics"))
	if len(consumerTopics) != 1 || consumerTopics[0] != topic {
		return fmt.Errorf("chat kafka consumer topics must contain only kafka.topic %q", topic)
	}
	if consumerGroupID := strings.TrimSpace(v.GetString("kafka.consumerOptions.groupId")); consumerGroupID != strings.TrimSpace(v.GetString("kafka.realtimeGroupId")) {
		return fmt.Errorf("chat kafka consumer group %q must match kafka.realtimeGroupId %q", consumerGroupID, v.GetString("kafka.realtimeGroupId"))
	}
	return nil
}

func validateSASL(v *viper.Viper, prefix string) error {
	username := strings.TrimSpace(v.GetString(prefix + ".username"))
	password := strings.TrimSpace(v.GetString(prefix + ".password"))
	if (username == "") != (password == "") {
		return fmt.Errorf("%s SASL username and password must be set together", prefix)
	}
	algorithm := firstNonEmpty(v.GetString(prefix+".scram_algorithm"), v.GetString(prefix+".scramAlgorithm"))
	if algorithm == "" {
		return nil
	}
	switch strings.ToUpper(algorithm) {
	case "SHA256", "SHA512":
		return nil
	default:
		return fmt.Errorf("%s SASL algorithm must be SHA256 or SHA512", prefix)
	}
}

func validateStringList(name string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s are required", name)
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not contain empty values", name)
		}
	}
	return nil
}

func setHostUUID(v *viper.Viper) error {
	uuidString, err := uuid.GetHostUuid()
	if err != nil || strings.TrimSpace(uuidString) == "" {
		uuidString, err = uuid.NewUUID()
	}
	if err != nil {
		return err
	}
	setNestedConfigValue(v, "server.uuid", uuidString)
	return nil
}

func setStringDefault(v *viper.Viper, key, fallback string) {
	if strings.TrimSpace(v.GetString(key)) == "" {
		setNestedConfigValue(v, key, fallback)
	}
}

func setIntDefault(v *viper.Viper, key string, fallback int) {
	if !v.IsSet(key) {
		setNestedConfigValue(v, key, fallback)
	}
}

func setDurationDefault(v *viper.Viper, key string, fallback time.Duration) {
	if !v.IsSet(key) {
		setNestedConfigValue(v, key, fallback)
	}
}

func setStringFromEnv(v *viper.Viper, key string, envs ...string) {
	if value := firstEnv(envs...); value != "" {
		setNestedConfigValue(v, key, value)
	}
}

func firstEnv(envs ...string) string {
	for _, env := range envs {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			return value
		}
	}
	return ""
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func stringDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func stringSlice(value interface{}) []string {
	switch values := value.(type) {
	case []string:
		return cleanStringSlice(values)
	case []interface{}:
		result := make([]string, 0, len(values))
		for _, item := range values {
			result = append(result, fmt.Sprint(item))
		}
		return cleanStringSlice(result)
	case string:
		return splitCommaSeparated(values)
	default:
		return nil
	}
}

func cleanStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
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

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_CHAT_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func isProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return true
	default:
		return false
	}
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

func validateProductionServerTLS(v *viper.Viper) error {
	if !v.GetBool("grpc.server.tls.enabled") {
		return errors.New("grpc.server.tls.enabled must be true in production")
	}
	for _, key := range []string{"grpc.server.tls.certFile", "grpc.server.tls.keyFile", "grpc.server.tls.clientCAFile"} {
		if strings.TrimSpace(v.GetString(key)) == "" {
			return fmt.Errorf("%s is required when grpc server TLS is enabled", key)
		}
	}
	return nil
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
