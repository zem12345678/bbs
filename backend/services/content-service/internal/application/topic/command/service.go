package command

import (
	"context"

	domain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type IDGenerator interface {
	Generate() int64
}

type Service struct {
	repo      domain.Repository
	idgen     IDGenerator
	publisher messaging.EventPublisher
	log       logger.Logger
}

func NewService(repo domain.Repository, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger) *Service {
	return &Service{repo: repo, idgen: idgen, publisher: publisher, log: log}
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
		s.log.Warn("publish topic events failed", logger.Error(err))
	}
}

func (s *Service) Create(ctx context.Context, cmd domain.CreateCmd) (*domain.Topic, error) {
	t, err := domain.New(s.idgen.Generate(), cmd)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateTopic(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Update(ctx context.Context, id int64, cmd domain.UpdateCmd) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Update(cmd); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopic(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) Publish(ctx context.Context, id int64) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Publish(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopicStatus(ctx, id, t.Status, t.PublishedAt); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, domain.NewTopicPublishedEvent(t))
	return t, nil
}

func (s *Service) Hide(ctx context.Context, id int64) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Hide(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopicStatus(ctx, id, t.Status, nil); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, domain.NewTopicHiddenEvent(t))
	return t, nil
}

func (s *Service) Archive(ctx context.Context, id int64) (*domain.Topic, error) {
	t, err := s.repo.FindTopicByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := t.Archive(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTopicStatus(ctx, id, t.Status, nil); err != nil {
		return nil, err
	}
	s.publishEvents(ctx, domain.NewTopicArchivedEvent(t))
	return t, nil
}
