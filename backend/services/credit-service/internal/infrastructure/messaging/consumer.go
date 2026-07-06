package messaging

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type HandlerFunc func(context.Context, eventEnvelope) error

type Consumer struct {
	reader  *kafka.Reader
	handler HandlerFunc
	name    string
}

type ConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
	Name    string
}

func NewConsumer(options ConsumerOptions, handler HandlerFunc) *Consumer {
	if len(options.Brokers) == 0 {
		options.Brokers = []string{"127.0.0.1:9092"}
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
	return &Consumer{reader: reader, handler: handler, name: options.Name}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch %s event: %w", c.name, err)
		}
		var env eventEnvelope
		if err := decodeEnvelope(msg.Value, &env); err != nil {
			log.Printf("decode %s event failed: %v", c.name, err)
		} else if c.handler != nil {
			if err := c.handler(ctx, env); err != nil {
				log.Printf("handle %s event failed: %v", c.name, err)
			}
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit %s event failed: %v", c.name, err)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
