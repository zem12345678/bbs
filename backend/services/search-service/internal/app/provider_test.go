package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"search-service/pkg/logger"
)

func TestEventConsumerRunnerRestartsStoppedConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	consumer := &restartingConsumer{
		calls: make(chan int, 2),
		done:  make(chan struct{}),
	}
	runner := &EventConsumerRunner{
		consumers:         []EventConsumer{consumer},
		log:               logger.NewNopLogger(),
		restartBackoff:    time.Millisecond,
		maxRestartBackoff: time.Millisecond,
	}

	runner.Start(ctx)

	if call := waitConsumerCall(t, consumer.calls); call != 1 {
		t.Fatalf("first call = %d, want 1", call)
	}
	if call := waitConsumerCall(t, consumer.calls); call != 2 {
		t.Fatalf("second call = %d, want 2", call)
	}

	cancel()
	select {
	case <-consumer.done:
	case <-time.After(time.Second):
		t.Fatal("consumer did not stop after context cancellation")
	}
}

func waitConsumerCall(t *testing.T, calls <-chan int) int {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for consumer start")
		return 0
	}
}

type restartingConsumer struct {
	calls     chan int
	done      chan struct{}
	callCount int
}

func (c *restartingConsumer) Start(ctx context.Context) error {
	c.callCount++
	c.calls <- c.callCount
	if c.callCount == 1 {
		return errors.New("transient failure")
	}
	<-ctx.Done()
	close(c.done)
	return ctx.Err()
}

func (c *restartingConsumer) Close() error {
	return nil
}
