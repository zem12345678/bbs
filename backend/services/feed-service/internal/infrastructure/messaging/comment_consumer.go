package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type CommentProjector interface {
	IncrementCommentCount(ctx context.Context, id int64, delta int64) error
}

type CommentConsumer struct {
	reader    *kafka.Reader
	projector CommentProjector
}

type CommentConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewCommentConsumer(options CommentConsumerOptions, projector CommentProjector) *CommentConsumer {
	if len(options.Brokers) == 0 {
		options.Brokers = []string{"127.0.0.1:9092"}
	}
	if options.Topic == "" {
		options.Topic = "comment.events"
	}
	if options.GroupID == "" {
		options.GroupID = "bbs-feed-comment-projector"
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
	return &CommentConsumer{reader: reader, projector: projector}
}

func (c *CommentConsumer) Start(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch comment event: %w", err)
		}
		if err := c.handle(ctx, msg.Value); err != nil {
			log.Printf("handle comment event failed: %v", err)
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit comment event failed: %v", err)
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
