package messaging

import (
	"context"
	"encoding/json"

	"feed-service/pkg/kafka_consumer"
	"feed-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type ReactionProjector interface {
	SetLikeCount(ctx context.Context, id int64, count int64) error
	SetFavoriteCount(ctx context.Context, id int64, count int64) error
}

type ReactionConsumer struct {
	reader    *kafka.Reader
	projector ReactionProjector
	log       logger.Logger
}

type ReactionConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewReactionConsumer(reader *kafka.Reader, projector ReactionProjector, log logger.Logger) *ReactionConsumer {
	return &ReactionConsumer{reader: reader, projector: projector, log: log}
}

func (c *ReactionConsumer) Start(ctx context.Context) error {
	handler := kafka_consumer.NewHandler[eventEnvelope](c.log, c.reader, func(_ kafka.Message, env eventEnvelope) error {
		return c.handle(ctx, env)
	})
	return handler.ConsumeClaim(ctx)
}

func (c *ReactionConsumer) Close() error {
	return c.reader.Close()
}

func (c *ReactionConsumer) handle(ctx context.Context, env eventEnvelope) error {
	var payload reactionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	if !payload.Changed || (payload.EntityType != "article" && payload.EntityType != "topic") {
		return nil
	}
	switch env.EventType {
	case "reaction.liked.v1", "reaction.unliked.v1":
		return c.projector.SetLikeCount(ctx, payload.EntityID, payload.Count)
	case "reaction.favorited.v1", "reaction.unfavorited.v1":
		return c.projector.SetFavoriteCount(ctx, payload.EntityID, payload.Count)
	default:
		if c.log != nil {
			c.log.Info("skip unsupported reaction event", logger.String("event_type", env.EventType))
		}
		return nil
	}
}

type reactionPayload struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Count      int64  `json:"count"`
	Changed    bool   `json:"changed"`
}
