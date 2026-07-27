package chat

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	ErrRedisSubscriberNotStarted     = errors.New("chat redis subscriber is not started")
	ErrRedisSubscriberStopped        = errors.New("chat redis subscriber is stopped")
	ErrRedisSubscriptionNotRequested = errors.New("chat redis channel was not requested")
	ErrRedisSubscriptionRemoved      = errors.New("chat redis channel subscription was removed")
)

type RedisSubscriber struct {
	client   redis.UniversalClient
	hub      *Hub
	logger   *zap.Logger
	commands chan subscriptionCommand
	stateMu  sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	once     sync.Once
}

type subscriptionCommand struct {
	channel string
	remove  bool
	waiter  *subscriptionWaiter
}

type subscriptionWaiter struct {
	ctx    context.Context
	result chan error
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
		s.stateMu.Lock()
		s.ctx, s.cancel = context.WithCancel(context.Background())
		s.stateMu.Unlock()
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

// Wait returns only after Redis acknowledges the current Pub/Sub subscription
// for channel. Callers must Add the channel first; separating the two calls
// keeps reference counting in Hub while giving request handlers a real
// readiness barrier.
func (s *RedisSubscriber) Wait(ctx context.Context, channel string) error {
	if channel == "" {
		return ErrRedisSubscriptionNotRequested
	}
	if ctx == nil {
		ctx = context.Background()
	}
	running := s.runningContext()
	if running == nil {
		return ErrRedisSubscriberNotStarted
	}
	result := make(chan error, 1)
	command := subscriptionCommand{
		channel: channel,
		waiter:  &subscriptionWaiter{ctx: ctx, result: result},
	}
	select {
	case s.commands <- command:
	case <-ctx.Done():
		return ctx.Err()
	case <-running.Done():
		return ErrRedisSubscriberStopped
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-running.Done():
		return ErrRedisSubscriberStopped
	}
}

func (s *RedisSubscriber) enqueue(command subscriptionCommand) {
	running := s.runningContext()
	if command.channel == "" || running == nil {
		return
	}
	select {
	case s.commands <- command:
	case <-running.Done():
	}
}

func (s *RedisSubscriber) runningContext() context.Context {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.ctx
}

func (s *RedisSubscriber) run() {
	defer s.wg.Done()
	desired := make(map[string]struct{})
	waiters := make(map[string][]*subscriptionWaiter)
	recoveryRequired := false
	for s.ctx.Err() == nil {
		active := make(map[string]struct{})
		channels := make([]string, 0, len(desired))
		pending := make(map[string]struct{}, len(desired))
		for channel := range desired {
			channels = append(channels, channel)
			pending[channel] = struct{}{}
		}
		pubsub := s.client.Subscribe(s.ctx, channels...)
		messages := pubsub.ChannelWithSubscriptions()
		restart := false
		automaticRecovery := false
		for !restart {
			select {
			case <-s.ctx.Done():
				_ = pubsub.Close()
				return
			case command := <-s.commands:
				if command.remove {
					delete(pending, command.channel)
					delete(active, command.channel)
					if _, exists := desired[command.channel]; !exists {
						completeWaiter(command.waiter, ErrRedisSubscriptionNotRequested)
						continue
					}
					delete(desired, command.channel)
					completeWaiters(waiters, command.channel, ErrRedisSubscriptionRemoved)
					if err := pubsub.Unsubscribe(s.ctx, command.channel); err != nil && !errors.Is(err, context.Canceled) {
						s.logger.Warn("unsubscribe chat redis channel failed", zap.Error(err))
						restart = true
					}
					continue
				}
				if command.waiter != nil {
					if _, exists := desired[command.channel]; !exists {
						completeWaiter(command.waiter, ErrRedisSubscriptionNotRequested)
						continue
					}
					if _, ready := active[command.channel]; ready {
						completeWaiter(command.waiter, nil)
						continue
					}
					if command.waiter.ctx.Err() != nil {
						completeWaiter(command.waiter, command.waiter.ctx.Err())
						continue
					}
					waiters[command.channel] = append(waiters[command.channel], command.waiter)
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
					if message.Kind != "subscribe" {
						continue
					}
					if _, wanted := desired[message.Channel]; !wanted {
						continue
					}
					if _, expected := pending[message.Channel]; expected {
						delete(pending, message.Channel)
					} else if !automaticRecovery {
						// go-redis emits a fresh subscribe acknowledgement when it
						// transparently restores a Pub/Sub connection. None of the
						// old acknowledgements are safe for a new waiter in that case.
						automaticRecovery = true
						active = make(map[string]struct{})
					}
					active[message.Channel] = struct{}{}
					completeWaiters(waiters, message.Channel, nil)
					if recoveryRequired && len(pending) == 0 {
						s.hub.BroadcastResync()
						recoveryRequired = false
						automaticRecovery = false
					} else if automaticRecovery && len(desired) > 0 && len(active) >= len(desired) {
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
			if !waitForSubscriberRetry(s.ctx) {
				return
			}
		}
	}
}

func (s *RedisSubscriber) Stop() error {
	s.stateMu.RLock()
	cancel := s.cancel
	s.stateMu.RUnlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	return nil
}

func completeWaiters(waiters map[string][]*subscriptionWaiter, channel string, err error) {
	for _, waiter := range waiters[channel] {
		completeWaiter(waiter, err)
	}
	delete(waiters, channel)
}

func completeWaiter(waiter *subscriptionWaiter, err error) {
	if waiter == nil {
		return
	}
	if err == nil && waiter.ctx.Err() != nil {
		err = waiter.ctx.Err()
	}
	select {
	case waiter.result <- err:
	default:
	}
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
