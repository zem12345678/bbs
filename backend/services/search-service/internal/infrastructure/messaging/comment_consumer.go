package messaging

import (
	"context"
	"encoding/json"
	"fmt"

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

func NewCommentConsumer(options CommentConsumerOptions, counter ArticleCommentCounter, log logger.Logger) *CommentConsumer {
	if len(options.Brokers) == 0 {
		options.Brokers = []string{"127.0.0.1:9092"}
	}
	if options.Topic == "" {
		options.Topic = "comment.events"
	}
	if options.GroupID == "" {
		options.GroupID = "bbs-search-comment-counter"
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
	return &CommentConsumer{reader: reader, counter: counter, log: log}
}

func (c *CommentConsumer) Start(ctx context.Context) error {
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
			return fmt.Errorf("fetch comment event: %w", err)
		}
		if err := c.handle(ctx, msg.Value); err != nil {
			if c.log != nil {
				c.log.Error("handle comment event failed", logger.Error(err))
			}
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && c.log != nil {
			c.log.Error("commit comment event failed", logger.Error(err))
		}
	}
}

func (c *CommentConsumer) Close() error {
	return c.reader.Close()
}

func (c *CommentConsumer) handle(ctx context.Context, value []byte) error {
	var env eventEnvelope
	if err := decodeEnvelope(value, &env); err != nil {
		return err
	}
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
