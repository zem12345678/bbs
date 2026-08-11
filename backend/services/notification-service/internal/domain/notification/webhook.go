package notification

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	WebhookMaxPerUser      = 10
	WebhookMaxNameRunes    = 100
	WebhookMaxURLBytes     = 1024
	WebhookMaxSecretBytes  = 1024
	WebhookMaxPayloadBytes = 256 * 1024
	WebhookMaxEventIDBytes = 128
)

const (
	WebhookEventMention  = "mention"
	WebhookEventUnfollow = "unfollow"
	WebhookEventFollow   = "follow"
	WebhookEventFollowed = "followed"
	WebhookEventNote     = "note"
	WebhookEventReply    = "reply"
	WebhookEventRenote   = "renote"
	WebhookEventReaction = "reaction"
	WebhookEventEdited   = "edited"
)

var (
	ErrInvalidWebhook       = errors.New("invalid webhook")
	ErrWebhookNotFound      = errors.New("webhook not found")
	ErrWebhookLimitReached  = errors.New("webhook limit reached")
	ErrWebhookUnsafeURL     = errors.New("unsafe webhook url")
	ErrWebhookPayloadTooBig = errors.New("webhook payload too large")
)

type Webhook struct {
	ID           int64
	UserID       int64
	Name         string
	URL          string
	Secret       string
	Events       []string
	Active       bool
	LatestSentAt *time.Time
	LatestStatus *int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type WebhookDelivery struct {
	ID           int64
	WebhookID    int64
	UserID       int64
	EventID      string
	EventType    string
	URL          string
	Secret       string
	Payload      []byte
	AttemptCount int32
	LockedAt     time.Time
	CreatedAt    time.Time
}

type WebhookConfig struct {
	Enabled               bool
	ServerURL             string
	AllowPrivateEndpoints bool
}

func WebhookEventTypes() []string {
	return []string{
		WebhookEventMention,
		WebhookEventUnfollow,
		WebhookEventFollow,
		WebhookEventFollowed,
		WebhookEventNote,
		WebhookEventReply,
		WebhookEventRenote,
		WebhookEventReaction,
		WebhookEventEdited,
	}
}

func NormalizeWebhookEvents(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !IsWebhookEventType(value) || seen[value] {
			return nil, ErrInvalidWebhook
		}
		seen[value] = true
	}
	if len(seen) == 0 {
		return nil, ErrInvalidWebhook
	}
	events := make([]string, 0, len(seen))
	for _, eventType := range WebhookEventTypes() {
		if seen[eventType] {
			events = append(events, eventType)
		}
	}
	return events, nil
}

func IsWebhookEventType(value string) bool {
	for _, eventType := range WebhookEventTypes() {
		if value == eventType {
			return true
		}
	}
	return false
}

func (item Webhook) Validate() error {
	name := strings.TrimSpace(item.Name)
	if item.UserID <= 0 || name == "" || utf8.RuneCountInString(name) > WebhookMaxNameRunes ||
		item.URL == "" || len(item.URL) > WebhookMaxURLBytes || len(item.Secret) > WebhookMaxSecretBytes ||
		strings.ContainsRune(name, '\x00') || strings.ContainsRune(item.URL, '\x00') || strings.ContainsRune(item.Secret, '\x00') {
		return ErrInvalidWebhook
	}
	if _, err := NormalizeWebhookEvents(item.Events); err != nil {
		return err
	}
	return nil
}

type WebhookRepository interface {
	CreateWebhook(ctx context.Context, item Webhook, maxPerUser int) (Webhook, error)
	ListWebhooks(ctx context.Context, userID int64) ([]Webhook, error)
	GetWebhook(ctx context.Context, userID, webhookID int64) (Webhook, error)
	UpdateWebhook(ctx context.Context, item Webhook) (Webhook, error)
	DeleteWebhook(ctx context.Context, userID, webhookID int64) error
	EnqueueWebhookEvent(ctx context.Context, userID int64, eventType, eventID string, payload []byte, createdAt time.Time) error
	EnqueueWebhookTest(ctx context.Context, item Webhook, eventType, eventID string, payload []byte, createdAt time.Time) error
}

type WebhookOutboxRepository interface {
	ClaimWebhookDeliveries(ctx context.Context, limit int, now time.Time, lockTimeout time.Duration) ([]WebhookDelivery, error)
	ReleaseWebhookDeliveries(ctx context.Context, deliveries []WebhookDelivery) error
	IsWebhookDeliveryActive(ctx context.Context, delivery WebhookDelivery) (bool, error)
	CompleteWebhookDelivery(ctx context.Context, delivery WebhookDelivery, statusCode int32, sentAt time.Time) error
	RetryWebhookDelivery(ctx context.Context, delivery WebhookDelivery, statusCode int32, attemptCount int32, nextAttempt time.Time, message string, exhausted bool, sentAt time.Time) error
	CleanupCompletedWebhookDeliveries(ctx context.Context, before time.Time, limit int) (int64, error)
}
