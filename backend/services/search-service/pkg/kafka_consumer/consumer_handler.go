package kafka_consumer

import (
	"context"
	"encoding/json"
	"search-service/pkg/logger"
	"time"

	"github.com/segmentio/kafka-go"
)

type Handler[T any] struct {
	l          logger.Logger
	fn         func(msg kafka.Message, t T) error
	reader     *kafka.Reader
	maxRetries int
}

func NewHandler[T any](l logger.Logger, reader *kafka.Reader, fn func(msg kafka.Message, t T) error) *Handler[T] {
	return &Handler[T]{
		l:          l,
		reader:     reader,
		fn:         fn,
		maxRetries: 3,
	}
}

func (h Handler[T]) Setup(ctx context.Context) error {
	return nil
}

func (h Handler[T]) Cleanup(ctx context.Context) error {
	return nil
}

func (h Handler[T]) ConsumeClaim(ctx context.Context) error {
	for {
		msg, err := h.reader.FetchMessage(ctx)
		if err != nil {
			// ctx 取消时正常退出
			if ctx.Err() != nil {
				return h.reader.Close()
			}
			h.l.Error("拉取消息失败", logger.Error(err))
			continue
		}

		var t T
		if err = json.Unmarshal(msg.Value, &t); err != nil {
			h.l.Error("反序列化消息失败",
				logger.Error(err),
				logger.String("topic", msg.Topic),
				logger.Int("partition", msg.Partition),
				logger.Int64("offset", msg.Offset),
			)
			// 跳过该消息并提交，避免卡住 offset
			h.commitMessage(ctx, msg)
			continue
		}

		// 重试执行业务逻辑
		var fnErr error
		for i := 0; i < h.maxRetries; i++ {
			fnErr = h.fn(msg, t)
			if fnErr == nil {
				break
			}
			h.l.Error("处理消息失败",
				logger.Error(fnErr),
				logger.String("topic", msg.Topic),
				logger.Int("partition", msg.Partition),
				logger.Int64("offset", msg.Offset),
			)
		}

		if fnErr != nil {
			h.l.Error("处理消息失败-已达重试上限",
				logger.Error(fnErr),
				logger.String("topic", msg.Topic),
				logger.Int("partition", msg.Partition),
				logger.Int64("offset", msg.Offset),
			)
			// 根据业务需要决定是否跳过；此处选择跳过继续消费
		}
		h.commitMessage(ctx, msg)
	}
}

func (h *Handler[T]) commitMessage(ctx context.Context, msg kafka.Message) {
	commitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := h.reader.CommitMessages(commitCtx, msg); err != nil {
		h.l.Error("提交 offset 失败",
			logger.Error(err),
			logger.String("topic", msg.Topic),
			logger.Int("partition", msg.Partition),
			logger.Int64("offset", msg.Offset),
		)
	}
}
