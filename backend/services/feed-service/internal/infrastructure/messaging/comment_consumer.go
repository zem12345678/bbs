package messaging

import (
	"context"
	"encoding/json"

	"feed-service/pkg/kafka_consumer"
	"feed-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type CommentProjector interface {
	IncrementCommentCount(ctx context.Context, id int64, delta int64) error
}

type CommentConsumer struct {
	reader    *kafka.Reader
	projector CommentProjector
	log       logger.Logger
}

type CommentConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewCommentConsumer(reader *kafka.Reader, projector CommentProjector, log logger.Logger) *CommentConsumer {
	return &CommentConsumer{reader: reader, projector: projector, log: log}
}

func (c *CommentConsumer) Start(ctx context.Context) error {
	handler := kafka_consumer.NewHandler[eventEnvelope](c.log, c.reader, func(_ kafka.Message, env eventEnvelope) error {
		return c.handle(ctx, env)
	})
	return handler.ConsumeClaim(ctx)
}

func (c *CommentConsumer) Close() error {
	return c.reader.Close()
}

func (c *CommentConsumer) handle(ctx context.Context, env eventEnvelope) error {
	var delta int64
	switch env.EventType {
	case "comment.created":
		delta = 1
	case "comment.deleted":
		delta = -1
	default:
		if c.log != nil {
			c.log.Info("skip unsupported comment event", logger.String("event_type", env.EventType))
		}
		return nil
	}
	var payload commentPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	if payload.EntityType != "article" && payload.EntityType != "topic" {
		return nil
	}
	return c.projector.IncrementCommentCount(ctx, payload.EntityID, delta)
}

type commentPayload struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
}
