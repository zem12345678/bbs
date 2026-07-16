package app

import (
	"strings"
	"time"

	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	mallclient "user-service/internal/clients/mall"
	"user-service/internal/infrastructure/messaging"
	"user-service/internal/infrastructure/persistence"
	iocgrpc "user-service/internal/ioc/grpc"
	"user-service/pkg/logger"
	"user-service/pkg/snowflake"

	"github.com/google/wire"
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
	return snowflake.NewNode(workerID)
}

func ProvideEventPublisher(writer *kafka.Writer, log logger.Logger) *messaging.KafkaEventPublisher {
	return messaging.NewKafkaEventPublisher(writer, log)
}

func ProvideProfileThemeEntitlementReader(grpcClient *iocgrpc.Client, v *viper.Viper) (command.ProfileThemeEntitlementReader, error) {
	return mallclient.NewClient(grpcClient, v)
}

func ProvideCommandService(repo *persistence.Repo, idgen *snowflake.Node, publisher *messaging.KafkaEventPublisher, log logger.Logger, v *viper.Viper, themeEntitlements command.ProfileThemeEntitlementReader) *command.Service {
	jwtTTL, err := DurationDefault(v, "jwt.ttl", 7*24*time.Hour)
	if err != nil {
		jwtTTL = 7 * 24 * time.Hour
	}
	return command.NewService(
		repo,
		idgen,
		publisher,
		log,
		StringDefault(v.GetString("jwt.secret"), "bbs-local-dev-secret"),
		jwtTTL,
		IntDefault(v.GetInt("password.minLength"), 8),
		themeEntitlements,
	)
}

func ProvideQueryService(repo *persistence.Repo, themeEntitlements command.ProfileThemeEntitlementReader) *query.Service {
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
	ProvideCommandService,
	ProvideQueryService,
)
