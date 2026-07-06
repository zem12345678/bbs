// Package canalx 提供 Canal binlog 监听消费支持。
// 用于监听 MySQL binlog 实现数据同步（如同步到 ES、缓存刷新等）。
package canalx

import (
	"context"
	"encoding/json"
	"user-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// CanalMessage Canal JSON 消息格式
type CanalMessage[T any] struct {
	Type     string `json:"type"` // INSERT, UPDATE, DELETE
	Database string `json:"database"`
	Table    string `json:"table"`
	Data     []T    `json:"data"`
	Old      []T    `json:"old,omitempty"`
}

// Handler Canal binlog 事件处理器接口
type Handler[T any] interface {
	OnInsert(ctx context.Context, data []T) error
	OnUpdate(ctx context.Context, old, new []T) error
	OnDelete(ctx context.Context, data []T) error
}

// Consume 解析 Canal JSON 消息并分发到 Handler
func Consume[T any](l logger.Logger, h Handler[T], msg *kafka.Message) error {
	var cm CanalMessage[T]
	if err := json.Unmarshal(msg.Value, &cm); err != nil {
		l.Error("Canal 消息反序列化失败", logger.Error(err))
		return err
	}
	ctx := context.Background()
	switch cm.Type {
	case "INSERT":
		return h.OnInsert(ctx, cm.Data)
	case "UPDATE":
		return h.OnUpdate(ctx, cm.Old, cm.Data)
	case "DELETE":
		return h.OnDelete(ctx, cm.Data)
	}
	return nil
}
