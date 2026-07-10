package messaging

import (
	"context"
	"encoding/json"
	"strconv"

	domain "feed-service/internal/domain/feed"
	"feed-service/pkg/kafka_consumer"
	"feed-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type FeedProjector interface {
	UpsertArticle(ctx context.Context, item domain.Item) error
	UpsertTopic(ctx context.Context, item domain.Item) error
	RemoveArticle(ctx context.Context, id int64) error
	RemoveTopic(ctx context.Context, id int64) error
	SetViewCount(ctx context.Context, id int64, count int64) error
}

type ArticleConsumer struct {
	reader    *kafka.Reader
	projector FeedProjector
	log       logger.Logger
}

type ArticleConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewArticleConsumer(reader *kafka.Reader, projector FeedProjector, log logger.Logger) *ArticleConsumer {
	return &ArticleConsumer{reader: reader, projector: projector, log: log}
}

func (c *ArticleConsumer) Start(ctx context.Context) error {
	handler := kafka_consumer.NewHandler[eventEnvelope](c.log, c.reader, func(_ kafka.Message, env eventEnvelope) error {
		return c.handle(ctx, env)
	})
	return handler.ConsumeClaim(ctx)
}

func (c *ArticleConsumer) Close() error {
	return c.reader.Close()
}

func (c *ArticleConsumer) handle(ctx context.Context, env eventEnvelope) error {
	switch env.EventType {
	case "article.published.v1":
		var payload articlePublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return c.projector.UpsertArticle(ctx, payload.toItem(env))
	case "article.hidden.v1", "article.archived.v1":
		id, err := strconv.ParseInt(env.AggregateID, 10, 64)
		if err != nil {
			return err
		}
		return c.projector.RemoveArticle(ctx, id)
	case "article.viewed.v1":
		var payload articleViewedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		id, err := payload.id(env)
		if err != nil {
			return err
		}
		return c.projector.SetViewCount(ctx, id, payload.ViewCount)
	case "topic.published.v1":
		var payload topicPublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return c.projector.UpsertTopic(ctx, payload.toItem(env))
	case "topic.hidden.v1", "topic.archived.v1":
		id, err := strconv.ParseInt(env.AggregateID, 10, 64)
		if err != nil {
			return err
		}
		return c.projector.RemoveTopic(ctx, id)
	case "topic.viewed.v1":
		var payload topicViewedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		id, err := payload.id(env)
		if err != nil {
			return err
		}
		return c.projector.SetViewCount(ctx, id, payload.ViewCount)
	default:
		if c.log != nil {
			c.log.Info("skip unsupported article event", logger.String("event_type", env.EventType))
		}
		return nil
	}
}

type articlePublishedPayload struct {
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

func (p articlePublishedPayload) toItem(env eventEnvelope) domain.Item {
	if p.CreatedAt == 0 {
		p.CreatedAt = env.OccurredAt.UnixMilli()
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = env.OccurredAt.UnixMilli()
	}
	if p.Status == 0 {
		p.Status = 2
	}
	return domain.Item{
		EntityType:  "article",
		ID:          p.ArticleID,
		Slug:        p.Slug,
		Title:       p.Title,
		Summary:     p.Summary,
		Body:        p.ContentExcerpt,
		CoverURL:    p.CoverURL,
		Tags:        p.Tags,
		AuthorID:    p.AuthorID,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		PublishedAt: env.OccurredAt.UnixMilli(),
		ViewCount:   p.ViewCount,
	}
}

type articleViewedPayload struct {
	ArticleID int64 `json:"article_id"`
	ViewCount int64 `json:"view_count"`
}

func (p articleViewedPayload) id(env eventEnvelope) (int64, error) {
	if p.ArticleID != 0 {
		return p.ArticleID, nil
	}
	return strconv.ParseInt(env.AggregateID, 10, 64)
}

type topicPublishedPayload struct {
	TopicID        int64    `json:"topic_id"`
	Slug           string   `json:"slug"`
	Type           string   `json:"type"`
	Title          string   `json:"title"`
	ContentExcerpt string   `json:"content_excerpt"`
	Tags           []string `json:"tags"`
	AuthorID       int64    `json:"author_id"`
	Status         int32    `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	ViewCount      int64    `json:"view_count"`
	CategoryID     int64    `json:"category_id"`
}

func (p topicPublishedPayload) toItem(env eventEnvelope) domain.Item {
	if p.TopicID == 0 {
		p.TopicID, _ = strconv.ParseInt(env.AggregateID, 10, 64)
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = env.OccurredAt.UnixMilli()
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = env.OccurredAt.UnixMilli()
	}
	if p.Status == 0 {
		p.Status = 2
	}
	return domain.Item{
		EntityType:  "topic",
		ID:          p.TopicID,
		Slug:        p.Slug,
		Title:       p.Title,
		Summary:     p.Type,
		Body:        p.ContentExcerpt,
		Tags:        p.Tags,
		AuthorID:    p.AuthorID,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		PublishedAt: env.OccurredAt.UnixMilli(),
		ViewCount:   p.ViewCount,
		CategoryID:  p.CategoryID,
	}
}

type topicViewedPayload struct {
	TopicID   int64 `json:"topic_id"`
	ViewCount int64 `json:"view_count"`
}

func (p topicViewedPayload) id(env eventEnvelope) (int64, error) {
	if p.TopicID != 0 {
		return p.TopicID, nil
	}
	return strconv.ParseInt(env.AggregateID, 10, 64)
}
