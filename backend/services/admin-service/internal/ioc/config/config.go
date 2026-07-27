package config

import (
	"admin/pkg/uuid"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

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

const (
	localDevJWTSecret                     = "bbs-admin-local-dev-secret"
	localDevDefaultAdminPassword          = "Admin123!"
	localDevSecretEncryptionKey           = "bbs-admin-local-setting-secret"
	localDevInternalAuthToken             = "bbs-local-admin-internal-token"
	localDevUserInternalAuthToken         = "bbs-local-user-internal-token"
	localDevReactionInternalAuthToken     = "bbs-local-reaction-internal-token"
	localDevContentInternalAuthToken      = "bbs-local-content-internal-token"
	localDevCommentInternalAuthToken      = "bbs-local-comment-internal-token"
	localDevNotificationInternalAuthToken = "bbs-local-notification-internal-token"
	localDevSearchInternalAuthToken       = "bbs-local-search-internal-token"
	minProductionSecretBytes              = 32
	minProductionInternalAuthTokenBytes   = 32
)

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
		cc := constant.ClientConfig{
			NamespaceId:         o.NamespaceID,
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			LogDir:              "tmp/nacos/log",
			CacheDir:            "tmp/nacos/cache",
			LogLevel:            "debug",
		}

		configClient, err := clients.CreateConfigClient(map[string]interface{}{
			"serverConfigs": sc,
			"clientConfig":  cc,
		})
		if err != nil {
			return nil, err
		}
		content, err := configClient.GetConfig(vo.ConfigParam{
			DataId: o.DataID,
			Group:  group})
		if err != nil {
			return nil, err
		}
		if err = v.MergeConfig(bytes.NewBufferString(content)); err != nil {
			return nil, errors.Wrap(err, "viper read nacos config error")
		}

		err = configClient.ListenConfig(vo.ConfigParam{
			DataId: o.DataID,
			Group:  group,
			OnChange: func(namespace, group, dataID, data string) {
				_ = namespace
				_ = group
				_ = dataID
				_ = v.MergeConfig(bytes.NewBufferString(data))
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "listenConfig nacos config error")
		}
	}
	applyEnvOverrides(v)
	if err := applyGRPCPortEnvOverride(v,
		"BBS_ADMIN_GRPC_SERVER_PORT",
		"BBS_ADMIN_SERVICE_GRPC_PORT",
	); err != nil {
		return nil, err
	}
	uuidstr, err := uuid.GetHostUuid()
	if err != nil || uuidstr == "" {
		fmt.Println("new uuid")
		uuidstr, err = uuid.NewUUID()
	}
	setDefaults(v)
	if err := validateProductionSecurityConfig(v); err != nil {
		return nil, err
	}
	v.Set("server.uuid", uuidstr)
	return v, err
}

// skipNacos makes the mounted runtime Secret the only startup configuration
// source for immutable deployments. Local development continues to use Nacos
// unless this explicit process environment switch is set.
func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_ADMIN_SKIP_NACOS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_ADMIN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "auth.jwtSecret", "BBS_ADMIN_AUTH_JWT_SECRET")
	bindEnv(v, "auth.jwtTtl", "BBS_ADMIN_AUTH_JWT_TTL")
	bindEnv(v, "auth.refreshTtl", "BBS_ADMIN_AUTH_REFRESH_TTL")
	bindEnv(v, "auth.defaultAdminPassword", "BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD")
	bindEnv(v, "auth.secretEncryptionKey", "BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY")
	bindEnv(v, "grpc.server.internalAuthToken", "BBS_ADMIN_GRPC_SERVER_INTERNAL_AUTH_TOKEN", "BBS_ADMIN_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "grpc.server.tls.enabled", "BBS_ADMIN_GRPC_SERVER_TLS_ENABLED")
	bindEnv(v, "grpc.server.tls.certFile", "BBS_ADMIN_GRPC_SERVER_TLS_CERT_FILE")
	bindEnv(v, "grpc.server.tls.keyFile", "BBS_ADMIN_GRPC_SERVER_TLS_KEY_FILE")
	bindEnv(v, "grpc.server.tls.clientCAFile", "BBS_ADMIN_GRPC_SERVER_TLS_CLIENT_CA_FILE")
	bindEnv(v, "service.grpcPort", "BBS_ADMIN_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_ADMIN_GRPC_SERVER_PORT", "BBS_ADMIN_SERVICE_GRPC_PORT")
	bindEnv(v, "trace.env", "BBS_ADMIN_TRACE_ENV")
	bindEnv(v, "upstreams.user", "BBS_ADMIN_UPSTREAMS_USER")
	bindEnv(v, "upstreams.userInternalAuthToken", "BBS_ADMIN_UPSTREAMS_USER_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.reaction", "BBS_ADMIN_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.reactionInternalAuthToken", "BBS_ADMIN_UPSTREAMS_REACTION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.content", "BBS_ADMIN_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.contentInternalAuthToken", "BBS_ADMIN_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.comment", "BBS_ADMIN_UPSTREAMS_COMMENT")
	bindEnv(v, "upstreams.commentInternalAuthToken", "BBS_ADMIN_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.notification", "BBS_ADMIN_UPSTREAMS_NOTIFICATION")
	bindEnv(v, "upstreams.notificationInternalAuthToken", "BBS_ADMIN_UPSTREAMS_NOTIFICATION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.search", "BBS_ADMIN_UPSTREAMS_SEARCH")
	bindEnv(v, "upstreams.searchInternalAuthToken", "BBS_ADMIN_UPSTREAMS_SEARCH_INTERNAL_AUTH_TOKEN")
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_ADMIN_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_ADMIN_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
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
	setStringDefault(v, "auth.jwtSecret", localDevJWTSecret)
	setStringDefault(v, "auth.jwtTtl", "168h")
	setStringDefault(v, "auth.refreshTtl", "720h")
	setStringDefault(v, "auth.defaultAdminPassword", localDevDefaultAdminPassword)
	setStringDefault(v, "auth.secretEncryptionKey", localDevSecretEncryptionKey)
	setStringDefault(v, "grpc.server.internalAuthToken", localDevInternalAuthToken)
	if !v.IsSet("grpc.server.tls.enabled") {
		v.Set("grpc.server.tls.enabled", false)
	}
	setStringDefault(v, "upstreams.user", "bbs-user-service")
	setStringDefault(v, "upstreams.userInternalAuthToken", localDevUserInternalAuthToken)
	setStringDefault(v, "upstreams.reaction", "bbs-reaction-service")
	setStringDefault(v, "upstreams.reactionInternalAuthToken", localDevReactionInternalAuthToken)
	setStringDefault(v, "upstreams.content", "bbs-content-service")
	setStringDefault(v, "upstreams.contentInternalAuthToken", localDevContentInternalAuthToken)
	setStringDefault(v, "upstreams.comment", "bbs-comment-service")
	setStringDefault(v, "upstreams.commentInternalAuthToken", localDevCommentInternalAuthToken)
	setStringDefault(v, "upstreams.notification", "bbs-notification-service")
	setStringDefault(v, "upstreams.notificationInternalAuthToken", localDevNotificationInternalAuthToken)
	setStringDefault(v, "upstreams.search", "bbs-search-service")
	setStringDefault(v, "upstreams.searchInternalAuthToken", localDevSearchInternalAuthToken)
}

func validateProductionSecurityConfig(v *viper.Viper) error {
	if !isProductionEnvironment(v.GetString("trace.env")) {
		return nil
	}

	jwtSecret := strings.TrimSpace(v.GetString("auth.jwtSecret"))
	if err := validateProductionSecret("auth.jwtSecret", jwtSecret); err != nil {
		return err
	}

	encryptionKey := strings.TrimSpace(v.GetString("auth.secretEncryptionKey"))
	if err := validateProductionSecret("auth.secretEncryptionKey", encryptionKey); err != nil {
		return err
	}
	if encryptionKey == jwtSecret {
		return fmt.Errorf("auth.secretEncryptionKey must differ from auth.jwtSecret in production")
	}

	bootstrapPassword := strings.TrimSpace(v.GetString("auth.defaultAdminPassword"))
	if bootstrapPassword == "" || bootstrapPassword == localDevDefaultAdminPassword {
		return fmt.Errorf("auth.defaultAdminPassword must be set to a non-default value in production")
	}
	if !validBootstrapAdminPassword(bootstrapPassword) {
		return fmt.Errorf("auth.defaultAdminPassword must be 8-64 characters and contain letters, digits, and special characters in production")
	}
	internalAuthToken := strings.TrimSpace(v.GetString("grpc.server.internalAuthToken"))
	if err := validateProductionInternalAuthToken("grpc.server.internalAuthToken", internalAuthToken, localDevInternalAuthToken); err != nil {
		return err
	}
	userInternalAuthToken := strings.TrimSpace(v.GetString("upstreams.userInternalAuthToken"))
	if err := validateProductionInternalAuthToken("upstreams.userInternalAuthToken", userInternalAuthToken, localDevUserInternalAuthToken); err != nil {
		return err
	}
	reactionInternalAuthToken := strings.TrimSpace(v.GetString("upstreams.reactionInternalAuthToken"))
	if err := validateProductionInternalAuthToken("upstreams.reactionInternalAuthToken", reactionInternalAuthToken, localDevReactionInternalAuthToken); err != nil {
		return err
	}
	notificationInternalAuthToken := strings.TrimSpace(v.GetString("upstreams.notificationInternalAuthToken"))
	if err := validateProductionInternalAuthToken("upstreams.notificationInternalAuthToken", notificationInternalAuthToken, localDevNotificationInternalAuthToken); err != nil {
		return err
	}
	searchInternalAuthToken := strings.TrimSpace(v.GetString("upstreams.searchInternalAuthToken"))
	if err := validateProductionInternalAuthToken("upstreams.searchInternalAuthToken", searchInternalAuthToken, localDevSearchInternalAuthToken); err != nil {
		return err
	}
	commentInternalAuthToken := strings.TrimSpace(v.GetString("upstreams.commentInternalAuthToken"))
	if err := validateProductionInternalAuthToken("upstreams.commentInternalAuthToken", commentInternalAuthToken, localDevCommentInternalAuthToken); err != nil {
		return err
	}
	contentInternalAuthToken := strings.TrimSpace(v.GetString("upstreams.contentInternalAuthToken"))
	if err := validateProductionInternalAuthToken("upstreams.contentInternalAuthToken", contentInternalAuthToken, localDevContentInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionServerTLS(v); err != nil {
		return err
	}
	return nil
}

func validateProductionSecret(key, value string) error {
	if value == "" || value == localDevJWTSecret || value == localDevSecretEncryptionKey {
		return fmt.Errorf("%s must be set to a non-default value in production", key)
	}
	if len([]byte(value)) < minProductionSecretBytes {
		return fmt.Errorf("%s must be at least %d bytes in production", key, minProductionSecretBytes)
	}
	return nil
}

func validateProductionInternalAuthToken(key, value, localDefault string) error {
	if value == "" || value == localDefault {
		return fmt.Errorf("%s must be set to a non-default value in production", key)
	}
	if len([]byte(value)) < minProductionInternalAuthTokenBytes {
		return fmt.Errorf("%s must be at least %d bytes in production", key, minProductionInternalAuthTokenBytes)
	}
	return nil
}

func validateProductionServerTLS(v *viper.Viper) error {
	if !v.GetBool("grpc.server.tls.enabled") {
		return fmt.Errorf("grpc.server.tls.enabled must be true in production")
	}
	for _, key := range []string{"grpc.server.tls.certFile", "grpc.server.tls.keyFile", "grpc.server.tls.clientCAFile"} {
		if strings.TrimSpace(v.GetString(key)) == "" {
			return fmt.Errorf("%s is required when grpc server TLS is enabled", key)
		}
	}
	return nil
}

func validBootstrapAdminPassword(password string) bool {
	runes := []rune(password)
	if len(runes) < 8 || len(runes) > 64 {
		return false
	}
	var hasLetter, hasDigit, hasSpecial bool
	for _, r := range runes {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	return hasLetter && hasDigit && hasSpecial
}

func isProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return true
	default:
		return false
	}
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
