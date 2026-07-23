package chat

import (
	"testing"
)

func TestHubMaintainsUserAndRoomReferenceCounts(t *testing.T) {
	subscriber := &recordingSubscriber{}
	hub := NewHub()
	hub.SetSubscriber(subscriber)
	first := newConnection(nil, hub, nil, 7)
	second := newConnection(nil, hub, nil, 7)
	if err := hub.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := hub.Register(second); err != nil {
		t.Fatal(err)
	}
	hub.ReplaceRooms(first, []RoomSubscription{{RoomID: 11, RoomNo: "AB12CD3E"}})
	hub.ReplaceRooms(second, []RoomSubscription{{RoomID: 11, RoomNo: "AB12CD3E"}})
	hub.Unregister(first)
	hub.Unregister(second)
	if subscriber.adds["chat:user:7"] != 1 || subscriber.removes["chat:user:7"] != 1 {
		t.Fatalf("user ref updates = %#v / %#v", subscriber.adds, subscriber.removes)
	}
	if subscriber.adds["chat:room:11"] != 1 || subscriber.removes["chat:room:11"] != 1 {
		t.Fatalf("room ref updates = %#v / %#v", subscriber.adds, subscriber.removes)
	}
}

func TestHubBroadcastsOnlyValidatedRoomMembers(t *testing.T) {
	hub := NewHub()
	first := newConnection(nil, hub, nil, 1)
	second := newConnection(nil, hub, nil, 2)
	if err := hub.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := hub.Register(second); err != nil {
		t.Fatal(err)
	}
	hub.ReplaceRooms(first, []RoomSubscription{{RoomID: 8, RoomNo: "AB12CD3E"}})
	hub.Broadcast("chat:room:8", []byte(`{"eventId":"e","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":8,"seq":1}}`))
	if len(first.outbound) != 1 || len(second.outbound) != 0 {
		t.Fatalf("outbound queues = %d / %d", len(first.outbound), len(second.outbound))
	}
}

func TestHubDeduplicatesDurableEventAcrossUserAndRoomChannels(t *testing.T) {
	hub := NewHub()
	connection := newConnection(nil, hub, nil, 7)
	if err := hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	hub.ReplaceRooms(connection, []RoomSubscription{{RoomID: 8, RoomNo: "AB12CD3E"}})
	event := []byte(`{"eventId":"membership-1","eventType":"chat.membership.joined.v1","version":1,"payload":{"roomId":8,"userId":7}}`)
	hub.Broadcast("chat:user:7", event)
	hub.Broadcast("chat:room:8", event)
	if len(connection.outbound) != 1 {
		t.Fatalf("outbound queue = %d, want one deduplicated event", len(connection.outbound))
	}
}

type recordingSubscriber struct {
	adds    map[string]int
	removes map[string]int
}

func (s *recordingSubscriber) Add(channel string) {
	if s.adds == nil {
		s.adds = make(map[string]int)
	}
	s.adds[channel]++
}

func (s *recordingSubscriber) Remove(channel string) {
	if s.removes == nil {
		s.removes = make(map[string]int)
	}
	s.removes[channel]++
}
