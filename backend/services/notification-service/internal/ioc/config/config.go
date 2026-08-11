package config

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/url"
	"notification-service/pkg/uuid"
	"os"
	"strconv"
	"strings"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	localDevInternalAuthToken           = "bbs-local-notification-internal-token"
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
	if err := applyGRPCPortEnvOverride(v,
		"BBS_NOTIFICATION_GRPC_SERVER_PORT",
		"BBS_NOTIFICATION_SERVICE_GRPC_PORT",
	); err != nil {
		return nil, err
	}
	setInternalAuthDefault(v)
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

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_NOTIFICATION")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bindEnv(v, "service.name", "BBS_NOTIFICATION_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_NOTIFICATION_SERVICE_GRPC_PORT")
	bindEnv(v, "app.name", "BBS_NOTIFICATION_APP_NAME", "BBS_NOTIFICATION_SERVICE_NAME")
	bindEnv(v, "postgres.dsn", "BBS_NOTIFICATION_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_NOTIFICATION_POSTGRES_DEBUG")
	bindEnv(v, "postgres.max_open_conns", "BBS_NOTIFICATION_POSTGRES_MAX_OPEN_CONNS")
	bindEnv(v, "webPush.enabled", "BBS_NOTIFICATION_WEB_PUSH_ENABLED")
	bindEnv(v, "webPush.subject", "BBS_NOTIFICATION_WEB_PUSH_SUBJECT")
	bindEnv(v, "webPush.publicKey", "BBS_NOTIFICATION_WEB_PUSH_PUBLIC_KEY")
	bindEnv(v, "webPush.privateKey", "BBS_NOTIFICATION_WEB_PUSH_PRIVATE_KEY")
	bindEnv(v, "webhook.enabled", "BBS_NOTIFICATION_WEBHOOK_ENABLED")
	bindEnv(v, "webhook.serverURL", "BBS_NOTIFICATION_WEBHOOK_SERVER_URL")
	bindEnv(v, "webhook.allowPrivateEndpoints", "BBS_NOTIFICATION_WEBHOOK_ALLOW_PRIVATE_ENDPOINTS")
	bindEnv(v, "kafka.brokers", "BBS_NOTIFICATION_KAFKA_BROKERS")
	bindEnv(v, "kafka.username", "BBS_NOTIFICATION_KAFKA_USERNAME")
	bindEnv(v, "kafka.password", "BBS_NOTIFICATION_KAFKA_PASSWORD")
	bindEnv(v, "kafka.scram_algorithm", "BBS_NOTIFICATION_KAFKA_SCRAM_ALGORITHM")
	bindEnv(v, "kafka.userTopic", "BBS_NOTIFICATION_KAFKA_USER_TOPIC")
	bindEnv(v, "kafka.articleTopic", "BBS_NOTIFICATION_KAFKA_ARTICLE_TOPIC")
	bindEnv(v, "kafka.commentTopic", "BBS_NOTIFICATION_KAFKA_COMMENT_TOPIC")
	bindEnv(v, "kafka.reactionTopic", "BBS_NOTIFICATION_KAFKA_REACTION_TOPIC")
	bindEnv(v, "kafka.mallTopic", "BBS_NOTIFICATION_KAFKA_MALL_TOPIC")
	bindEnv(v, "kafka.userGroupId", "BBS_NOTIFICATION_KAFKA_USER_GROUP_ID")
	bindEnv(v, "kafka.articleGroupId", "BBS_NOTIFICATION_KAFKA_ARTICLE_GROUP_ID")
	bindEnv(v, "kafka.commentGroupId", "BBS_NOTIFICATION_KAFKA_COMMENT_GROUP_ID")
	bindEnv(v, "kafka.reactionGroupId", "BBS_NOTIFICATION_KAFKA_REACTION_GROUP_ID")
	bindEnv(v, "kafka.mallGroupId", "BBS_NOTIFICATION_KAFKA_MALL_GROUP_ID")
	bindEnv(v, "grpc.server.port", "BBS_NOTIFICATION_GRPC_SERVER_PORT", "BBS_NOTIFICATION_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_NOTIFICATION_GRPC_SERVER_SERVICE_NAME", "BBS_NOTIFICATION_SERVICE_NAME")
	bindEnv(v, "grpc.server.etcdAddr", "BBS_NOTIFICATION_GRPC_SERVER_ETCD_ADDR")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_NOTIFICATION_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_NOTIFICATION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "grpc.client.etcdAddr", "BBS_NOTIFICATION_GRPC_CLIENT_ETCD_ADDR")
	bindEnv(v, "trace.grpcEndpoint", "BBS_NOTIFICATION_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.serviceName", "BBS_NOTIFICATION_TRACE_SERVICE_NAME", "BBS_NOTIFICATION_SERVICE_NAME")
	bindEnv(v, "trace.version", "BBS_NOTIFICATION_TRACE_VERSION")
	bindEnv(v, "trace.env", "BBS_NOTIFICATION_TRACE_ENV")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	setStringEnv(v, "service.name", "BBS_NOTIFICATION_SERVICE_NAME")
	setStringEnv(v, "app.name", "BBS_NOTIFICATION_APP_NAME")
	setStringEnv(v, "postgres.dsn", "BBS_NOTIFICATION_POSTGRES_DSN")
	setStringEnv(v, "postgres.debug", "BBS_NOTIFICATION_POSTGRES_DEBUG")
	setStringEnv(v, "postgres.max_open_conns", "BBS_NOTIFICATION_POSTGRES_MAX_OPEN_CONNS")
	setStringEnv(v, "webPush.enabled", "BBS_NOTIFICATION_WEB_PUSH_ENABLED")
	setStringEnv(v, "webPush.subject", "BBS_NOTIFICATION_WEB_PUSH_SUBJECT")
	setStringEnv(v, "webPush.publicKey", "BBS_NOTIFICATION_WEB_PUSH_PUBLIC_KEY")
	setStringEnv(v, "webPush.privateKey", "BBS_NOTIFICATION_WEB_PUSH_PRIVATE_KEY")
	setStringEnv(v, "webhook.serverURL", "BBS_NOTIFICATION_WEBHOOK_SERVER_URL")
	setBoolEnv(v, "webhook.enabled", "BBS_NOTIFICATION_WEBHOOK_ENABLED")
	setBoolEnv(v, "webhook.allowPrivateEndpoints", "BBS_NOTIFICATION_WEBHOOK_ALLOW_PRIVATE_ENDPOINTS")
	if value := strings.TrimSpace(os.Getenv("BBS_NOTIFICATION_KAFKA_BROKERS")); value != "" {
		setNestedConfigValue(v, "kafka.brokers", splitCommaSeparated(value))
	}
	setStringEnv(v, "kafka.username", "BBS_NOTIFICATION_KAFKA_USERNAME")
	setStringEnv(v, "kafka.password", "BBS_NOTIFICATION_KAFKA_PASSWORD")
	setStringEnv(v, "kafka.scram_algorithm", "BBS_NOTIFICATION_KAFKA_SCRAM_ALGORITHM")
	setStringEnv(v, "kafka.userTopic", "BBS_NOTIFICATION_KAFKA_USER_TOPIC")
	setStringEnv(v, "kafka.articleTopic", "BBS_NOTIFICATION_KAFKA_ARTICLE_TOPIC")
	setStringEnv(v, "kafka.commentTopic", "BBS_NOTIFICATION_KAFKA_COMMENT_TOPIC")
	setStringEnv(v, "kafka.reactionTopic", "BBS_NOTIFICATION_KAFKA_REACTION_TOPIC")
	setStringEnv(v, "kafka.mallTopic", "BBS_NOTIFICATION_KAFKA_MALL_TOPIC")
	setStringEnv(v, "kafka.userGroupId", "BBS_NOTIFICATION_KAFKA_USER_GROUP_ID")
	setStringEnv(v, "kafka.articleGroupId", "BBS_NOTIFICATION_KAFKA_ARTICLE_GROUP_ID")
	setStringEnv(v, "kafka.commentGroupId", "BBS_NOTIFICATION_KAFKA_COMMENT_GROUP_ID")
	setStringEnv(v, "kafka.reactionGroupId", "BBS_NOTIFICATION_KAFKA_REACTION_GROUP_ID")
	setStringEnv(v, "kafka.mallGroupId", "BBS_NOTIFICATION_KAFKA_MALL_GROUP_ID")
	if value := strings.TrimSpace(os.Getenv("BBS_NOTIFICATION_GRPC_SERVER_ETCD_ADDR")); value != "" {
		setNestedConfigValue(v, "grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_NOTIFICATION_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		setNestedConfigValue(v, "grpc.client.etcdAddr", splitCommaSeparated(value))
	}
	if value := firstNonEmptyEnv("BBS_NOTIFICATION_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_NOTIFICATION_INTERNAL_AUTH_TOKEN"); value != "" {
		setNestedConfigValue(v, "grpc.server.internalAuthToken", value)
	}
	setStringEnv(v, "trace.grpcEndpoint", "BBS_NOTIFICATION_TRACE_GRPC_ENDPOINT")
	setStringEnv(v, "trace.serviceName", "BBS_NOTIFICATION_TRACE_SERVICE_NAME")
	setStringEnv(v, "trace.version", "BBS_NOTIFICATION_TRACE_VERSION")
	setStringEnv(v, "trace.env", "BBS_NOTIFICATION_TRACE_ENV")
}

func setStringEnv(v *viper.Viper, key string, env string) {
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		setNestedConfigValue(v, key, value)
	}
}

func setBoolEnv(v *viper.Viper, key string, env string) {
	value := strings.TrimSpace(os.Getenv(env))
	if value == "" {
		return
	}
	parsed, err := strconv.ParseBool(value)
	if err == nil {
		setNestedConfigValue(v, key, parsed)
	}
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
	setNestedConfigValue(v, "service.grpcPort", port)
	setNestedConfigValue(v, "grpc.server.port", port)
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

func setInternalAuthDefault(v *viper.Viper) {
	if strings.TrimSpace(v.GetString("grpc.server.internalAuthToken")) == "" {
		setNestedConfigValue(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
	}
}

func validate(v *viper.Viper) error {
	if err := validateWebPush(v); err != nil {
		return err
	}
	environment := strings.ToLower(strings.TrimSpace(v.GetString("trace.env")))
	production := environment == "production" || environment == "prod"
	if err := validateWebhook(v, production); err != nil {
		return err
	}
	if !production {
		return nil
	}
	return validateProductionInternalAuthToken(v.GetString("grpc.server.internalAuthToken"))
}

func validateWebhook(v *viper.Viper, production bool) error {
	if !v.GetBool("webhook.enabled") {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(v.GetString("webhook.serverURL")))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("webhook.serverURL must be an absolute http or https URL")
	}
	if production && parsed.Scheme != "https" {
		return errors.New("webhook.serverURL must use https in production")
	}
	if production && v.GetBool("webhook.allowPrivateEndpoints") {
		return errors.New("webhook.allowPrivateEndpoints must be false in production")
	}
	return nil
}

func validateWebPush(v *viper.Viper) error {
	if !v.GetBool("webPush.enabled") {
		return nil
	}
	subject := strings.TrimSpace(v.GetString("webPush.subject"))
	publicKey := strings.TrimSpace(v.GetString("webPush.publicKey"))
	privateKey := strings.TrimSpace(v.GetString("webPush.privateKey"))
	if subject == "" || publicKey == "" || privateKey == "" {
		return errors.New("webPush.subject, webPush.publicKey and webPush.privateKey are required when web push is enabled")
	}
	parsedSubject, err := url.Parse(subject)
	if err != nil || (parsedSubject.Scheme != "mailto" && parsedSubject.Scheme != "https") ||
		(parsedSubject.Scheme == "mailto" && parsedSubject.Opaque == "") ||
		(parsedSubject.Scheme == "https" && parsedSubject.Host == "") {
		return errors.New("webPush.subject must be a mailto or https URI")
	}
	decodedPublic, err := decodeWebPushKey(publicKey)
	if err != nil {
		return errors.New("webPush.publicKey must be a valid uncompressed P-256 public key")
	}
	decodedPrivate, err := decodeWebPushKey(privateKey)
	if err != nil {
		return errors.New("webPush.privateKey must be a valid P-256 private key")
	}
	if err := validateVAPIDKeyPair(decodedPublic, decodedPrivate); err != nil {
		return err
	}
	return nil
}

func validateVAPIDKeyPair(publicKey, privateKey []byte) error {
	curve := elliptic.P256()
	publicX, publicY := elliptic.Unmarshal(curve, publicKey)
	if publicX == nil || publicY == nil {
		return errors.New("webPush.publicKey must be a valid uncompressed P-256 public key")
	}
	privateScalar := new(big.Int).SetBytes(privateKey)
	if len(privateKey) != 32 || privateScalar.Sign() <= 0 || privateScalar.Cmp(curve.Params().N) >= 0 {
		return errors.New("webPush.privateKey must be a valid P-256 private key")
	}
	expectedX, expectedY := curve.ScalarBaseMult(privateKey)
	if publicX.Cmp(expectedX) != 0 || publicY.Cmp(expectedY) != 0 {
		return errors.New("webPush.publicKey and webPush.privateKey must be a matching P-256 key pair")
	}
	return nil
}

func decodeWebPushKey(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(value)
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

func stringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_NOTIFICATION_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
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
