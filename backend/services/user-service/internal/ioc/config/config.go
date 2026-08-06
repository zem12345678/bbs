package config

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"user-service/pkg/snowflake"
	"user-service/pkg/uuid"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken             = "bbs-local-user-internal-token"
	minProductionInternalAuthTokenBytes   = 32
	localDevContentInternalAuthToken      = "bbs-local-content-internal-token"
	localDevCommentInternalAuthToken      = "bbs-local-comment-internal-token"
	localDevReactionInternalAuthToken     = "bbs-local-reaction-internal-token"
	localDevChatInternalAuthToken         = "bbs-local-chat-internal-token"
	localDevNotificationInternalAuthToken = "bbs-local-notification-internal-token"
	localDevFileInternalAuthToken         = "bbs-local-file-internal-token"
	localDevCreditInternalAuthToken       = "bbs-local-credit-internal-token"
	localDevFeedInternalAuthToken         = "bbs-local-feed-internal-token"
	localDevSearchInternalAuthToken       = "bbs-local-search-internal-token"
	localDevMallInternalAuthToken         = "bbs-local-mall-internal-token"
	minProductionMallInternalAuthBytes    = 32
	minProductionMFAEncryptionKeyBytes    = 32
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
	setNestedConfigValue(v, "server.uuid", uuidstr)
	return v, err
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_USER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.grpcPort", "BBS_USER_SERVICE_GRPC_PORT", "BBS_USER_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_USER_GRPC_SERVER_PORT", "BBS_USER_GRPC_PORT", "BBS_USER_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_USER_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_USER_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "postgres.dsn", "BBS_USER_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_USER_POSTGRES_DEBUG")
	bindEnv(v, "trace.env", "BBS_USER_TRACE_ENV")
	bindEnv(v, "upstreams.content", "BBS_USER_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.contentInternalAuthToken", "BBS_USER_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.comment", "BBS_USER_UPSTREAMS_COMMENT")
	bindEnv(v, "upstreams.commentInternalAuthToken", "BBS_USER_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.reaction", "BBS_USER_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.reactionInternalAuthToken", "BBS_USER_UPSTREAMS_REACTION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.chat", "BBS_USER_UPSTREAMS_CHAT")
	bindEnv(v, "upstreams.chatInternalAuthToken", "BBS_USER_UPSTREAMS_CHAT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.notification", "BBS_USER_UPSTREAMS_NOTIFICATION")
	bindEnv(v, "upstreams.notificationInternalAuthToken", "BBS_USER_UPSTREAMS_NOTIFICATION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.file", "BBS_USER_UPSTREAMS_FILE")
	bindEnv(v, "upstreams.fileInternalAuthToken", "BBS_USER_UPSTREAMS_FILE_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.credit", "BBS_USER_UPSTREAMS_CREDIT")
	bindEnv(v, "upstreams.creditInternalAuthToken", "BBS_USER_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.feed", "BBS_USER_UPSTREAMS_FEED")
	bindEnv(v, "upstreams.feedInternalAuthToken", "BBS_USER_UPSTREAMS_FEED_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.search", "BBS_USER_UPSTREAMS_SEARCH")
	bindEnv(v, "upstreams.searchInternalAuthToken", "BBS_USER_UPSTREAMS_SEARCH_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.mall", "BBS_USER_UPSTREAMS_MALL")
	bindEnv(v, "upstreams.mallInternalAuthToken", "BBS_USER_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "accountDeletion.workerID", "BBS_USER_ACCOUNT_DELETION_WORKER_ID")
	bindEnv(v, "accountDeletion.pollInterval", "BBS_USER_ACCOUNT_DELETION_POLL_INTERVAL")
	bindEnv(v, "accountDeletion.lease", "BBS_USER_ACCOUNT_DELETION_LEASE")
	bindEnv(v, "accountDeletion.stepTimeout", "BBS_USER_ACCOUNT_DELETION_STEP_TIMEOUT")
	bindEnv(v, "accountDeletion.retryBase", "BBS_USER_ACCOUNT_DELETION_RETRY_BASE")
	bindEnv(v, "accountDeletion.maxAttempts", "BBS_USER_ACCOUNT_DELETION_MAX_ATTEMPTS")
	bindEnv(v, "accountDeletion.drainLimit", "BBS_USER_ACCOUNT_DELETION_DRAIN_LIMIT")
	bindEnv(v, "outbox.owner", "BBS_USER_OUTBOX_OWNER")
	bindEnv(v, "outbox.batchSize", "BBS_USER_OUTBOX_BATCH_SIZE")
	bindEnv(v, "outbox.leaseDuration", "BBS_USER_OUTBOX_LEASE_DURATION")
	bindEnv(v, "outbox.interval", "BBS_USER_OUTBOX_INTERVAL")
	bindEnv(v, "outbox.retryDelay", "BBS_USER_OUTBOX_RETRY_DELAY")
	bindEnv(v, "outbox.publishTimeout", "BBS_USER_OUTBOX_PUBLISH_TIMEOUT")
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
	bindEnv(v, "mfa.encryptionKey", "BBS_USER_MFA_ENCRYPTION_KEY")
	bindEnv(v, "mfa.issuer", "BBS_USER_MFA_ISSUER")
	bindEnv(v, "passkeys.rpId", "BBS_USER_PASSKEY_RP_ID")
	bindEnv(v, "passkeys.rpDisplayName", "BBS_USER_PASSKEY_RP_DISPLAY_NAME")
	bindEnv(v, "passkeys.ceremonyTTL", "BBS_USER_PASSKEY_CEREMONY_TTL")
	bindEnv(v, "snowflake.workerId", "BBS_USER_SNOWFLAKE_WORKER_ID")
	bindEnv(v, "snowflake.workerIdRangeStart", "BBS_USER_SNOWFLAKE_WORKER_ID_RANGE_START")
	bindEnv(v, "snowflake.workerIdRangeSize", "BBS_USER_SNOWFLAKE_WORKER_ID_RANGE_SIZE")
	bindEnv(v, "snowflake.instanceName", "BBS_USER_SNOWFLAKE_INSTANCE_NAME")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func setDefaults(v *viper.Viper) {
	setStringDefault(v, "upstreams.content", "bbs-content-service")
	setStringDefault(v, "upstreams.contentInternalAuthToken", localDevContentInternalAuthToken)
	setStringDefault(v, "upstreams.comment", "bbs-comment-service")
	setStringDefault(v, "upstreams.commentInternalAuthToken", localDevCommentInternalAuthToken)
	setStringDefault(v, "upstreams.reaction", "bbs-reaction-service")
	setStringDefault(v, "upstreams.reactionInternalAuthToken", localDevReactionInternalAuthToken)
	setStringDefault(v, "upstreams.chat", "bbs-chat-service")
	setStringDefault(v, "upstreams.chatInternalAuthToken", localDevChatInternalAuthToken)
	setStringDefault(v, "upstreams.notification", "bbs-notification-service")
	setStringDefault(v, "upstreams.notificationInternalAuthToken", localDevNotificationInternalAuthToken)
	setStringDefault(v, "upstreams.file", "bbs-file-service")
	setStringDefault(v, "upstreams.fileInternalAuthToken", localDevFileInternalAuthToken)
	setStringDefault(v, "upstreams.credit", "bbs-credit-service")
	setStringDefault(v, "upstreams.creditInternalAuthToken", localDevCreditInternalAuthToken)
	setStringDefault(v, "upstreams.feed", "bbs-feed-service")
	setStringDefault(v, "upstreams.feedInternalAuthToken", localDevFeedInternalAuthToken)
	setStringDefault(v, "upstreams.search", "bbs-search-service")
	setStringDefault(v, "upstreams.searchInternalAuthToken", localDevSearchInternalAuthToken)
	setStringDefault(v, "upstreams.mall", "bbs-mall-service")
	setStringDefault(v, "upstreams.mallInternalAuthToken", localDevMallInternalAuthToken)
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
}

func validate(v *viper.Viper) error {
	if _, err := snowflake.ResolveWorkerID(
		v.GetInt64("snowflake.workerId"),
		v.GetInt64("snowflake.workerIdRangeStart"),
		v.GetInt64("snowflake.workerIdRangeSize"),
		v.GetString("snowflake.instanceName"),
	); err != nil {
		return fmt.Errorf("user snowflake worker ID: %w", err)
	}
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
	for _, upstream := range []struct {
		key          string
		defaultValue string
	}{
		{key: "contentInternalAuthToken", defaultValue: localDevContentInternalAuthToken},
		{key: "commentInternalAuthToken", defaultValue: localDevCommentInternalAuthToken},
		{key: "reactionInternalAuthToken", defaultValue: localDevReactionInternalAuthToken},
		{key: "chatInternalAuthToken", defaultValue: localDevChatInternalAuthToken},
		{key: "notificationInternalAuthToken", defaultValue: localDevNotificationInternalAuthToken},
		{key: "fileInternalAuthToken", defaultValue: localDevFileInternalAuthToken},
		{key: "creditInternalAuthToken", defaultValue: localDevCreditInternalAuthToken},
		{key: "feedInternalAuthToken", defaultValue: localDevFeedInternalAuthToken},
		{key: "searchInternalAuthToken", defaultValue: localDevSearchInternalAuthToken},
	} {
		key := "upstreams." + upstream.key
		value := strings.TrimSpace(v.GetString(key))
		if value == "" || value == upstream.defaultValue {
			return fmt.Errorf("%s must be set to a non-default value in production", key)
		}
		if len([]byte(value)) < minProductionInternalAuthTokenBytes {
			return fmt.Errorf("%s must be at least %d bytes in production", key, minProductionInternalAuthTokenBytes)
		}
	}
	mfaKey := strings.TrimSpace(v.GetString("mfa.encryptionKey"))
	if len([]byte(mfaKey)) < minProductionMFAEncryptionKeyBytes {
		return fmt.Errorf("mfa.encryptionKey must be at least %d bytes in production", minProductionMFAEncryptionKeyBytes)
	}
	rpID := strings.ToLower(strings.TrimSpace(v.GetString("passkeys.rpId")))
	if rpID == "" || rpID == "localhost" || net.ParseIP(rpID) != nil {
		return fmt.Errorf("passkeys.rpId must be a production domain")
	}
	if strings.TrimSpace(v.GetString("passkeys.rpDisplayName")) == "" {
		return fmt.Errorf("passkeys.rpDisplayName must be set in production")
	}
	origins := v.GetStringSlice("passkeys.origins")
	if len(origins) == 0 {
		return fmt.Errorf("passkeys.origins must contain at least one HTTPS origin in production")
	}
	for _, origin := range origins {
		parsed, err := url.Parse(strings.TrimSpace(origin))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("passkeys.origins must contain only HTTPS origins in production")
		}
		host := strings.ToLower(parsed.Hostname())
		if host != rpID && !strings.HasSuffix(host, "."+rpID) {
			return fmt.Errorf("passkeys origin %q is not within RP ID %q", origin, rpID)
		}
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
	postgresOverrides := map[string]interface{}{}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_POSTGRES_DSN")); value != "" {
		postgresOverrides["dsn"] = value
	}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_POSTGRES_DEBUG")); value != "" {
		postgresOverrides["debug"] = value
	}
	if len(postgresOverrides) > 0 {
		overrides["postgres"] = postgresOverrides
	}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_KAFKA_BROKERS")); value != "" {
		overrides["kafka"] = map[string]interface{}{"brokers": splitCommaSeparated(value)}
	}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_PASSKEY_ORIGINS")); value != "" {
		overrides["passkeys"] = map[string]interface{}{"origins": splitCommaSeparated(value)}
	}
	grpcServerOverrides := map[string]interface{}{}
	grpcClientOverrides := map[string]interface{}{}
	serviceOverrides := map[string]interface{}{}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_GRPC_SERVER_ETCD_ADDR")); value != "" {
		grpcServerOverrides["etcdAddr"] = splitCommaSeparated(value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		grpcClientOverrides["etcdAddr"] = splitCommaSeparated(value)
	}
	port, err := grpcPortOverride()
	if err != nil {
		return err
	}
	if port > 0 {
		grpcServerOverrides["port"] = port
		serviceOverrides["grpcPort"] = port
	}
	grpcOverrides := map[string]interface{}{}
	if len(grpcServerOverrides) > 0 {
		grpcOverrides["server"] = grpcServerOverrides
	}
	if len(grpcClientOverrides) > 0 {
		grpcOverrides["client"] = grpcClientOverrides
	}
	if len(grpcOverrides) > 0 {
		overrides["grpc"] = grpcOverrides
	}
	if len(serviceOverrides) > 0 {
		overrides["service"] = serviceOverrides
	}
	if len(overrides) == 0 {
		return nil
	}
	applyOverrideTree(v, "", overrides)
	return nil
}

// grpcPortOverride returns the gRPC port requested through the environment, or 0
// when no override is configured.
func grpcPortOverride() (int, error) {
	value := firstEnv("BBS_USER_GRPC_SERVER_PORT", "BBS_USER_GRPC_PORT", "BBS_USER_SERVICE_GRPC_PORT")
	if value == "" {
		return 0, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse user gRPC server port: %w", err)
	}
	if port <= 0 {
		return 0, fmt.Errorf("user gRPC server port must be greater than zero: %d", port)
	}
	return port, nil
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
		setNestedConfigValue(v, key, fallback)
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

// applyOverrideTree writes every leaf of tree through setNestedConfigValue.
//
// MergeConfigMap would land these values in viper's config layer, which
// AutomaticEnv/BindEnv outrank, so a comma-split list such as kafka.brokers would
// collapse back into the single raw env string on a flat GetStringSlice read.
func applyOverrideTree(v *viper.Viper, prefix string, tree map[string]interface{}) {
	for key, value := range tree {
		full := key
		if prefix != "" {
			full = prefix + "." + key
		}
		if sub, ok := value.(map[string]interface{}); ok {
			applyOverrideTree(v, full, sub)
			continue
		}
		setNestedConfigValue(v, full, value)
	}
}

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
