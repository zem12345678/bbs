package query

import (
	"context"
	"errors"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type TopicView struct {
	Topic *domain.Topic
}

type Service struct {
	repo      domain.Repository
	polls     domain.PollRepository
	publisher messaging.EventPublisher
	log       logger.Logger
}

func NewService(repo domain.Repository, publisher messaging.EventPublisher, log logger.Logger) *Service {
	polls, _ := repo.(domain.PollRepository)
	return &Service{repo: repo, polls: polls, publisher: publisher, log: log}
}

func toViews(topics []*domain.Topic) []TopicView {
	out := make([]TopicView, 0, len(topics))
	for _, t := range topics {
		out = append(out, TopicView{Topic: t})
	}
	return out
}

func (s *Service) GetBySlug(ctx context.Context, slug string, trackView bool) (TopicView, error) {
	return s.GetBySlugForViewer(ctx, slug, trackView, 0)
}

func (s *Service) GetBySlugForViewer(ctx context.Context, slug string, trackView bool, viewerUserID int64) (TopicView, error) {
	t, err := s.repo.FindTopicBySlug(ctx, slug)
	if err != nil {
		return TopicView{}, err
	}
	if trackView && t.Status.CanReadPublicly() {
		if count, err := s.repo.IncrementTopicViewCount(ctx, t.ID); err == nil {
			t.ViewCount = count
			s.publishEvents(ctx, domain.NewTopicViewedEvent(t))
		}
	}
	if err := s.attachPoll(ctx, t, viewerUserID); err != nil {
		return TopicView{}, err
	}
	return TopicView{Topic: t}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64, trackView bool) (TopicView, error) {
	return s.GetByIDForViewer(ctx, id, trackView, 0)
}

func (s *Service) GetByIDForViewer(ctx context.Context, id int64, trackView bool, viewerUserID int64) (TopicView, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return TopicView{}, err
	}
	if trackView && t.Status.CanReadPublicly() {
		if count, err := s.repo.IncrementTopicViewCount(ctx, t.ID); err == nil {
			t.ViewCount = count
			s.publishEvents(ctx, domain.NewTopicViewedEvent(t))
		}
	}
	if err := s.attachPoll(ctx, t, viewerUserID); err != nil {
		return TopicView{}, err
	}
	return TopicView{Topic: t}, nil
}

func (s *Service) attachPoll(ctx context.Context, t *domain.Topic, viewerUserID int64) error {
	if s.polls == nil {
		return nil
	}
	poll, err := s.polls.FindTopicPoll(ctx, t.ID, viewerUserID)
	if errors.Is(err, domain.ErrPollNotFound) {
		t.Poll = nil
		return nil
	}
	if err != nil {
		return err
	}
	t.Poll = poll
	return nil
}

func (s *Service) List(ctx context.Context, status domain.Status, typ domain.Type, tag string, authorID int64, categoryID int64, channelID int64, sort string, limit, offset int) ([]TopicView, int64, error) {
	topics, total, err := s.repo.ListTopics(ctx, status, typ, tag, authorID, categoryID, channelID, sort, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toViews(topics), total, nil
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
