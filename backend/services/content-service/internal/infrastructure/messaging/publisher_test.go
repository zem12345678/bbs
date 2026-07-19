package messaging

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaEventPublisherBoundsEventPublishDeadline(t *testing.T) {
	transport := &deadlineCapturingTransport{}
	publisher := NewKafkaEventPublisher(&kafka.Writer{
		Addr:      kafka.TCP("127.0.0.1:9092"),
		Topic:     "content.events",
		Transport: transport,
	}, nil)

	err := publisher.PublishDomainEvents(context.Background(), []DomainEvent{testEvent{}})
	if !errors.Is(err, errKafkaUnavailable) {
		t.Fatalf("publish events error = %v, want kafka unavailable", err)
	}
	if !transport.hasDeadline {
		t.Fatal("kafka publish context has no deadline")
	}
	if remaining := time.Until(transport.deadline); remaining < eventPublishTimeout-250*time.Millisecond || remaining > eventPublishTimeout+100*time.Millisecond {
		t.Fatalf("kafka publish deadline remaining = %s, want about %s", remaining, eventPublishTimeout)
	}
}

var errKafkaUnavailable = errors.New("kafka unavailable")

type deadlineCapturingTransport struct {
	deadline    time.Time
	hasDeadline bool
}

func (t *deadlineCapturingTransport) RoundTrip(ctx context.Context, _ net.Addr, _ kafka.Request) (kafka.Response, error) {
	t.deadline, t.hasDeadline = ctx.Deadline()
	return nil, errKafkaUnavailable
}

type testEvent struct{}

func (testEvent) EventName() string     { return "content.test" }
func (testEvent) OccurredAt() time.Time { return time.Now() }
func (testEvent) AggregateID() int64    { return 1 }
