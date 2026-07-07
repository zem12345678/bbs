package kafka_consumer

import (
	"notification-service/pkg/logger"
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

// BatchHandler 批量消费接口
type BatchHandler[T any] struct {
	l             logger.Logger
	fn            func(msgs []kafka.Message, ts []T) error
	reader        *kafka.Reader
	batchSize     int
	batchDuration time.Duration
}

func NewBatchHandler[T any](l logger.Logger, reader *kafka.Reader, fn func(msgs []kafka.Message, t []T) error) *BatchHandler[T] {
	return &BatchHandler[T]{
		l:             l,
		fn:            fn,
		reader:        reader,
		batchSize:     10,
		batchDuration: time.Second,
	}
}

func (b *BatchHandler[T]) Setup(ctx context.Context) error {
	return nil
}

func (b *BatchHandler[T]) Cleanup(ctx context.Context) error {
	return nil
}

func (b *BatchHandler[T]) ConsumeClaim(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return b.reader.Close()
		default:
		}

		if err := b.consumeBatch(ctx); err != nil {
			return err
		}
	}
}

func (b *BatchHandler[T]) consumeBatch(ctx context.Context) error {
	// 每批次独立超时，超时后处理已积累的消息
	batchCtx, cancel := context.WithTimeout(ctx, b.batchDuration)
	defer cancel()

	msgs := make([]kafka.Message, 0, b.batchSize)
	ts := make([]T, 0, b.batchSize)

	for i := 0; i < b.batchSize; i++ {
		msg, err := b.reader.FetchMessage(batchCtx)
		if err != nil {
			// 外部 ctx 已取消，通知 Start 退出
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// 批次超时，退出内层循环，处理已积累的消息
			break
		}

		var t T
		if err = json.Unmarshal(msg.Value, &t); err != nil {
			b.l.Error("反序列化失败",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int("partition", msg.Partition),
				logger.Int64("offset", msg.Offset),
			)
			// 反序列化失败仍然记录消息以便提交 offset，跳过加入业务切片
			msgs = append(msgs, msg)
			continue
		}

		msgs = append(msgs, msg)
		ts = append(ts, t)
	}

	if len(msgs) == 0 {
		return nil
	}

	if err := b.fn(msgs, ts); err != nil {
		b.l.Error("调用业务批量接口失败",
			logger.Error(err))
		// 继续消费，不中断
	}

	// 批量提交 offset（kafka-go 内部取批次中最大的 offset 提交）
	commitCtx, commitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer commitCancel()
	if err := b.reader.CommitMessages(commitCtx, msgs...); err != nil {
		b.l.Error("提交 offset 失败", logger.Error(err))
	}

	return nil
}
