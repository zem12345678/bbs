package topic

import "time"

type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
	AggregateID() int64
}

type baseEvent struct {
	occurredAt time.Time
}

func newBaseEvent() baseEvent {
	return baseEvent{occurredAt: time.Now()}
}

func (e baseEvent) OccurredAt() time.Time { return e.occurredAt }

type TopicPublishedEvent struct {
	baseEvent
	TopicID        int64    `json:"topic_id"`
	Slug           string   `json:"slug"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	ContentExcerpt string   `json:"content_excerpt"`
	Tags           []string `json:"tags"`
	AuthorID       int64    `json:"author_id"`
	CategoryID     int64    `json:"category_id"`
	Status         int32    `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	ViewCount      int64    `json:"view_count"`
}

func NewTopicPublishedEvent(t *Topic) TopicPublishedEvent {
	return TopicPublishedEvent{
		baseEvent:      newBaseEvent(),
		TopicID:        t.ID,
		Slug:           t.Slug,
		Type:           string(t.Type),
		Title:          t.Title,
		ContentExcerpt: excerpt(t.Body, 512),
		Tags:           t.Tags,
		AuthorID:       t.AuthorID,
		CategoryID:     t.CategoryID,
		Status:         int32(t.Status),
		CreatedAt:      t.CreatedAt.UnixMilli(),
		UpdatedAt:      t.UpdatedAt.UnixMilli(),
		ViewCount:      t.ViewCount,
	}
}

func (e TopicPublishedEvent) EventName() string  { return "topic.published.v1" }
func (e TopicPublishedEvent) AggregateID() int64 { return e.TopicID }

type TopicViewedEvent struct {
	baseEvent
	TopicID   int64 `json:"topic_id"`
	ViewCount int64 `json:"view_count"`
}

func NewTopicViewedEvent(t *Topic) TopicViewedEvent {
	return TopicViewedEvent{baseEvent: newBaseEvent(), TopicID: t.ID, ViewCount: t.ViewCount}
}

func (e TopicViewedEvent) EventName() string  { return "topic.viewed.v1" }
func (e TopicViewedEvent) AggregateID() int64 { return e.TopicID }

type TopicHiddenEvent struct {
	baseEvent
	TopicID int64  `json:"topic_id"`
	Slug    string `json:"slug"`
}

func NewTopicHiddenEvent(t *Topic) TopicHiddenEvent {
	return TopicHiddenEvent{baseEvent: newBaseEvent(), TopicID: t.ID, Slug: t.Slug}
}

func (e TopicHiddenEvent) EventName() string  { return "topic.hidden.v1" }
func (e TopicHiddenEvent) AggregateID() int64 { return e.TopicID }

type TopicArchivedEvent struct {
	baseEvent
	TopicID int64  `json:"topic_id"`
	Slug    string `json:"slug"`
}

func NewTopicArchivedEvent(t *Topic) TopicArchivedEvent {
	return TopicArchivedEvent{baseEvent: newBaseEvent(), TopicID: t.ID, Slug: t.Slug}
}

func (e TopicArchivedEvent) EventName() string  { return "topic.archived.v1" }
func (e TopicArchivedEvent) AggregateID() int64 { return e.TopicID }

func excerpt(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
