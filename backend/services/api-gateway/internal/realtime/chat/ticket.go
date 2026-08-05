package chat

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrInvalidTicket = errors.New("invalid websocket ticket")

const (
	ticketKeyPrefix = "chat:ws-ticket:"
	maxTicketTTL    = 60 * time.Second
)

type TicketCommands interface {
	SetNX(context.Context, string, interface{}, time.Duration) *redis.BoolCmd
	GetDel(context.Context, string) *redis.StringCmd
}

type Ticket struct {
	UserID                 int64      `json:"user_id"`
	IssuedAt               time.Time  `json:"issued_at"`
	ExpiresAt              time.Time  `json:"expires_at"`
	TokenFingerprint       string     `json:"token_fingerprint,omitempty"`
	SessionID              string     `json:"session_id,omitempty"`
	TokenExpiresAt         *time.Time `json:"token_expires_at,omitempty"`
	CredentialVersion      string     `json:"credential_version,omitempty"`
	CredentialVersionClaim bool       `json:"credential_version_claim,omitempty"`
}

type TicketStore struct {
	redis TicketCommands
	ttl   time.Duration
	now   func() time.Time
}

func NewTicketStore(client TicketCommands, ttl time.Duration) *TicketStore {
	if ttl <= 0 || ttl > maxTicketTTL {
		ttl = maxTicketTTL
	}
	return &TicketStore{redis: client, ttl: ttl, now: time.Now}
}

func (s *TicketStore) Issue(ctx context.Context, userID int64) (string, time.Time, error) {
	return s.IssueAuthenticated(ctx, Ticket{UserID: userID})
}

// IssueAuthenticated persists the identity metadata needed to revalidate a
// WebSocket session after the one-time ticket has been consumed. It stores a
// token fingerprint, never the bearer token itself.
func (s *TicketStore) IssueAuthenticated(ctx context.Context, ticket Ticket) (string, time.Time, error) {
	if ticket.UserID <= 0 {
		return "", time.Time{}, ErrInvalidTicket
	}
	if s == nil || s.redis == nil {
		return "", time.Time{}, errors.New("websocket ticket store is unavailable")
	}
	now := s.now().UTC()
	expires := now.Add(s.ttl)
	ticket.TokenFingerprint = strings.TrimSpace(ticket.TokenFingerprint)
	ticket.CredentialVersion = strings.TrimSpace(ticket.CredentialVersion)
	ticket.SessionID = strings.TrimSpace(ticket.SessionID)
	ticket.IssuedAt = now
	ticket.ExpiresAt = expires
	payload, err := json.Marshal(ticket)
	if err != nil {
		return "", time.Time{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, 32)
		if _, err := rand.Read(buffer); err != nil {
			return "", time.Time{}, err
		}
		token := base64.RawURLEncoding.EncodeToString(buffer)
		ok, err := s.redis.SetNX(ctx, ticketKeyPrefix+token, payload, s.ttl).Result()
		if err != nil {
			return "", time.Time{}, err
		}
		if ok {
			return token, expires, nil
		}
	}
	return "", time.Time{}, errors.New("could not allocate websocket ticket")
}

func (s *TicketStore) Consume(ctx context.Context, token string) (Ticket, error) {
	if s == nil || s.redis == nil || !validTicketToken(token) {
		return Ticket{}, ErrInvalidTicket
	}
	value, err := s.redis.GetDel(ctx, ticketKeyPrefix+token).Result()
	if errors.Is(err, redis.Nil) {
		return Ticket{}, ErrInvalidTicket
	}
	if err != nil {
		return Ticket{}, err
	}
	var ticket Ticket
	if json.Unmarshal([]byte(value), &ticket) != nil || ticket.UserID <= 0 || ticket.ExpiresAt.Before(s.now().UTC()) {
		return Ticket{}, ErrInvalidTicket
	}
	return ticket, nil
}

func validTicketToken(token string) bool {
	if len(token) != 43 || strings.TrimSpace(token) != token {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}
