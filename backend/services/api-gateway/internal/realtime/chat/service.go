package chat

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/chatpb"
	"api-gateway/pkg/ratelimit"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const commandTimeout = 10 * time.Second

var (
	ErrOriginRejected               = errors.New("websocket origin is not allowed")
	ErrSessionInvalid               = errors.New("chat websocket session is invalid")
	ErrSessionValidationUnavailable = errors.New("chat websocket session validation is unavailable")
)

// SessionValidator rechecks the identity captured in a single-use ticket
// before a connection is established and before each client command.
type SessionValidator interface {
	ValidateChatSession(context.Context, Ticket) error
}

type ChatClient interface {
	ValidateRoomSubscriptions(context.Context, *chatpb.ValidateRoomSubscriptionsRequest, ...grpc.CallOption) (*chatpb.ValidateRoomSubscriptionsResponse, error)
	SendMessage(context.Context, *chatpb.SendMessageRequest, ...grpc.CallOption) (*chatpb.SendMessageResponse, error)
	AdvanceRead(context.Context, *chatpb.AdvanceReadRequest, ...grpc.CallOption) (*chatpb.AdvanceReadResponse, error)
}

type roomSubscriptionEvent struct {
	RoomID string `json:"room_id"`
	RoomNo string `json:"room_no"`
}

type Options struct {
	TicketTTL             time.Duration
	AllowedOrigins        []string
	Logger                *zap.Logger
	SubscribeLimiter      ratelimit.Limiter
	SendLimiter           ratelimit.Limiter
	ReadLimiter           ratelimit.Limiter
	MaxConnectionsPerUser int
	MaxConnectionsPerIP   int
	SessionValidator      SessionValidator
}

type Service struct {
	redis            redis.UniversalClient
	client           ChatClient
	tickets          *TicketStore
	hub              *Hub
	subscriber       *RedisSubscriber
	registry         *redisConnectionRegistry
	upgrader         websocket.Upgrader
	logger           *zap.Logger
	subscribeLimit   ratelimit.Limiter
	sendLimit        ratelimit.Limiter
	readLimit        ratelimit.Limiter
	sessionValidator SessionValidator
}

func NewService(redisClient redis.UniversalClient, client ChatClient, options Options) *Service {
	logger := options.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	hub := NewHubWithOptions(HubOptions{
		MaxConnectionsPerUser: options.MaxConnectionsPerUser,
		MaxConnectionsPerIP:   options.MaxConnectionsPerIP,
	})
	subscriber := NewRedisSubscriber(redisClient, hub, logger)
	hub.SetSubscriber(subscriber)
	service := &Service{
		redis: redisClient, client: client, tickets: NewTicketStore(redisClient, options.TicketTTL),
		hub: hub, subscriber: subscriber, registry: newRedisConnectionRegistry(redisClient, options.MaxConnectionsPerUser, options.MaxConnectionsPerIP),
		logger: logger, subscribeLimit: options.SubscribeLimiter, sendLimit: options.SendLimiter, readLimit: options.ReadLimiter,
		sessionValidator: options.SessionValidator,
	}
	service.upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		CheckOrigin:      originChecker(options.AllowedOrigins),
	}
	return service
}

func (s *Service) Start() error {
	if s == nil || s.redis == nil || s.client == nil {
		return errors.New("chat realtime dependencies are unavailable")
	}
	return s.subscriber.Start()
}

func (s *Service) Stop() error {
	if s == nil {
		return nil
	}
	s.hub.Close()
	if err := s.subscriber.Stop(); err != nil {
		return err
	}
	if s.redis != nil {
		return s.redis.Close()
	}
	return nil
}

func (s *Service) IssueTicket(ctx context.Context, userID int64) (string, time.Time, error) {
	if s == nil {
		return "", time.Time{}, errors.New("chat realtime service is unavailable")
	}
	return s.tickets.Issue(ctx, userID)
}

func (s *Service) IssueAuthenticatedTicket(ctx context.Context, ticket Ticket) (string, time.Time, error) {
	if s == nil {
		return "", time.Time{}, errors.New("chat realtime service is unavailable")
	}
	return s.tickets.IssueAuthenticated(ctx, ticket)
}

// SetSessionValidator is called while the HTTP handler is assembled, before
// the gateway starts accepting WebSocket connections.
func (s *Service) SetSessionValidator(validator SessionValidator) {
	if s != nil {
		s.sessionValidator = validator
	}
}

// CheckConnectionCapacity provides an early, non-reserving admission result
// for the ticket endpoint. ServeWebSocketWithClientIP acquires the real lease
// later, so concurrent requests remain safe even if this check becomes stale.
func (s *Service) CheckConnectionCapacity(ctx context.Context, userID int64, clientIP string) error {
	if s == nil {
		return errors.New("chat realtime service is unavailable")
	}
	clientIP = strings.TrimSpace(clientIP)
	if s.registry != nil {
		if err := s.registry.canAcquire(ctx, userID, clientIP); err != nil {
			return err
		}
	}
	return s.hub.canAcceptConnection(userID, clientIP)
}

func (s *Service) ServeWebSocket(w http.ResponseWriter, request *http.Request, token string) error {
	return s.ServeWebSocketWithClientIP(w, request, token, remoteIP(request))
}

// ServeWebSocketWithClientIP lets the HTTP adapter pass its trusted-proxy aware
// client address into Hub's per-IP connection limit.
func (s *Service) ServeWebSocketWithClientIP(w http.ResponseWriter, request *http.Request, token string, clientIP string) error {
	if s == nil {
		return errors.New("chat realtime service is unavailable")
	}
	if s.upgrader.CheckOrigin != nil && !s.upgrader.CheckOrigin(request) {
		return ErrOriginRejected
	}
	ticket, err := s.tickets.Consume(request.Context(), token)
	if err != nil {
		return err
	}
	if err := s.validateSession(request.Context(), ticket); err != nil {
		if errors.Is(err, ErrSessionInvalid) {
			return ErrInvalidTicket
		}
		return err
	}
	clientIP = strings.TrimSpace(clientIP)
	var registryLease *redisConnectionLease
	if s.registry != nil {
		registryLease, err = s.registry.acquire(request.Context(), ticket.UserID, clientIP)
		if err != nil {
			return err
		}
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), connectionRegistryOperationWait)
			err := registryLease.Release(releaseCtx)
			cancel()
			if err != nil && s.logger != nil {
				s.logger.Warn("release chat websocket connection lease failed", zap.Error(err))
			}
		}()
	}
	lease, err := s.hub.reserveConnection(ticket.UserID, clientIP)
	if err != nil {
		return err
	}
	defer s.hub.releaseLease(lease)
	connection, err := s.upgrader.Upgrade(w, request, nil)
	if err != nil {
		return err
	}
	chatConnection := newConnection(connection, s.hub, s, ticket.UserID)
	chatConnection.ticket = ticket
	chatConnection.clientIP = clientIP
	chatConnection.registryLease = registryLease
	if err := s.hub.registerReserved(chatConnection, lease); err != nil {
		chatConnection.Close()
		return err
	}
	chatConnection.runRegistered(request.Context())
	return nil
}

func remoteIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	address := strings.TrimSpace(request.RemoteAddr)
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}

func (s *Service) HandleCommand(parent context.Context, connection *Connection, envelope ClientEnvelope) {
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	if err := s.validateConnectionSession(ctx, connection); err != nil {
		if errors.Is(err, ErrSessionInvalid) {
			connection.Close()
			return
		}
		if s.logger != nil {
			s.logger.Warn("chat websocket session validation failed", zap.Error(err))
		}
		connection.Enqueue(errorEvent(envelope.RequestID, "unavailable", "chat session validation is unavailable"))
		return
	}
	switch envelope.Type {
	case "room.subscribe":
		s.handleSubscribe(ctx, connection, envelope)
	case "message.send":
		s.handleSend(ctx, connection, envelope)
	case "read.advance":
		s.handleRead(ctx, connection, envelope)
	default:
		connection.Enqueue(errorEvent(envelope.RequestID, "unsupported_event", "unsupported websocket event type"))
	}
}

func (s *Service) validateConnectionSession(ctx context.Context, connection *Connection) error {
	if connection == nil {
		return ErrSessionInvalid
	}
	return s.validateSession(ctx, connection.ticket)
}

func (s *Service) validateSession(ctx context.Context, ticket Ticket) error {
	if ticket.TokenFingerprint == "" {
		return nil
	}
	if ticket.TokenExpiresAt != nil && !ticket.TokenExpiresAt.After(time.Now()) {
		return ErrSessionInvalid
	}
	if s == nil || s.sessionValidator == nil {
		return nil
	}
	return s.sessionValidator.ValidateChatSession(ctx, ticket)
}

func (s *Service) handleSubscribe(ctx context.Context, connection *Connection, envelope ClientEnvelope) {
	rooms, err := parseSubscribePayload(envelope.Payload)
	if err != nil {
		connection.Enqueue(errorEvent(envelope.RequestID, "bad_request", err.Error()))
		return
	}
	if s.subscribeLimit != nil {
		limited, err := s.subscribeLimit.Limit(ctx, "rate:chat:subscribe:"+strconv.FormatInt(connection.userID, 10))
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("chat subscription rate limiter failed", zap.Error(err))
			}
			connection.Enqueue(errorEvent(envelope.RequestID, "unavailable", "chat subscription rate limiter unavailable"))
			return
		}
		if limited {
			connection.Enqueue(errorEvent(envelope.RequestID, "rate_limited", "chat subscription rate limit exceeded"))
			return
		}
	}
	response, err := s.client.ValidateRoomSubscriptions(ctx, &chatpb.ValidateRoomSubscriptionsRequest{
		UserId: connection.userID, RoomNumbers: rooms,
	})
	if err != nil {
		s.enqueueRPCError(connection, envelope.RequestID, err)
		return
	}
	subscriptions := make([]RoomSubscription, 0, len(response.GetSubscriptions()))
	for _, subscription := range response.GetSubscriptions() {
		if subscription.GetRoomId() <= 0 || subscription.GetRoomNo() == "" {
			continue
		}
		subscriptions = append(subscriptions, RoomSubscription{
			RoomID: subscription.GetRoomId(), RoomNo: subscription.GetRoomNo(),
		})
	}
	if len(subscriptions) != len(response.GetRoomNumbers()) {
		connection.Enqueue(errorEvent(envelope.RequestID, "upstream_contract", "chat subscription response is incomplete"))
		return
	}
	if err := s.hub.ReplaceRooms(ctx, connection, subscriptions); err != nil {
		if s.logger != nil && !errors.Is(err, ErrConnectionNotRegistered) && !errors.Is(err, ErrHubClosed) {
			s.logger.Warn("chat room subscription was not confirmed", zap.Error(err))
		}
		connection.Enqueue(errorEvent(envelope.RequestID, "unavailable", "chat realtime subscription is unavailable"))
		return
	}
	connection.Enqueue(encodeServerEvent("room.subscribed", envelope.RequestID, map[string]any{
		"subscriptions": roomSubscriptionEvents(subscriptions),
	}))
}

func roomSubscriptionEvents(subscriptions []RoomSubscription) []roomSubscriptionEvent {
	events := make([]roomSubscriptionEvent, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		events = append(events, roomSubscriptionEvent{
			RoomID: strconv.FormatInt(subscription.RoomID, 10),
			RoomNo: subscription.RoomNo,
		})
	}
	return events
}

func (s *Service) handleSend(ctx context.Context, connection *Connection, envelope ClientEnvelope) {
	var payload sendPayload
	if err := decodePayload(envelope.Payload, &payload, "invalid message payload"); err != nil {
		connection.Enqueue(errorEvent(envelope.RequestID, "bad_request", err.Error()))
		return
	}
	payload.RoomNo = strings.ToUpper(strings.TrimSpace(payload.RoomNo))
	payload.ClientMessageID = strings.TrimSpace(payload.ClientMessageID)
	if err := validateMessagePayload(payload); err != nil {
		connection.Enqueue(errorEvent(envelope.RequestID, "bad_request", err.Error()))
		return
	}
	if !s.hub.HasRoom(connection, payload.RoomNo) {
		connection.Enqueue(errorEvent(envelope.RequestID, "not_subscribed", "room subscription is required"))
		return
	}
	if s.sendLimit != nil {
		limited, err := s.sendLimit.Limit(ctx, "rate:chat:send:"+strconv.FormatInt(connection.userID, 10))
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("chat send rate limiter failed", zap.Error(err))
			}
			connection.Enqueue(errorEvent(envelope.RequestID, "unavailable", "chat rate limiter unavailable"))
			return
		}
		if limited {
			connection.Enqueue(errorEvent(envelope.RequestID, "rate_limited", "chat rate limit exceeded"))
			return
		}
	}
	response, err := s.client.SendMessage(ctx, &chatpb.SendMessageRequest{
		RoomNo: payload.RoomNo, UserId: connection.userID,
		ClientMessageId: payload.ClientMessageID, Body: payload.Body,
	})
	if err != nil {
		s.enqueueRPCError(connection, envelope.RequestID, err)
		return
	}
	message := response.GetMessage()
	if message == nil {
		connection.Enqueue(errorEvent(envelope.RequestID, "upstream_contract", "chat message response is empty"))
		return
	}
	connection.Enqueue(encodeServerEvent("message.ack", envelope.RequestID, map[string]any{
		"message": map[string]any{
			"id": int64String(message.GetId()), "room_id": int64String(message.GetRoomId()),
			"seq": int64String(message.GetSeq()), "sender_id": int64String(message.GetSenderId()),
			"client_message_id": message.GetClientMessageId(), "body": message.GetBody(),
			"status": message.GetStatus(), "created_at": int64String(message.GetCreatedAt()),
		},
		"latest_seq": int64String(response.GetLatestSeq()),
	}))
}

func (s *Service) handleRead(ctx context.Context, connection *Connection, envelope ClientEnvelope) {
	var payload readPayload
	if err := decodePayload(envelope.Payload, &payload, "invalid read payload"); err != nil {
		connection.Enqueue(errorEvent(envelope.RequestID, "bad_request", err.Error()))
		return
	}
	payload.RoomNo = strings.ToUpper(strings.TrimSpace(payload.RoomNo))
	if payload.RoomNo == "" || payload.ReadSeq < 0 {
		connection.Enqueue(errorEvent(envelope.RequestID, "bad_request", "room_no and a non-negative read_seq are required"))
		return
	}
	if !s.hub.HasRoom(connection, payload.RoomNo) {
		connection.Enqueue(errorEvent(envelope.RequestID, "not_subscribed", "room subscription is required"))
		return
	}
	if s.readLimit != nil {
		limited, err := s.readLimit.Limit(ctx, "rate:chat:read:"+strconv.FormatInt(connection.userID, 10))
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("chat read rate limiter failed", zap.Error(err))
			}
			connection.Enqueue(errorEvent(envelope.RequestID, "unavailable", "chat read rate limiter unavailable"))
			return
		}
		if limited {
			connection.Enqueue(errorEvent(envelope.RequestID, "rate_limited", "chat read rate limit exceeded"))
			return
		}
	}
	response, err := s.client.AdvanceRead(ctx, &chatpb.AdvanceReadRequest{
		RoomNo: payload.RoomNo, UserId: connection.userID, ReadSeq: int64(payload.ReadSeq),
	})
	if err != nil {
		s.enqueueRPCError(connection, envelope.RequestID, err)
		return
	}
	membership := response.GetMembership()
	if membership == nil {
		connection.Enqueue(errorEvent(envelope.RequestID, "upstream_contract", "chat read response is empty"))
		return
	}
	connection.Enqueue(encodeServerEvent("read.advanced", envelope.RequestID, map[string]any{
		"room_id": int64String(membership.GetRoomId()), "user_id": int64String(membership.GetUserId()),
		"last_read_seq": int64String(membership.GetLastReadSeq()),
		"latest_seq":    int64String(response.GetLatestSeq()), "unread_count": int64String(response.GetUnreadCount()),
	}))
}

func (s *Service) enqueueRPCError(connection *Connection, requestID string, err error) {
	code := strings.ToLower(status.Code(err).String())
	connection.Enqueue(errorEvent(requestID, code, status.Convert(err).Message()))
}

func originChecker(allowed []string) func(*http.Request) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		value = strings.TrimRight(strings.TrimSpace(value), "/")
		if value != "" {
			allowedSet[value] = struct{}{}
		}
	}
	return func(request *http.Request) bool {
		origin := strings.TrimRight(strings.TrimSpace(request.Header.Get("Origin")), "/")
		if origin == "" {
			return true
		}
		if _, ok := allowedSet[origin]; ok {
			return true
		}
		if len(allowedSet) != 0 {
			return false
		}
		parsed, err := url.Parse(origin)
		return err == nil && strings.EqualFold(parsed.Host, request.Host)
	}
}
