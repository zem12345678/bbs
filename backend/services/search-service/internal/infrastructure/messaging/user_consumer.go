package messaging

import (
	"context"
	"encoding/json"
	"strconv"

	domain "search-service/internal/domain/search"
	"search-service/pkg/kafka_consumer"
	"search-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

const userStatusActive int32 = 1

type UserIndexer interface {
	EnsureUserIndex(ctx context.Context) error
	IndexUser(ctx context.Context, doc domain.UserDocument) error
	DeleteUser(ctx context.Context, id int64) error
}

type UserConsumer struct {
	reader  *kafka.Reader
	indexer UserIndexer
	log     logger.Logger
}

func NewUserConsumer(reader *kafka.Reader, indexer UserIndexer, log logger.Logger) *UserConsumer {
	return &UserConsumer{reader: reader, indexer: indexer, log: log}
}

func (c *UserConsumer) Start(ctx context.Context) error {
	if err := c.indexer.EnsureUserIndex(ctx); err != nil {
		return err
	}
	handler := kafka_consumer.NewHandler[eventEnvelope](c.log, c.reader, func(_ kafka.Message, env eventEnvelope) error {
		return c.handle(ctx, env)
	})
	return handler.ConsumeClaim(ctx)
}

func (c *UserConsumer) Close() error {
	return c.reader.Close()
}

func (c *UserConsumer) handle(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "user.created", "user.updated":
		var payload userPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		doc, err := payload.toDocument(env)
		if err != nil {
			return err
		}
		if doc.Status != userStatusActive {
			return c.indexer.DeleteUser(ctx, doc.ID)
		}
		return c.indexer.IndexUser(ctx, doc)
	case "user.deleted":
		id, err := strconv.ParseInt(env.AggregateID, 10, 64)
		if err != nil {
			return err
		}
		return c.indexer.DeleteUser(ctx, id)
	default:
		if c.log != nil {
			c.log.Info("skip unsupported user event", logger.String("event_type", env.EventType))
		}
		return nil
	}
}

// userPayload selects the public fields needed for search. User events may
// include internal fields such as email, which are deliberately ignored.
type userPayload struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Status    int32  `json:"status"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (p userPayload) toDocument(env eventEnvelope) (domain.UserDocument, error) {
	if p.UserID == 0 {
		id, err := strconv.ParseInt(env.AggregateID, 10, 64)
		if err != nil {
			return domain.UserDocument{}, err
		}
		p.UserID = id
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = env.OccurredAt.UnixMilli()
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = env.OccurredAt.UnixMilli()
	}
	return domain.UserDocument{
		ID:        p.UserID,
		Username:  p.Username,
		Nickname:  p.Nickname,
		Status:    p.Status,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}, nil
}
