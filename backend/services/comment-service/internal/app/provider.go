package app

import (
	"context"
	"strings"

	"comment-service/internal/application/comment/command"
	"comment-service/internal/application/comment/query"
	"comment-service/internal/infrastructure/messaging"
	"comment-service/internal/infrastructure/persistence"
	iocmongo "comment-service/internal/ioc/db/mongo"
	"comment-service/pkg/logger"
	"comment-service/pkg/snowflake"

	"github.com/google/wire"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideRepository(ctx context.Context, mongodb *iocmongo.MongoDB) (*persistence.Repository, error) {
	// MongoDB index creation is an explicit operational migration. A normal
	// replica startup must not attempt a potentially long-running index build.
	return persistence.NewRepository(mongodb.DB), nil
}

func ProvideIDGenerator(v *viper.Viper) (*snowflake.Node, error) {
	workerID := v.GetInt64("snowflake.workerId")
	if workerID == 0 {
		workerID = 4
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

func ProvideCommandService(repo *persistence.Repository, idgen *snowflake.Node, publisher *messaging.KafkaEventPublisher, log logger.Logger) *command.Service {
	return command.NewService(repo, idgen, publisher, log)
}

func ProvideQueryService(repo *persistence.Repository) *query.Service {
	return query.NewService(repo)
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

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideRepository,
	ProvideIDGenerator,
	ProvideEventPublisher,
	ProvideCommandService,
	ProvideQueryService,
)
