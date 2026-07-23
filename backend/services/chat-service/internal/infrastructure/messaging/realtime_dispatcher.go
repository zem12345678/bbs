package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const defaultRealtimeRetryDelay = time.Second

type MessageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

type RealtimePublisher interface {
	Publish(context.Context, string, []byte) error
}

type RedisRealtimePublisher struct {
	client redis.UniversalClient
}

func NewRedisRealtimePublisher(client redis.UniversalClient) *RedisRealtimePublisher {
	return &RedisRealtimePublisher{client: client}
}

func (p *RedisRealtimePublisher) Publish(ctx context.Context, channel string, payload []byte) error {
	return p.client.Publish(ctx, channel, payload).Err()
}

type RealtimeDispatcher struct {
	reader     MessageReader
	publisher  RealtimePublisher
	retryDelay time.Duration
	logger     *zap.Logger
}

func NewRealtimeDispatcher(reader MessageReader, publisher RealtimePublisher, retryDelay time.Duration, logger *zap.Logger) *RealtimeDispatcher {
	if retryDelay <= 0 {
		retryDelay = defaultRealtimeRetryDelay
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RealtimeDispatcher{reader: reader, publisher: publisher, retryDelay: retryDelay, logger: logger}
}

func (d *RealtimeDispatcher) Run(ctx context.Context) error {
	for {
		message, err := d.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return nil
			}
			d.logger.Warn("fetch chat realtime event failed", zap.Error(err))
			if !waitForRetry(ctx, d.retryDelay) {
				return nil
			}
			continue
		}

		channels, routeErr := realtimeChannels(message.Value)
		if routeErr != nil {
			d.logger.Warn("discard invalid chat realtime event", zap.Error(routeErr))
		} else if err := d.publishUntilSuccess(ctx, channels, message.Value); err != nil {
			return err
		}
		if err := d.commitUntilSuccess(ctx, message); err != nil {
			return err
		}
	}
}

func (d *RealtimeDispatcher) publishUntilSuccess(ctx context.Context, channels []string, payload []byte) error {
	for {
		failed := false
		for _, channel := range channels {
			if err := d.publisher.Publish(ctx, channel, payload); err != nil {
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					return nil
				}
				d.logger.Warn("publish chat realtime event failed", zap.String("channel", channel), zap.Error(err))
				failed = true
				break
			}
		}
		if !failed {
			return nil
		}
		if !waitForRetry(ctx, d.retryDelay) {
			return nil
		}
	}
}

func (d *RealtimeDispatcher) commitUntilSuccess(ctx context.Context, message kafka.Message) error {
	for {
		if err := d.reader.CommitMessages(ctx, message); err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return nil
			}
			d.logger.Warn("commit chat realtime event failed", zap.Error(err))
			if !waitForRetry(ctx, d.retryDelay) {
				return nil
			}
			continue
		}
		return nil
	}
}

func (d *RealtimeDispatcher) Close() error {
	return d.reader.Close()
}

type realtimeEnvelope struct {
	EventID   string          `json:"eventId"`
	EventType string          `json:"eventType"`
	Version   int             `json:"version"`
	Payload   json.RawMessage `json:"payload"`
}

type realtimePayload struct {
	RoomID int64 `json:"roomId"`
	UserID int64 `json:"userId"`
}

func realtimeChannels(value []byte) ([]string, error) {
	var envelope realtimeEnvelope
	if err := json.Unmarshal(value, &envelope); err != nil {
		return nil, fmt.Errorf("decode event envelope: %w", err)
	}
	if envelope.EventID == "" || envelope.EventType == "" || envelope.Version != 1 {
		return nil, errors.New("event envelope is incomplete or unsupported")
	}
	var payload realtimePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return nil, fmt.Errorf("decode event payload: %w", err)
	}

	roomChannel := func() (string, error) {
		if payload.RoomID <= 0 {
			return "", errors.New("room event is missing roomId")
		}
		return "chat:room:" + strconv.FormatInt(payload.RoomID, 10), nil
	}
	userChannel := func() (string, error) {
		if payload.UserID <= 0 {
			return "", errors.New("user event is missing userId")
		}
		return "chat:user:" + strconv.FormatInt(payload.UserID, 10), nil
	}

	switch envelope.EventType {
	case "chat.message.created.v1", "chat.announcement.updated.v1":
		channel, err := roomChannel()
		return channelList(channel), err
	case "chat.read.advanced.v1":
		channel, err := userChannel()
		return channelList(channel), err
	case "chat.membership.joined.v1":
		room, err := roomChannel()
		if err != nil {
			return nil, err
		}
		user, err := userChannel()
		if err != nil {
			return nil, err
		}
		return []string{room, user}, nil
	default:
		return nil, nil
	}
}

func channelList(channel string) []string {
	if channel == "" {
		return nil
	}
	return []string{channel}
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
