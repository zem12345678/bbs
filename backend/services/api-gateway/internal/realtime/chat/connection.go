package chat

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait        = 10 * time.Second
	pongWait         = 60 * time.Second
	pingPeriod       = 54 * time.Second
	queueSize        = 128
	durableDedupSize = 256
)

type commandHandler interface {
	HandleCommand(context.Context, *Connection, ClientEnvelope)
}

type Connection struct {
	ws           *websocket.Conn
	hub          *Hub
	handler      commandHandler
	userID       int64
	rooms        map[int64]string // owned by Hub.mu
	outbound     chan []byte
	done         chan struct{}
	closeOnce    sync.Once
	durableMu    sync.Mutex
	durableIDs   map[string]struct{}
	durableOrder []string
}

func newConnection(ws *websocket.Conn, hub *Hub, handler commandHandler, userID int64) *Connection {
	return &Connection{
		ws: ws, hub: hub, handler: handler, userID: userID,
		outbound: make(chan []byte, queueSize), done: make(chan struct{}),
		rooms: make(map[int64]string),
	}
}

func (c *Connection) Run(ctx context.Context) {
	if err := c.hub.Register(c); err != nil {
		c.Close()
		return
	}
	defer c.hub.Unregister(c)
	defer c.Close()
	go c.writePump()
	c.Enqueue(encodeServerEvent("session.ready", "", map[string]string{"user_id": int64String(c.userID)}))
	c.readPump(ctx)
}

func (c *Connection) Enqueue(message []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.outbound <- message:
		return true
	default:
		c.Close()
		return false
	}
}

func (c *Connection) EnqueueDurable(eventID string, message []byte) bool {
	if eventID == "" {
		return c.Enqueue(message)
	}
	c.durableMu.Lock()
	if _, exists := c.durableIDs[eventID]; exists {
		c.durableMu.Unlock()
		return true
	}
	if c.durableIDs == nil {
		c.durableIDs = make(map[string]struct{}, durableDedupSize)
	}
	c.durableIDs[eventID] = struct{}{}
	c.durableOrder = append(c.durableOrder, eventID)
	if len(c.durableOrder) > durableDedupSize {
		delete(c.durableIDs, c.durableOrder[0])
		c.durableOrder = c.durableOrder[1:]
	}
	c.durableMu.Unlock()
	return c.Enqueue(message)
}

func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeWait))
		_ = c.ws.Close()
	})
}

func (c *Connection) readPump(ctx context.Context) {
	c.ws.SetReadLimit(maxClientMessageBytes)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		messageType, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			c.Enqueue(errorEvent("", "unsupported_frame", "only text websocket messages are supported"))
			continue
		}
		envelope, err := decodeClientEnvelope(data)
		if err != nil {
			c.Enqueue(errorEvent("", "bad_request", err.Error()))
			continue
		}
		c.handler.HandleCommand(ctx, c, envelope)
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case message := <-c.outbound:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				c.Close()
				return
			}
		case <-ticker.C:
			if err := c.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				c.Close()
				return
			}
		}
	}
}

func errorEvent(requestID, code, message string) []byte {
	return encodeServerEvent("error", requestID, map[string]string{"code": code, "message": message})
}
