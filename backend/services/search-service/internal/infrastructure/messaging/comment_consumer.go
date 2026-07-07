package messaging

import (
	"context"
	"encoding/json"

	"search-service/pkg/kafka_consumer"
	"search-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type ArticleCommentCounter interface {
	EnsureArticleIndex(ctx context.Context) error
	EnsureTopicIndex(ctx context.Context) error
	IncrementArticleCommentCount(ctx context.Context, id int64, delta int64) error
	IncrementTopicCommentCount(ctx context.Context, id int64, delta int64) error
}

type CommentConsumer struct {
	reader  *kafka.Reader
	counter ArticleCommentCounter
	log     logger.Logger
}

type CommentConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewCommentConsumer(reader *kafka.Reader, counter ArticleCommentCounter, log logger.Logger) *CommentConsumer {
	return &CommentConsumer{reader: reader, counter: counter, log: log}
}

func (c *CommentConsumer) Start(ctx context.Context) error {
	if err := c.counter.EnsureArticleIndex(ctx); err != nil {
		return err
	}
	if err := c.counter.EnsureTopicIndex(ctx); err != nil {
		return err
	}
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
	switch payload.EntityType {
	case "article":
		return c.counter.IncrementArticleCommentCount(ctx, payload.EntityID, delta)
	case "topic":
		return c.counter.IncrementTopicCommentCount(ctx, payload.EntityID, delta)
	default:
		return nil
	}
}

type commentPayload struct {
	CommentID  int64  `json:"comment_id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	RootID     int64  `json:"root_id"`
	ParentID   int64  `json:"parent_id"`
	AuthorID   int64  `json:"author_id"`
	ActorID    int64  `json:"actor_id"`
}
