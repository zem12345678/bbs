package query

import (
	"context"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type TopicView struct {
	Topic *domain.Topic
}

type Service struct {
	repo      domain.Repository
	publisher messaging.EventPublisher
	log       logger.Logger
}

func NewService(repo domain.Repository, publisher messaging.EventPublisher, log logger.Logger) *Service {
	return &Service{repo: repo, publisher: publisher, log: log}
}

func toViews(topics []*domain.Topic) []TopicView {
	out := make([]TopicView, 0, len(topics))
	for _, t := range topics {
		out = append(out, TopicView{Topic: t})
	}
	return out
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (TopicView, error) {
	t, err := s.repo.FindTopicBySlug(ctx, slug)
	if err != nil {
		return TopicView{}, err
	}
	if t.Status.CanReadPublicly() {
		if count, err := s.repo.IncrementTopicViewCount(ctx, t.ID); err == nil {
			t.ViewCount = count
			s.publishEvents(ctx, domain.NewTopicViewedEvent(t))
		}
	}
	return TopicView{Topic: t}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (TopicView, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return TopicView{}, err
	}
	if t.Status.CanReadPublicly() {
		if count, err := s.repo.IncrementTopicViewCount(ctx, t.ID); err == nil {
			t.ViewCount = count
			s.publishEvents(ctx, domain.NewTopicViewedEvent(t))
		}
	}
	return TopicView{Topic: t}, nil
}

func (s *Service) List(ctx context.Context, status domain.Status, typ domain.Type, tag string, authorID int64, categoryID int64, sort string, limit, offset int) ([]TopicView, error) {
	topics, err := s.repo.ListTopics(ctx, status, typ, tag, authorID, categoryID, sort, limit, offset)
	if err != nil {
		return nil, err
	}
	return toViews(topics), nil
}

func (s *Service) publishEvents(ctx context.Context, events ...domain.DomainEvent) {
	if s.publisher == nil || len(events) == 0 {
		return
	}
	out := make([]messaging.DomainEvent, 0, len(events))
	for _, event := range events {
		out = append(out, event)
	}
	if err := s.publisher.PublishDomainEvents(ctx, out); err != nil && s.log != nil {
		s.log.Warn("publish topic view event failed", logger.Error(err))
	}
}
