package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type ReactionProjector interface {
	SetLikeCount(ctx context.Context, id int64, count int64) error
	SetFavoriteCount(ctx context.Context, id int64, count int64) error
}

type ReactionConsumer struct {
	reader    *kafka.Reader
	projector ReactionProjector
}

type ReactionConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewReactionConsumer(options ReactionConsumerOptions, projector ReactionProjector) *ReactionConsumer {
	if len(options.Brokers) == 0 {
		options.Brokers = []string{"127.0.0.1:9092"}
	}
	if options.Topic == "" {
		options.Topic = "reaction.events"
	}
	if options.GroupID == "" {
		options.GroupID = "bbs-feed-reaction-projector"
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
	return &ReactionConsumer{reader: reader, projector: projector}
}

func (c *ReactionConsumer) Start(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch reaction event: %w", err)
		}
		if err := c.handle(ctx, msg.Value); err != nil {
			log.Printf("handle reaction event failed: %v", err)
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit reaction event failed: %v", err)
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
	if !payload.Changed || (payload.EntityType != "article" && payload.EntityType != "topic") {
		return nil
	}
	switch env.EventType {
	case "reaction.liked.v1", "reaction.unliked.v1":
		return c.projector.SetLikeCount(ctx, payload.EntityID, payload.Count)
	case "reaction.favorited.v1", "reaction.unfavorited.v1":
		return c.projector.SetFavoriteCount(ctx, payload.EntityID, payload.Count)
	default:
		return nil
	}
}

type reactionPayload struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Count      int64  `json:"count"`
	Changed    bool   `json:"changed"`
}
