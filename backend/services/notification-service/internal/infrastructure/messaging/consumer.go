package messaging

import (
	"context"
	"fmt"

	"notification-service/pkg/kafka_consumer"
	"notification-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type HandlerFunc func(context.Context, eventEnvelope) error

type Consumer struct {
	reader  *kafka.Reader
	handler HandlerFunc
	name    string
	log     logger.Logger
}

type ConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
	Name    string
}

func NewConsumer(reader *kafka.Reader, name string, handler HandlerFunc, log logger.Logger) *Consumer {
	return &Consumer{reader: reader, handler: handler, name: name, log: log}
}

func (c *Consumer) Start(ctx context.Context) error {
	handler := kafka_consumer.NewHandler[eventEnvelope](c.log, c.reader, func(_ kafka.Message, env eventEnvelope) error {
		if c.handler == nil {
			return nil
		}
		if err := c.handler(ctx, env); err != nil {
			return fmt.Errorf("handle %s event: %w", c.name, err)
		}
		return nil
	})
	return handler.ConsumeClaim(ctx)
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
