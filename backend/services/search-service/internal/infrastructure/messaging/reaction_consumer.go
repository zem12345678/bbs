package messaging

import (
	"context"
	"encoding/json"

	"search-service/pkg/kafka_consumer"
	"search-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type ArticleReactionCounter interface {
	EnsureArticleIndex(ctx context.Context) error
	EnsureTopicIndex(ctx context.Context) error
	SetArticleLikeCount(ctx context.Context, id int64, count int64) error
	SetTopicLikeCount(ctx context.Context, id int64, count int64) error
	SetArticleFavoriteCount(ctx context.Context, id int64, count int64) error
	SetTopicFavoriteCount(ctx context.Context, id int64, count int64) error
}

type ReactionConsumer struct {
	reader  *kafka.Reader
	counter ArticleReactionCounter
	log     logger.Logger
}

type ReactionConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewReactionConsumer(reader *kafka.Reader, counter ArticleReactionCounter, log logger.Logger) *ReactionConsumer {
	return &ReactionConsumer{reader: reader, counter: counter, log: log}
}

func (c *ReactionConsumer) Start(ctx context.Context) error {
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

func (c *ReactionConsumer) Close() error {
	return c.reader.Close()
}

func (c *ReactionConsumer) handle(ctx context.Context, env eventEnvelope) error {
	var payload reactionPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}
	if !payload.Changed {
		return nil
	}
	switch env.EventType {
	case "reaction.liked.v1", "reaction.unliked.v1":
		if payload.EntityType == "article" {
			return c.counter.SetArticleLikeCount(ctx, payload.EntityID, payload.Count)
		}
		if payload.EntityType == "topic" {
			return c.counter.SetTopicLikeCount(ctx, payload.EntityID, payload.Count)
		}
		return nil
	case "reaction.favorited.v1", "reaction.unfavorited.v1":
		if payload.EntityType == "article" {
			return c.counter.SetArticleFavoriteCount(ctx, payload.EntityID, payload.Count)
		}
		if payload.EntityType == "topic" {
			return c.counter.SetTopicFavoriteCount(ctx, payload.EntityID, payload.Count)
		}
		return nil
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
	UserID     int64  `json:"user_id"`
	Count      int64  `json:"count"`
	Changed    bool   `json:"changed"`
}
