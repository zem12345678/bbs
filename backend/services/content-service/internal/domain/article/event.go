package article

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

type ArticlePublishedEvent struct {
	baseEvent
	ArticleID      int64    `json:"article_id"`
	Slug           string   `json:"slug"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	ContentExcerpt string   `json:"content_excerpt"`
	Tags           []string `json:"tags"`
	CoverURL       string   `json:"cover_url"`
	AuthorID       int64    `json:"author_id"`
	Status         int32    `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	ViewCount      int64    `json:"view_count"`
}

func NewArticlePublishedEvent(a *Article) ArticlePublishedEvent {
	return ArticlePublishedEvent{
		baseEvent:      newBaseEvent(),
		ArticleID:      a.ID,
		Slug:           a.Slug,
		Title:          a.Title,
		Summary:        a.Summary,
		ContentExcerpt: excerpt(a.Body, 512),
		Tags:           a.Tags,
		CoverURL:       a.CoverURL,
		AuthorID:       a.AuthorID,
		Status:         int32(a.Status),
		CreatedAt:      a.CreatedAt.UnixMilli(),
		UpdatedAt:      a.UpdatedAt.UnixMilli(),
		ViewCount:      a.ViewCount,
	}
}

func (e ArticlePublishedEvent) EventName() string  { return "article.published.v1" }
func (e ArticlePublishedEvent) AggregateID() int64 { return e.ArticleID }

type ArticleViewedEvent struct {
	baseEvent
	ArticleID int64 `json:"article_id"`
	ViewCount int64 `json:"view_count"`
}

func NewArticleViewedEvent(a *Article) ArticleViewedEvent {
	return ArticleViewedEvent{baseEvent: newBaseEvent(), ArticleID: a.ID, ViewCount: a.ViewCount}
}

func (e ArticleViewedEvent) EventName() string  { return "article.viewed.v1" }
func (e ArticleViewedEvent) AggregateID() int64 { return e.ArticleID }

type ArticleHiddenEvent struct {
	baseEvent
	ArticleID int64  `json:"article_id"`
	Slug      string `json:"slug"`
}

func NewArticleHiddenEvent(a *Article) ArticleHiddenEvent {
	return ArticleHiddenEvent{baseEvent: newBaseEvent(), ArticleID: a.ID, Slug: a.Slug}
}

func (e ArticleHiddenEvent) EventName() string  { return "article.hidden.v1" }
func (e ArticleHiddenEvent) AggregateID() int64 { return e.ArticleID }

type ArticleArchivedEvent struct {
	baseEvent
	ArticleID int64  `json:"article_id"`
	Slug      string `json:"slug"`
}

func NewArticleArchivedEvent(a *Article) ArticleArchivedEvent {
	return ArticleArchivedEvent{baseEvent: newBaseEvent(), ArticleID: a.ID, Slug: a.Slug}
}

func (e ArticleArchivedEvent) EventName() string  { return "article.archived.v1" }
func (e ArticleArchivedEvent) AggregateID() int64 { return e.ArticleID }

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
