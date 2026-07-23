package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestTicketIsSingleUseUnderConcurrency(t *testing.T) {
	backend := newTicketBackend()
	store := NewTicketStore(backend, 30*time.Second)
	token, _, err := store.Issue(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticket, consumeErr := store.Consume(context.Background(), token)
			if consumeErr == nil && ticket.UserID != 42 {
				consumeErr = errors.New("wrong ticket user")
			}
			results <- consumeErr
		}()
	}
	wg.Wait()
	close(results)
	var successes int
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrInvalidTicket) {
			t.Fatalf("consume error = %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful consumes = %d, want 1", successes)
	}
}

func TestTicketRejectsMalformedAndExpiredValues(t *testing.T) {
	backend := newTicketBackend()
	store := NewTicketStore(backend, time.Second)
	if _, err := store.Consume(context.Background(), "not-a-ticket"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("malformed token error = %v", err)
	}
	backend.values[ticketKeyPrefix+validTestToken()] = `{"user_id":7,"expires_at":"2000-01-01T00:00:00Z"}`
	if _, err := store.Consume(context.Background(), validTestToken()); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expired ticket error = %v", err)
	}
}

func TestTicketTTLIsCapped(t *testing.T) {
	backend := newTicketBackend()
	store := NewTicketStore(backend, 5*time.Minute)
	_, expires, err := store.Issue(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if expires.Sub(store.now()) > maxTicketTTL {
		t.Fatalf("ticket ttl = %s, want <= %s", expires.Sub(store.now()), maxTicketTTL)
	}
}

type ticketBackend struct {
	mu     sync.Mutex
	values map[string]string
}

func newTicketBackend() *ticketBackend { return &ticketBackend{values: make(map[string]string)} }

func (b *ticketBackend) SetNX(_ context.Context, key string, value interface{}, _ time.Duration) *redis.BoolCmd {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.values[key]; exists {
		return redis.NewBoolResult(false, nil)
	}
	b.values[key] = stringValue(value)
	return redis.NewBoolResult(true, nil)
}

func (b *ticketBackend) GetDel(_ context.Context, key string) *redis.StringCmd {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, exists := b.values[key]
	if !exists {
		return redis.NewStringResult("", redis.Nil)
	}
	delete(b.values, key)
	return redis.NewStringResult(value, nil)
}

func stringValue(value interface{}) string {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return value.(string)
}

func validTestToken() string { return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" }
