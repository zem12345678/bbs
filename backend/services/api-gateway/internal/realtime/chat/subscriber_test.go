package chat

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisSubscriberDoesNotResyncForDynamicSubscriptions(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	hub := NewHub()
	subscriber := NewRedisSubscriber(client, hub, nil)
	hub.SetSubscriber(subscriber)
	if err := subscriber.Start(); err != nil {
		t.Fatal(err)
	}
	connection := newConnection(nil, hub, nil, 7)
	t.Cleanup(func() {
		hub.Unregister(connection)
		_ = subscriber.Stop()
		_ = client.Close()
		server.Close()
	})
	if err := hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	hub.ReplaceRooms(connection, []RoomSubscription{{RoomID: 11, RoomNo: "AB12CD3E"}})

	waitForRedisSubscriptions(t, server, userChannel(7), roomChannel(11))
	assertNoOutboundEvent(t, connection.outbound)
}

func TestRedisSubscriberResyncsAfterSubscriptionRecovery(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	hub := NewHub()
	subscriber := NewRedisSubscriber(client, hub, nil)
	hub.SetSubscriber(subscriber)
	if err := subscriber.Start(); err != nil {
		t.Fatal(err)
	}
	connection := newConnection(nil, hub, nil, 7)
	t.Cleanup(func() {
		hub.Unregister(connection)
		_ = subscriber.Stop()
		_ = client.Close()
		server.Close()
	})
	if err := hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	waitForRedisSubscriptions(t, server, userChannel(7))
	assertNoOutboundEvent(t, connection.outbound)

	server.Close()
	if err := server.Restart(); err != nil {
		t.Fatalf("restart miniredis: %v", err)
	}
	waitForRedisSubscriptions(t, server, userChannel(7))
	waitFor(t, func() bool { return len(connection.outbound) == 1 })

	event := <-connection.outbound
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(event, &envelope); err != nil {
		t.Fatalf("decode resync event: %v", err)
	}
	if envelope.Type != "resync.required" {
		t.Fatalf("event type = %q, want resync.required", envelope.Type)
	}
	assertNoOutboundEvent(t, connection.outbound)
}

func waitForRedisSubscriptions(t *testing.T, server *miniredis.Miniredis, channels ...string) {
	t.Helper()
	waitFor(t, func() bool {
		counts := server.PubSubNumSub(channels...)
		for _, channel := range channels {
			if counts[channel] == 0 {
				return false
			}
		}
		return true
	})
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func assertNoOutboundEvent(t *testing.T, outbound <-chan []byte) {
	t.Helper()
	select {
	case event := <-outbound:
		t.Fatalf("unexpected outbound event: %s", event)
	case <-time.After(100 * time.Millisecond):
	}
}
