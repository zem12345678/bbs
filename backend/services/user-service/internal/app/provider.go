package app

import (
	"strings"
	"time"

	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	mallclient "user-service/internal/clients/mall"
	credential "user-service/internal/infrastructure/credential"
	securityemail "user-service/internal/infrastructure/email"
	"user-service/internal/infrastructure/messaging"
	mfasecurity "user-service/internal/infrastructure/mfa"
	passkeysecurity "user-service/internal/infrastructure/passkey"
	"user-service/internal/infrastructure/persistence"
	iocgrpc "user-service/internal/ioc/grpc"
	"user-service/pkg/logger"
	"user-service/pkg/snowflake"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideRepository(db *gorm.DB) *persistence.Repo {
	return persistence.NewRepo(db)
}

func ProvideIDGenerator(v *viper.Viper) (*snowflake.Node, error) {
	workerID := v.GetInt64("snowflake.workerId")
	if workerID == 0 {
		workerID = 2
	}
	var err error
	workerID, err = snowflake.ResolveWorkerID(
		workerID,
		v.GetInt64("snowflake.workerIdRangeStart"),
		v.GetInt64("snowflake.workerIdRangeSize"),
		v.GetString("snowflake.instanceName"),
	)
	if err != nil {
		return nil, err
	}
	return snowflake.NewNode(workerID)
}

func ProvideEventPublisher(writer *kafka.Writer, log logger.Logger) *messaging.KafkaEventPublisher {
	return messaging.NewKafkaEventPublisher(writer, log)
}

func ProvideProfileThemeEntitlementReader(grpcClient *iocgrpc.Client, v *viper.Viper) (*mallclient.Client, error) {
	return mallclient.NewClient(grpcClient, v)
}

func ProvideSecurityEmailSender(v *viper.Viper) (command.SecurityEmailSender, error) {
	return securityemail.New(securityemail.NewOptions(v))
}

func ProvideCredentialVersionCache(client *redis.Client) *credential.Store {
	return credential.NewStore(client)
}

func ProvideMFAManager(v *viper.Viper) (command.MFAManager, error) {
	return mfasecurity.New(
		StringDefault(v.GetString("mfa.encryptionKey"), "bbs-local-user-mfa-encryption-key"),
		StringDefault(v.GetString("mfa.issuer"), "BBS Community"),
	)
}

func ProvidePasskeyManager(v *viper.Viper) (command.PasskeyManager, error) {
	ceremonyTTL, err := DurationDefault(v, "passkeys.ceremonyTTL", 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return passkeysecurity.New(passkeysecurity.Options{
		RPID:          StringDefault(v.GetString("passkeys.rpId"), "127.0.0.1"),
		RPDisplayName: StringDefault(v.GetString("passkeys.rpDisplayName"), "BBS Community"),
		RPOrigins:     StringSliceDefault(v.GetStringSlice("passkeys.origins"), []string{"http://127.0.0.1:8850"}),
		EncryptionKey: StringDefault(v.GetString("mfa.encryptionKey"), "bbs-local-user-mfa-encryption-key"),
		CeremonyTTL:   ceremonyTTL,
	})
}

func ProvideCommandService(repo *persistence.Repo, idgen *snowflake.Node, publisher *messaging.KafkaEventPublisher, log logger.Logger, v *viper.Viper, themeEntitlements command.ProfileThemeEntitlementReader, securityEmails command.SecurityEmailSender, credentialVersions *credential.Store, mfaManager command.MFAManager, passkeyManager command.PasskeyManager) *command.Service {
	jwtTTL, err := DurationDefault(v, "jwt.ttl", 7*24*time.Hour)
	if err != nil {
		jwtTTL = 7 * 24 * time.Hour
	}
	return command.NewServiceWithPasskeys(
		repo,
		idgen,
		publisher,
		log,
		StringDefault(v.GetString("jwt.secret"), "bbs-local-dev-secret"),
		jwtTTL,
		IntDefault(v.GetInt("password.minLength"), 8),
		themeEntitlements,
		securityEmails,
		credentialVersions,
		mfaManager,
		passkeyManager,
	)
}

func ProvideQueryService(repo *persistence.Repo, themeEntitlements query.ProfileEntitlementReader) *query.Service {
	return query.NewService(repo, themeEntitlements)
}

func StringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func StringSliceDefault(values []string, fallback []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func IntDefault(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func DurationDefault(v *viper.Viper, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(v.GetString(key))
	if raw != "" {
		return time.ParseDuration(raw)
	}
	if value := v.GetDuration(key); value > 0 {
		return value, nil
	}
	return fallback, nil
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideRepository,
	ProvideIDGenerator,
	ProvideEventPublisher,
	ProvideProfileThemeEntitlementReader,
	wire.Bind(new(command.ProfileThemeEntitlementReader), new(*mallclient.Client)),
	wire.Bind(new(query.ProfileEntitlementReader), new(*mallclient.Client)),
	ProvideSecurityEmailSender,
	ProvideCredentialVersionCache,
	ProvideMFAManager,
	ProvidePasskeyManager,
	ProvideCommandService,
	ProvideQueryService,
)
