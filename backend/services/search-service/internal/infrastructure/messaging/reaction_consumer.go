package messaging

import (
	"context"
	"encoding/json"
	"fmt"

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

func NewReactionConsumer(options ReactionConsumerOptions, counter ArticleReactionCounter, log logger.Logger) *ReactionConsumer {
	if len(options.Brokers) == 0 {
		options.Brokers = []string{"127.0.0.1:9092"}
	}
	if options.Topic == "" {
		options.Topic = "reaction.events"
	}
	if options.GroupID == "" {
		options.GroupID = "bbs-search-reaction-counter"
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        options.Brokers,
		Topic:          options.Topic,
		GroupID:        options.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
	})
	return &ReactionConsumer{reader: reader, counter: counter, log: log}
}

func (c *ReactionConsumer) Start(ctx context.Context) error {
	if err := c.counter.EnsureArticleIndex(ctx); err != nil {
		return err
	}
	if err := c.counter.EnsureTopicIndex(ctx); err != nil {
		return err
	}
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch reaction event: %w", err)
		}
		if err := c.handle(ctx, msg.Value); err != nil {
			if c.log != nil {
				c.log.Error("handle reaction event failed", logger.Error(err))
			}
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && c.log != nil {
			c.log.Error("commit reaction event failed", logger.Error(err))
		}
	}
}

func (c *ReactionConsumer) Close() error {
	return c.reader.Close()
}

func (c *ReactionConsumer) handle(ctx context.Context, value []byte) error {
	var env eventEnvelope
	if err := decodeEnvelope(value, &env); err != nil {
		return err
	}
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
