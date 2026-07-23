package messaging

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestRealtimeChannels(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "message", value: `{"eventId":"e1","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":42}}`, want: []string{"chat:room:42"}},
		{name: "announcement", value: `{"eventId":"e2","eventType":"chat.announcement.updated.v1","version":1,"payload":{"roomId":42}}`, want: []string{"chat:room:42"}},
		{name: "read", value: `{"eventId":"e3","eventType":"chat.read.advanced.v1","version":1,"payload":{"roomId":42,"userId":7}}`, want: []string{"chat:user:7"}},
		{name: "membership", value: `{"eventId":"e4","eventType":"chat.membership.joined.v1","version":1,"payload":{"roomId":42,"userId":7}}`, want: []string{"chat:room:42", "chat:user:7"}},
		{name: "unknown", value: `{"eventId":"e5","eventType":"chat.future.v1","version":1,"payload":{"roomId":42}}`, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := realtimeChannels([]byte(test.value))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("channels = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestRealtimeChannelsRejectsInvalidEnvelope(t *testing.T) {
	for _, value := range []string{
		`not-json`,
		`{"eventId":"","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":1}}`,
		`{"eventId":"e1","eventType":"chat.message.created.v1","version":2,"payload":{"roomId":1}}`,
		`{"eventId":"e1","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":0}}`,
	} {
		if _, err := realtimeChannels([]byte(value)); err == nil {
			t.Fatalf("realtimeChannels(%q) unexpectedly succeeded", value)
		}
	}
}

func TestRealtimeDispatcherRetriesPublishBeforeCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &readerStub{messages: []kafka.Message{{Value: []byte(`{"eventId":"e1","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":42}}`)}}}
	publisher := &publisherStub{failures: 1, afterSuccess: cancel}
	dispatcher := NewRealtimeDispatcher(reader, publisher, time.Millisecond, nil)

	if err := dispatcher.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if publisher.calls != 2 {
		t.Fatalf("publish calls = %d, want 2", publisher.calls)
	}
	if reader.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", reader.commitCalls)
	}
}

func TestRealtimeDispatcherCommitsInvalidMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &readerStub{messages: []kafka.Message{{Value: []byte(`not-json`)}}, afterCommit: cancel}
	dispatcher := NewRealtimeDispatcher(reader, &publisherStub{}, time.Millisecond, nil)

	if err := dispatcher.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if reader.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", reader.commitCalls)
	}
}

type readerStub struct {
	mu          sync.Mutex
	messages    []kafka.Message
	fetchCalls  int
	commitCalls int
	afterCommit func()
}

func (r *readerStub) FetchMessage(ctx context.Context) (kafka.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fetchCalls < len(r.messages) {
		message := r.messages[r.fetchCalls]
		r.fetchCalls++
		return message, nil
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (r *readerStub) CommitMessages(context.Context, ...kafka.Message) error {
	r.mu.Lock()
	r.commitCalls++
	afterCommit := r.afterCommit
	r.mu.Unlock()
	if afterCommit != nil {
		afterCommit()
	}
	return nil
}

func (r *readerStub) Close() error { return nil }

type publisherStub struct {
	mu           sync.Mutex
	failures     int
	calls        int
	afterSuccess func()
}

func (p *publisherStub) Publish(context.Context, string, []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.failures > 0 {
		p.failures--
		return errors.New("redis unavailable")
	}
	if p.afterSuccess != nil {
		p.afterSuccess()
	}
	return nil
}
