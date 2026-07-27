package chat

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
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
	if err := hub.ReplaceRooms(context.Background(), first, []RoomSubscription{{RoomID: 11, RoomNo: "AB12CD3E"}}); err != nil {
		t.Fatal(err)
	}
	if err := hub.ReplaceRooms(context.Background(), second, []RoomSubscription{{RoomID: 11, RoomNo: "AB12CD3E"}}); err != nil {
		t.Fatal(err)
	}
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
	if err := hub.ReplaceRooms(context.Background(), first, []RoomSubscription{{RoomID: 8, RoomNo: "AB12CD3E"}}); err != nil {
		t.Fatal(err)
	}
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
	if err := hub.ReplaceRooms(context.Background(), connection, []RoomSubscription{{RoomID: 8, RoomNo: "AB12CD3E"}}); err != nil {
		t.Fatal(err)
	}
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
	waitErr error
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

func (s *recordingSubscriber) Wait(context.Context, string) error { return s.waitErr }

func TestHubRetainsExistingRoomsWhenNewSubscriptionCannotBeConfirmed(t *testing.T) {
	subscriber := &recordingSubscriber{}
	hub := NewHub()
	hub.SetSubscriber(subscriber)
	connection := newConnection(nil, hub, nil, 7)
	if err := hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	if err := hub.ReplaceRooms(context.Background(), connection, []RoomSubscription{{RoomID: 8, RoomNo: "AB12CD3E"}}); err != nil {
		t.Fatal(err)
	}
	subscriber.waitErr = errors.New("redis is unavailable")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := hub.ReplaceRooms(ctx, connection, []RoomSubscription{{RoomID: 9, RoomNo: "EF45GH6J"}})
	if err == nil {
		t.Fatal("expected subscription confirmation failure")
	}
	if !hub.HasRoom(connection, "AB12CD3E") {
		t.Fatal("existing room was removed after replacement failure")
	}
	if hub.HasRoom(connection, "EF45GH6J") {
		t.Fatal("unconfirmed room remained routable")
	}
	if subscriber.removes[roomChannel(9)] != 1 {
		t.Fatalf("new room removals = %d, want 1", subscriber.removes[roomChannel(9)])
	}
	if subscriber.removes[roomChannel(8)] != 0 {
		t.Fatalf("existing room removals = %d, want 0", subscriber.removes[roomChannel(8)])
	}
}

func TestHubQueuesReconnectSubscriptionCallbacksInOrder(t *testing.T) {
	const (
		userID int64 = 7
		roomID int64 = 11
	)
	room := []RoomSubscription{{RoomID: roomID, RoomNo: "AB12CD3E"}}
	subscriber := newOrderedBlockingSubscriber(userChannel(userID))
	hub := NewHub()
	hub.SetSubscriber(subscriber)
	first := newConnection(nil, hub, nil, userID)
	if err := hub.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := hub.ReplaceRooms(context.Background(), first, room); err != nil {
		t.Fatal(err)
	}
	subscriber.resetEvents()

	// Keep the Hub state lock occupied until Unregister has acquired the
	// subscription lock. This makes the callback ordering deterministic.
	hub.mu.Lock()
	hubMuLocked := true
	t.Cleanup(func() {
		if hubMuLocked {
			hub.mu.Unlock()
		}
		subscriber.release()
	})
	unregistered := make(chan struct{})
	go func() {
		hub.Unregister(first)
		close(unregistered)
	}()
	awaitSubscriptionLock(t, hub)

	second := newConnection(nil, hub, nil, userID)
	reconnected := make(chan error, 1)
	go func() {
		if err := hub.Register(second); err != nil {
			reconnected <- err
			return
		}
		reconnected <- hub.ReplaceRooms(context.Background(), second, room)
	}()
	hub.mu.Unlock()
	hubMuLocked = false

	awaitSignal(t, subscriber.removeStarted, "initial unsubscribe callback")
	subscriber.release()
	awaitSignal(t, unregistered, "initial unregister completion")
	if err := awaitError(t, reconnected, "replacement registration"); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"remove:" + userChannel(userID),
		"remove:" + roomChannel(roomID),
		"add:" + userChannel(userID),
		"add:" + roomChannel(roomID),
	}
	if got := subscriber.eventsSnapshot(); !sameStrings(got, want) {
		t.Fatalf("subscription callback order = %#v, want %#v", got, want)
	}
}

func TestHubSerializesRoomReplacementsForConnection(t *testing.T) {
	const (
		firstRoomID  int64 = 1
		secondRoomID int64 = 2
		thirdRoomID  int64 = 3
	)
	subscriber := newGatedRoomSubscriber(roomChannel(secondRoomID))
	hub := NewHub()
	hub.SetSubscriber(subscriber)
	connection := newConnection(nil, hub, nil, 7)
	if err := hub.Register(connection); err != nil {
		t.Fatal(err)
	}
	if err := hub.ReplaceRooms(context.Background(), connection, []RoomSubscription{{RoomID: firstRoomID, RoomNo: "FIRST001"}}); err != nil {
		t.Fatal(err)
	}

	firstReplacement := make(chan error, 1)
	go func() {
		firstReplacement <- hub.ReplaceRooms(context.Background(), connection, []RoomSubscription{{RoomID: secondRoomID, RoomNo: "SECOND02"}})
	}()
	awaitSignal(t, subscriber.waitStarted, "first room subscription acknowledgement")
	t.Cleanup(subscriber.release)

	if connection.membershipMu.TryLock() {
		connection.membershipMu.Unlock()
		t.Fatal("room replacement released the connection membership lock while waiting")
	}

	secondReplacement := make(chan error, 1)
	go func() {
		secondReplacement <- hub.ReplaceRooms(context.Background(), connection, []RoomSubscription{{RoomID: thirdRoomID, RoomNo: "THIRD003"}})
	}()
	subscriber.release()
	if err := awaitError(t, firstReplacement, "first room replacement"); err != nil {
		t.Fatal(err)
	}
	if err := awaitError(t, secondReplacement, "second room replacement"); err != nil {
		t.Fatal(err)
	}

	if hub.HasRoom(connection, "FIRST001") || hub.HasRoom(connection, "SECOND02") || !hub.HasRoom(connection, "THIRD003") {
		t.Fatalf("room membership after serialized replacements is incorrect")
	}
	hub.Broadcast(roomChannel(secondRoomID), []byte(`{"eventId":"second-room","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":2,"seq":1}}`))
	if got := len(connection.outbound); got != 0 {
		t.Fatalf("old room broadcast targets = %d, want 0", got)
	}
	hub.Broadcast(roomChannel(thirdRoomID), []byte(`{"eventId":"third-room","eventType":"chat.message.created.v1","version":1,"payload":{"roomId":3,"seq":1}}`))
	if got := len(connection.outbound); got != 1 {
		t.Fatalf("new room broadcast targets = %d, want 1", got)
	}
}

type orderedBlockingSubscriber struct {
	blockChannel  string
	removeStarted chan struct{}
	releaseRemove chan struct{}
	releaseOnce   sync.Once
	mu            sync.Mutex
	blocked       bool
	events        []string
}

func newOrderedBlockingSubscriber(blockChannel string) *orderedBlockingSubscriber {
	return &orderedBlockingSubscriber{
		blockChannel:  blockChannel,
		removeStarted: make(chan struct{}),
		releaseRemove: make(chan struct{}),
	}
}

func (s *orderedBlockingSubscriber) Add(channel string) {
	s.record("add:" + channel)
}

func (s *orderedBlockingSubscriber) Remove(channel string) {
	s.mu.Lock()
	block := channel == s.blockChannel && !s.blocked
	if block {
		s.blocked = true
	}
	s.mu.Unlock()
	if block {
		close(s.removeStarted)
		<-s.releaseRemove
	}
	s.record("remove:" + channel)
}

func (s *orderedBlockingSubscriber) Wait(context.Context, string) error { return nil }

func (s *orderedBlockingSubscriber) release() {
	s.releaseOnce.Do(func() { close(s.releaseRemove) })
}

func (s *orderedBlockingSubscriber) record(event string) {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
}

func (s *orderedBlockingSubscriber) resetEvents() {
	s.mu.Lock()
	s.events = nil
	s.mu.Unlock()
}

func (s *orderedBlockingSubscriber) eventsSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.events...)
}

type gatedRoomSubscriber struct {
	blockChannel string
	waitStarted  chan struct{}
	releaseWait  chan struct{}
	waitOnce     sync.Once
	releaseOnce  sync.Once
}

func newGatedRoomSubscriber(blockChannel string) *gatedRoomSubscriber {
	return &gatedRoomSubscriber{
		blockChannel: blockChannel,
		waitStarted:  make(chan struct{}),
		releaseWait:  make(chan struct{}),
	}
}

func (*gatedRoomSubscriber) Add(string)    {}
func (*gatedRoomSubscriber) Remove(string) {}

func (s *gatedRoomSubscriber) Wait(ctx context.Context, channel string) error {
	if channel != s.blockChannel {
		return nil
	}
	s.waitOnce.Do(func() { close(s.waitStarted) })
	select {
	case <-s.releaseWait:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *gatedRoomSubscriber) release() {
	s.releaseOnce.Do(func() { close(s.releaseWait) })
}

func awaitSubscriptionLock(t *testing.T, hub *Hub) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if !hub.subscriptionMu.TryLock() {
			return
		}
		hub.subscriptionMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for unregister to acquire the subscription lock")
		}
		runtime.Gosched()
	}
}

func awaitSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func awaitError(t *testing.T, result <-chan error, description string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestHubEnforcesConnectionLimitsAndFreesCapacityOnUnregister(t *testing.T) {
	hub := NewHubWithOptions(HubOptions{MaxConnectionsPerUser: 1, MaxConnectionsPerIP: 1})
	first := newConnection(nil, hub, nil, 7)
	first.clientIP = "198.51.100.20"
	if err := hub.Register(first); err != nil {
		t.Fatal(err)
	}

	sameUser := newConnection(nil, hub, nil, 7)
	sameUser.clientIP = "198.51.100.21"
	if err := hub.Register(sameUser); !errors.Is(err, ErrUserConnectionLimit) {
		t.Fatalf("same-user registration error = %v, want %v", err, ErrUserConnectionLimit)
	}
	differentUserSameIP := newConnection(nil, hub, nil, 8)
	differentUserSameIP.clientIP = first.clientIP
	if err := hub.Register(differentUserSameIP); !errors.Is(err, ErrIPConnectionLimit) {
		t.Fatalf("same-IP registration error = %v, want %v", err, ErrIPConnectionLimit)
	}

	hub.Unregister(first)
	if err := hub.Register(sameUser); err != nil {
		t.Fatalf("same user should be admitted after unregister: %v", err)
	}
	hub.Unregister(sameUser)
	if err := hub.Register(differentUserSameIP); err != nil {
		t.Fatalf("same IP should be admitted after unregister: %v", err)
	}
}

func TestHubConnectionLeaseReservesCapacityUntilReleased(t *testing.T) {
	hub := NewHubWithOptions(HubOptions{MaxConnectionsPerUser: 1, MaxConnectionsPerIP: 1})
	lease, err := hub.reserveConnection(7, "198.51.100.20")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.reserveConnection(7, "198.51.100.21"); !errors.Is(err, ErrUserConnectionLimit) {
		t.Fatalf("reserved same-user connection error = %v, want %v", err, ErrUserConnectionLimit)
	}
	if _, err := hub.reserveConnection(8, "198.51.100.20"); !errors.Is(err, ErrIPConnectionLimit) {
		t.Fatalf("reserved same-IP connection error = %v, want %v", err, ErrIPConnectionLimit)
	}

	hub.releaseLease(lease)
	connection := newConnection(nil, hub, nil, 8)
	connection.clientIP = "198.51.100.20"
	if err := hub.Register(connection); err != nil {
		t.Fatalf("released lease should free capacity: %v", err)
	}
}
