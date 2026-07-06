package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	domain "search-service/internal/domain/search"
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

func NewArticleConsumer(options ArticleConsumerOptions, indexer ArticleIndexer, log logger.Logger) *ArticleConsumer {
	if len(options.Brokers) == 0 {
		options.Brokers = []string{"127.0.0.1:9092"}
	}
	if options.Topic == "" {
		options.Topic = "article.events"
	}
	if options.GroupID == "" {
		options.GroupID = "bbs-search-indexer"
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        options.Brokers,
		Topic:          options.Topic,
		GroupID:        options.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
	})
	return &ArticleConsumer{reader: reader, indexer: indexer, log: log}
}

func (c *ArticleConsumer) Start(ctx context.Context) error {
	if err := c.indexer.EnsureArticleIndex(ctx); err != nil {
		return err
	}
	if err := c.indexer.EnsureTopicIndex(ctx); err != nil {
		return err
	}
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch article event: %w", err)
		}
		if err := c.handle(ctx, msg.Value); err != nil {
			if c.log != nil {
				c.log.Error("handle article event failed", logger.Error(err))
			}
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && c.log != nil {
			c.log.Error("commit article event failed", logger.Error(err))
		}
	}
}

func (c *ArticleConsumer) Close() error {
	return c.reader.Close()
}

func (c *ArticleConsumer) handle(ctx context.Context, value []byte) error {
	var env eventEnvelope
	if err := decodeEnvelope(value, &env); err != nil {
		return err
	}
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
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
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
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}
