package messaging

import (
	"context"
	"encoding/json"
	"strconv"

	domain "search-service/internal/domain/search"
	"search-service/pkg/kafka_consumer"
	"search-service/pkg/logger"

	"github.com/segmentio/kafka-go"
)

type ArticleIndexer interface {
	EnsureArticleIndex(ctx context.Context) error
	EnsureTopicIndex(ctx context.Context) error
	IndexArticle(ctx context.Context, doc domain.ArticleDocument) error
	IndexTopic(ctx context.Context, doc domain.TopicDocument) error
	DeleteArticle(ctx context.Context, id int64) error
	DeleteTopic(ctx context.Context, id int64) error
	SetArticleViewCount(ctx context.Context, id int64, count int64) error
	SetTopicViewCount(ctx context.Context, id int64, count int64) error
}

type ArticleConsumer struct {
	reader  *kafka.Reader
	indexer ArticleIndexer
	log     logger.Logger
}

type ArticleConsumerOptions struct {
	Brokers []string
	Topic   string
	GroupID string
}

func NewArticleConsumer(reader *kafka.Reader, indexer ArticleIndexer, log logger.Logger) *ArticleConsumer {
	return &ArticleConsumer{reader: reader, indexer: indexer, log: log}
}

func (c *ArticleConsumer) Start(ctx context.Context) error {
	if err := c.indexer.EnsureArticleIndex(ctx); err != nil {
		return err
	}
	if err := c.indexer.EnsureTopicIndex(ctx); err != nil {
		return err
	}
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
		return c.indexer.IndexArticle(ctx, payload.toDocument(env))
	case "article.hidden.v1", "article.archived.v1":
		id, err := strconv.ParseInt(env.AggregateID, 10, 64)
		if err != nil {
			return err
		}
		return c.indexer.DeleteArticle(ctx, id)
	case "article.viewed.v1":
		var payload articleViewedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		id, err := payload.id(env)
		if err != nil {
			return err
		}
		return c.indexer.SetArticleViewCount(ctx, id, payload.ViewCount)
	case "topic.published.v1":
		var payload topicPublishedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		return c.indexer.IndexTopic(ctx, payload.toDocument(env))
	case "topic.hidden.v1", "topic.archived.v1":
		id, err := strconv.ParseInt(env.AggregateID, 10, 64)
		if err != nil {
			return err
		}
		return c.indexer.DeleteTopic(ctx, id)
	case "topic.viewed.v1":
		var payload topicViewedPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			return err
		}
		id, err := payload.id(env)
		if err != nil {
			return err
		}
		return c.indexer.SetTopicViewCount(ctx, id, payload.ViewCount)
	default:
		if c.log != nil {
			c.log.Info("skip unsupported article event", logger.String("event_type", env.EventType))
		}
		return nil
	}
}

type articlePublishedPayload struct {
	ArticleID      int64    `json:"article_id"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	ContentExcerpt string   `json:"content_excerpt"`
	Tags           []string `json:"tags"`
	AuthorID       int64    `json:"author_id"`
	Status         int32    `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	ViewCount      int64    `json:"view_count"`
}

func (p articlePublishedPayload) toDocument(env eventEnvelope) domain.ArticleDocument {
	if p.CreatedAt == 0 {
		p.CreatedAt = env.OccurredAt.UnixMilli()
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = env.OccurredAt.UnixMilli()
	}
	if p.Status == 0 {
		p.Status = 2
	}
	return domain.ArticleDocument{
		ID:             p.ArticleID,
		Title:          p.Title,
		Summary:        p.Summary,
		ContentExcerpt: p.ContentExcerpt,
		TagNames:       p.Tags,
		AuthorID:       p.AuthorID,
		Status:         p.Status,
		ViewCount:      p.ViewCount,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
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

func (p topicPublishedPayload) toDocument(env eventEnvelope) domain.TopicDocument {
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
	return domain.TopicDocument{
		ID:             p.TopicID,
		Slug:           p.Slug,
		Type:           p.Type,
		Title:          p.Title,
		ContentExcerpt: p.ContentExcerpt,
		TagNames:       p.Tags,
		AuthorID:       p.AuthorID,
		Status:         p.Status,
		ViewCount:      p.ViewCount,
		CategoryID:     p.CategoryID,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
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
