package chat

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisSubscriber struct {
	client   redis.UniversalClient
	hub      *Hub
	logger   *zap.Logger
	commands chan subscriptionCommand
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	once     sync.Once
}

type subscriptionCommand struct {
	channel string
	remove  bool
}

func NewRedisSubscriber(client redis.UniversalClient, hub *Hub, logger *zap.Logger) *RedisSubscriber {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RedisSubscriber{
		client: client, hub: hub, logger: logger,
		commands: make(chan subscriptionCommand, 1024),
	}
}

func (s *RedisSubscriber) Start() error {
	s.once.Do(func() {
		s.ctx, s.cancel = context.WithCancel(context.Background())
		s.wg.Add(1)
		go s.run()
	})
	return nil
}

func (s *RedisSubscriber) Add(channel string) {
	s.enqueue(subscriptionCommand{channel: channel})
}

func (s *RedisSubscriber) Remove(channel string) {
	s.enqueue(subscriptionCommand{channel: channel, remove: true})
}

func (s *RedisSubscriber) enqueue(command subscriptionCommand) {
	if command.channel == "" || s.ctx == nil {
		return
	}
	select {
	case s.commands <- command:
	case <-s.ctx.Done():
	}
}

func (s *RedisSubscriber) run() {
	defer s.wg.Done()
	desired := make(map[string]struct{})
	recoveryRequired := false
	automaticRecovery := false
	for s.ctx.Err() == nil {
		channels := make([]string, 0, len(desired))
		pending := make(map[string]struct{}, len(desired))
		for channel := range desired {
			channels = append(channels, channel)
			pending[channel] = struct{}{}
		}
		pubsub := s.client.Subscribe(s.ctx, channels...)
		messages := pubsub.ChannelWithSubscriptions()
		restart := false
		for !restart {
			select {
			case <-s.ctx.Done():
				_ = pubsub.Close()
				return
			case command := <-s.commands:
				if command.remove {
					delete(pending, command.channel)
					if _, exists := desired[command.channel]; !exists {
						continue
					}
					delete(desired, command.channel)
					if err := pubsub.Unsubscribe(s.ctx, command.channel); err != nil && !errors.Is(err, context.Canceled) {
						s.logger.Warn("unsubscribe chat redis channel failed", zap.Error(err))
						restart = true
					}
					continue
				}
				if _, exists := desired[command.channel]; exists {
					continue
				}
				desired[command.channel] = struct{}{}
				pending[command.channel] = struct{}{}
				if err := pubsub.Subscribe(s.ctx, command.channel); err != nil && !errors.Is(err, context.Canceled) {
					s.logger.Warn("subscribe chat redis channel failed", zap.Error(err))
					restart = true
				}
			case value, open := <-messages:
				if !open {
					restart = true
					continue
				}
				switch message := value.(type) {
				case *redis.Subscription:
					if _, expected := pending[message.Channel]; expected {
						delete(pending, message.Channel)
					} else if message.Kind == "subscribe" && message.Count > 0 {
						// An unexpected subscribe acknowledgement is emitted by
						// go-redis when it restores a Pub/Sub connection.
						automaticRecovery = true
					}
					if recoveryRequired && message.Kind == "subscribe" && len(pending) == 0 {
						s.hub.BroadcastResync()
						recoveryRequired = false
						automaticRecovery = false
					} else if automaticRecovery && len(desired) > 0 && message.Count >= len(desired) {
						s.hub.BroadcastResync()
						automaticRecovery = false
					}
				case *redis.Message:
					s.hub.Broadcast(message.Channel, []byte(message.Payload))
				}
			}
		}
		_ = pubsub.Close()
		if s.ctx.Err() == nil {
			recoveryRequired = len(desired) > 0
			automaticRecovery = false
			if !waitForSubscriberRetry(s.ctx) {
				return
			}
		}
	}
}

func (s *RedisSubscriber) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func waitForSubscriberRetry(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
