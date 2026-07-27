package chat

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrHubClosed               = errors.New("chat websocket hub is closed")
	ErrConnectionNotRegistered = errors.New("chat websocket connection is not registered")
	ErrInvalidConnectionLease  = errors.New("chat websocket connection lease is invalid")
	ErrUserConnectionLimit     = errors.New("chat websocket user connection limit reached")
	ErrIPConnectionLimit       = errors.New("chat websocket IP connection limit reached")
)

type RoomSubscription struct {
	RoomID int64  `json:"room_id"`
	RoomNo string `json:"room_no"`
}

// ChannelSubscriber mirrors the Hub's channel membership in the transport.
// Its methods must not synchronously re-enter this Hub's lifecycle methods;
// Hub invokes them while serializing a membership transition.
type ChannelSubscriber interface {
	Add(string)
	Remove(string)
	Wait(context.Context, string) error
}

type HubOptions struct {
	MaxConnectionsPerUser int
	MaxConnectionsPerIP   int
}

// connectionLease reserves a slot while a WebSocket handshake is in progress.
// It is only used by Service, which keeps the admission decision and HTTP
// upgrade atomic from a caller's point of view.
type connectionLease struct {
	hub      *Hub
	userID   int64
	clientIP string
	released bool // owned by Hub.mu
}

type Hub struct {
	// subscriptionMu serializes subscriber callbacks with the corresponding
	// membership transition. It is acquired before mu so a stale unsubscribe
	// cannot be enqueued after a newer subscribe for the same channel.
	subscriptionMu  sync.Mutex
	mu              sync.RWMutex
	connections     map[*Connection]struct{}
	users           map[int64]map[*Connection]struct{}
	rooms           map[int64]map[*Connection]struct{}
	userRefs        map[int64]int
	ipRefs          map[string]int
	pendingUserRefs map[int64]int
	pendingIPRefs   map[string]int
	roomRefs        map[int64]int
	subscriber      ChannelSubscriber
	maxUserRefs     int
	maxIPRefs       int
	closed          bool
}

func NewHub() *Hub {
	return NewHubWithOptions(HubOptions{})
}

func NewHubWithOptions(options HubOptions) *Hub {
	return &Hub{
		connections:     make(map[*Connection]struct{}),
		users:           make(map[int64]map[*Connection]struct{}),
		rooms:           make(map[int64]map[*Connection]struct{}),
		userRefs:        make(map[int64]int),
		ipRefs:          make(map[string]int),
		pendingUserRefs: make(map[int64]int),
		pendingIPRefs:   make(map[string]int),
		roomRefs:        make(map[int64]int),
		maxUserRefs:     options.MaxConnectionsPerUser,
		maxIPRefs:       options.MaxConnectionsPerIP,
	}
}

// SetSubscriber configures the transport before the Hub begins serving
// connections. Replacing it later does not migrate existing subscriptions.
func (h *Hub) SetSubscriber(subscriber ChannelSubscriber) {
	h.subscriptionMu.Lock()
	defer h.subscriptionMu.Unlock()
	h.mu.Lock()
	h.subscriber = subscriber
	h.mu.Unlock()
}

func (h *Hub) Register(connection *Connection) error {
	connection.membershipMu.Lock()
	defer connection.membershipMu.Unlock()
	h.subscriptionMu.Lock()
	defer h.subscriptionMu.Unlock()
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return ErrHubClosed
	}
	if _, exists := h.connections[connection]; exists {
		h.mu.Unlock()
		return nil
	}
	if err := h.canRegisterLocked(connection.userID, connection.clientIP); err != nil {
		h.mu.Unlock()
		return err
	}
	firstUser := h.registerLocked(connection)
	subscriber := h.subscriber
	h.mu.Unlock()
	if firstUser && subscriber != nil {
		subscriber.Add(userChannel(connection.userID))
	}
	return nil
}

func (h *Hub) registerLocked(connection *Connection) bool {
	h.connections[connection] = struct{}{}
	userID := connection.userID
	if h.users[userID] == nil {
		h.users[userID] = make(map[*Connection]struct{})
	}
	h.users[userID][connection] = struct{}{}
	h.userRefs[userID]++
	firstUser := h.userRefs[userID] == 1
	clientIP := connection.clientIP
	if clientIP != "" {
		h.ipRefs[clientIP]++
	}
	return firstUser
}

// reserveConnection admits a handshake before it upgrades to a WebSocket. The
// slot is either converted by registerReserved or released when the handshake
// fails, so concurrent callers cannot pass a stale capacity preflight.
func (h *Hub) reserveConnection(userID int64, clientIP string) (*connectionLease, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrHubClosed
	}
	if err := h.canRegisterLocked(userID, clientIP); err != nil {
		return nil, err
	}
	h.pendingUserRefs[userID]++
	if clientIP != "" {
		h.pendingIPRefs[clientIP]++
	}
	return &connectionLease{hub: h, userID: userID, clientIP: clientIP}, nil
}

func (h *Hub) canAcceptConnection(userID int64, clientIP string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return ErrHubClosed
	}
	return h.canRegisterLocked(userID, clientIP)
}

func (h *Hub) registerReserved(connection *Connection, lease *connectionLease) error {
	connection.membershipMu.Lock()
	defer connection.membershipMu.Unlock()
	h.subscriptionMu.Lock()
	defer h.subscriptionMu.Unlock()
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return ErrHubClosed
	}
	if lease == nil || lease.hub != h || lease.released || lease.userID != connection.userID || lease.clientIP != connection.clientIP {
		h.mu.Unlock()
		return ErrInvalidConnectionLease
	}
	if _, exists := h.connections[connection]; exists {
		h.mu.Unlock()
		return ErrInvalidConnectionLease
	}
	h.releaseLeaseLocked(lease)
	firstUser := h.registerLocked(connection)
	subscriber := h.subscriber
	h.mu.Unlock()
	if firstUser && subscriber != nil {
		subscriber.Add(userChannel(connection.userID))
	}
	return nil
}

func (h *Hub) releaseLease(lease *connectionLease) {
	if lease == nil || lease.hub != h {
		return
	}
	h.mu.Lock()
	h.releaseLeaseLocked(lease)
	h.mu.Unlock()
}

func (h *Hub) releaseLeaseLocked(lease *connectionLease) {
	if lease.released {
		return
	}
	lease.released = true
	h.pendingUserRefs[lease.userID]--
	if h.pendingUserRefs[lease.userID] <= 0 {
		delete(h.pendingUserRefs, lease.userID)
	}
	if lease.clientIP == "" {
		return
	}
	h.pendingIPRefs[lease.clientIP]--
	if h.pendingIPRefs[lease.clientIP] <= 0 {
		delete(h.pendingIPRefs, lease.clientIP)
	}
}

func (h *Hub) canRegisterLocked(userID int64, clientIP string) error {
	if h.maxUserRefs > 0 && h.userRefs[userID]+h.pendingUserRefs[userID] >= h.maxUserRefs {
		return ErrUserConnectionLimit
	}
	if clientIP != "" && h.maxIPRefs > 0 && h.ipRefs[clientIP]+h.pendingIPRefs[clientIP] >= h.maxIPRefs {
		return ErrIPConnectionLimit
	}
	return nil
}

func (h *Hub) Unregister(connection *Connection) {
	connection.membershipMu.Lock()
	defer connection.membershipMu.Unlock()
	h.subscriptionMu.Lock()
	defer h.subscriptionMu.Unlock()
	h.mu.Lock()
	if _, exists := h.connections[connection]; !exists {
		h.mu.Unlock()
		return
	}
	delete(h.connections, connection)
	userID := connection.userID
	if members := h.users[userID]; members != nil {
		delete(members, connection)
		if len(members) == 0 {
			delete(h.users, userID)
		}
	}
	h.userRefs[userID]--
	removeUser := h.userRefs[userID] <= 0
	if removeUser {
		delete(h.userRefs, userID)
	}
	clientIP := connection.clientIP
	if clientIP != "" {
		h.ipRefs[clientIP]--
		if h.ipRefs[clientIP] <= 0 {
			delete(h.ipRefs, clientIP)
		}
	}
	removeRooms := make([]int64, 0, len(connection.rooms))
	for roomID := range connection.rooms {
		if members := h.rooms[roomID]; members != nil {
			delete(members, connection)
			if len(members) == 0 {
				delete(h.rooms, roomID)
			}
		}
		h.roomRefs[roomID]--
		if h.roomRefs[roomID] <= 0 {
			delete(h.roomRefs, roomID)
			removeRooms = append(removeRooms, roomID)
		}
	}
	connection.rooms = make(map[int64]string)
	subscriber := h.subscriber
	h.mu.Unlock()

	if subscriber != nil {
		if removeUser {
			subscriber.Remove(userChannel(userID))
		}
		for _, roomID := range removeRooms {
			subscriber.Remove(roomChannel(roomID))
		}
	}
}

// ReplaceRooms installs routes for new rooms before waiting for the Redis
// subscription acknowledgement. A Redis message published after that
// acknowledgement can therefore always find the connection in the Hub.
//
// Existing routes remain active until every requested room is confirmed. If a
// confirmation fails, only the newly-added routes are rolled back so a refresh
// cannot transiently disconnect the client from rooms it was already receiving.
func (h *Hub) ReplaceRooms(ctx context.Context, connection *Connection, subscriptions []RoomSubscription) error {
	connection.membershipMu.Lock()
	defer connection.membershipMu.Unlock()
	h.subscriptionMu.Lock()
	h.mu.Lock()
	if _, exists := h.connections[connection]; !exists || h.closed {
		h.mu.Unlock()
		h.subscriptionMu.Unlock()
		if h.closed {
			return ErrHubClosed
		}
		return ErrConnectionNotRegistered
	}
	wanted := make(map[int64]string, len(subscriptions))
	for _, subscription := range subscriptions {
		roomNo := strings.ToUpper(strings.TrimSpace(subscription.RoomNo))
		if subscription.RoomID > 0 && roomNo != "" {
			wanted[subscription.RoomID] = roomNo
		}
	}
	previous := make(map[int64]string, len(connection.rooms))
	for roomID, roomNo := range connection.rooms {
		previous[roomID] = roomNo
	}
	added := make(map[int64]struct{})
	add := make([]int64, 0)
	for roomID, roomNo := range wanted {
		if _, exists := connection.rooms[roomID]; exists {
			connection.rooms[roomID] = roomNo
			continue
		}
		connection.rooms[roomID] = roomNo
		if h.rooms[roomID] == nil {
			h.rooms[roomID] = make(map[*Connection]struct{})
		}
		h.rooms[roomID][connection] = struct{}{}
		h.roomRefs[roomID]++
		added[roomID] = struct{}{}
		if h.roomRefs[roomID] == 1 {
			add = append(add, roomID)
		}
	}
	subscriber := h.subscriber
	h.mu.Unlock()

	if subscriber != nil {
		for _, roomID := range add {
			subscriber.Add(roomChannel(roomID))
		}
	}
	h.subscriptionMu.Unlock()

	if subscriber != nil {
		for roomID := range wanted {
			if err := subscriber.Wait(ctx, roomChannel(roomID)); err != nil {
				h.rollbackRoomAdditions(connection, previous, added, subscriber)
				return err
			}
		}
	}

	if err := h.commitRoomReplacement(connection, wanted, subscriber); err != nil {
		return err
	}
	return nil
}

func (h *Hub) rollbackRoomAdditions(connection *Connection, previous map[int64]string, added map[int64]struct{}, subscriber ChannelSubscriber) {
	h.subscriptionMu.Lock()
	defer h.subscriptionMu.Unlock()
	h.mu.Lock()
	if _, exists := h.connections[connection]; !exists {
		h.mu.Unlock()
		return
	}
	remove := make([]int64, 0, len(added))
	for roomID := range added {
		if _, exists := connection.rooms[roomID]; !exists {
			continue
		}
		delete(connection.rooms, roomID)
		if members := h.rooms[roomID]; members != nil {
			delete(members, connection)
			if len(members) == 0 {
				delete(h.rooms, roomID)
			}
		}
		h.roomRefs[roomID]--
		if h.roomRefs[roomID] <= 0 {
			delete(h.roomRefs, roomID)
			remove = append(remove, roomID)
		}
	}
	for roomID, roomNo := range previous {
		if _, exists := connection.rooms[roomID]; exists {
			connection.rooms[roomID] = roomNo
		}
	}
	h.mu.Unlock()

	for _, roomID := range remove {
		subscriber.Remove(roomChannel(roomID))
	}
}

func (h *Hub) commitRoomReplacement(connection *Connection, wanted map[int64]string, subscriber ChannelSubscriber) error {
	h.subscriptionMu.Lock()
	defer h.subscriptionMu.Unlock()
	h.mu.Lock()
	if _, exists := h.connections[connection]; !exists {
		h.mu.Unlock()
		return ErrConnectionNotRegistered
	}
	if h.closed {
		h.mu.Unlock()
		return ErrHubClosed
	}
	remove := make([]int64, 0)
	for roomID := range connection.rooms {
		if _, keep := wanted[roomID]; keep {
			continue
		}
		delete(connection.rooms, roomID)
		if members := h.rooms[roomID]; members != nil {
			delete(members, connection)
			if len(members) == 0 {
				delete(h.rooms, roomID)
			}
		}
		h.roomRefs[roomID]--
		if h.roomRefs[roomID] <= 0 {
			delete(h.roomRefs, roomID)
			remove = append(remove, roomID)
		}
	}
	for roomID, roomNo := range wanted {
		connection.rooms[roomID] = roomNo
	}
	h.mu.Unlock()
	if subscriber != nil {
		for _, roomID := range remove {
			subscriber.Remove(roomChannel(roomID))
		}
	}
	return nil
}

func (h *Hub) HasRoom(connection *Connection, roomNo string) bool {
	roomNo = strings.ToUpper(strings.TrimSpace(roomNo))
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, subscribedRoomNo := range connection.rooms {
		if subscribedRoomNo == roomNo {
			return true
		}
	}
	return false
}

func (h *Hub) Broadcast(channel string, payload []byte) {
	eventID, eventType, eventPayload, ok := normalizeDurableEvent(payload)
	if !ok {
		return
	}
	encoded := encodeServerEvent(eventType, "", eventPayload)
	h.mu.RLock()
	var targets map[*Connection]struct{}
	if id, parsed := parseChannelID(channel, "chat:user:"); parsed {
		targets = h.users[id]
	} else if id, parsed := parseChannelID(channel, "chat:room:"); parsed {
		targets = h.rooms[id]
	}
	connections := make([]*Connection, 0, len(targets))
	for connection := range targets {
		connections = append(connections, connection)
	}
	h.mu.RUnlock()
	for _, connection := range connections {
		connection.EnqueueDurable(eventID, encoded)
	}
}

func (h *Hub) BroadcastResync() {
	encoded := encodeServerEvent("resync.required", "", map[string]string{"reason": "realtime_transport_unavailable"})
	h.mu.RLock()
	connections := make([]*Connection, 0, len(h.connections))
	for connection := range h.connections {
		connections = append(connections, connection)
	}
	h.mu.RUnlock()
	for _, connection := range connections {
		connection.Enqueue(encoded)
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	connections := make([]*Connection, 0, len(h.connections))
	for connection := range h.connections {
		connections = append(connections, connection)
	}
	h.mu.Unlock()
	for _, connection := range connections {
		connection.Close()
	}
}

func userChannel(userID int64) string { return "chat:user:" + int64String(userID) }
func roomChannel(roomID int64) string { return "chat:room:" + int64String(roomID) }

func parseChannelID(channel, prefix string) (int64, bool) {
	if !strings.HasPrefix(channel, prefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(channel, prefix), 10, 64)
	return id, err == nil && id > 0
}
