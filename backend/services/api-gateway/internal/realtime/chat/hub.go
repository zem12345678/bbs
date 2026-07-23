package chat

import (
	"errors"
	"strconv"
	"strings"
	"sync"
)

var ErrHubClosed = errors.New("chat websocket hub is closed")

type RoomSubscription struct {
	RoomID int64  `json:"room_id"`
	RoomNo string `json:"room_no"`
}

type ChannelSubscriber interface {
	Add(string)
	Remove(string)
}

type Hub struct {
	mu          sync.RWMutex
	connections map[*Connection]struct{}
	users       map[int64]map[*Connection]struct{}
	rooms       map[int64]map[*Connection]struct{}
	userRefs    map[int64]int
	roomRefs    map[int64]int
	subscriber  ChannelSubscriber
	closed      bool
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[*Connection]struct{}),
		users:       make(map[int64]map[*Connection]struct{}),
		rooms:       make(map[int64]map[*Connection]struct{}),
		userRefs:    make(map[int64]int),
		roomRefs:    make(map[int64]int),
	}
}

func (h *Hub) SetSubscriber(subscriber ChannelSubscriber) {
	h.mu.Lock()
	h.subscriber = subscriber
	h.mu.Unlock()
}

func (h *Hub) Register(connection *Connection) error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return ErrHubClosed
	}
	h.connections[connection] = struct{}{}
	userID := connection.userID
	if h.users[userID] == nil {
		h.users[userID] = make(map[*Connection]struct{})
	}
	h.users[userID][connection] = struct{}{}
	h.userRefs[userID]++
	firstUser := h.userRefs[userID] == 1
	subscriber := h.subscriber
	h.mu.Unlock()
	if firstUser && subscriber != nil {
		subscriber.Add(userChannel(userID))
	}
	return nil
}

func (h *Hub) Unregister(connection *Connection) {
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

func (h *Hub) ReplaceRooms(connection *Connection, subscriptions []RoomSubscription) {
	h.mu.Lock()
	if _, exists := h.connections[connection]; !exists || h.closed {
		h.mu.Unlock()
		return
	}
	wanted := make(map[int64]string, len(subscriptions))
	for _, subscription := range subscriptions {
		if subscription.RoomID > 0 && subscription.RoomNo != "" {
			wanted[subscription.RoomID] = subscription.RoomNo
		}
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
		for _, roomID := range remove {
			subscriber.Remove(roomChannel(roomID))
		}
	}
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
