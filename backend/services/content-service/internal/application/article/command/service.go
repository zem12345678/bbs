package command

import (
	"context"

	domain "content-service/internal/domain/article"
	"content-service/internal/infrastructure/cache"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type IDGenerator interface {
	Generate() int64
}

type Service struct {
	repo      domain.Repository
	cache     *cache.ArticleCache
	idgen     IDGenerator
	publisher messaging.EventPublisher
	log       logger.Logger
}

func NewService(repo domain.Repository, c *cache.ArticleCache, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger) *Service {
	return &Service{repo: repo, cache: c, idgen: idgen, publisher: publisher, log: log}
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
		s.log.Warn("publish content events failed", logger.Error(err))
	}
}

func (s *Service) Create(ctx context.Context, cmd domain.CreateCmd) (*domain.Article, error) {
	a, err := domain.New(s.idgen.Generate(), cmd)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) Update(ctx context.Context, id int64, cmd domain.UpdateCmd) (*domain.Article, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.Update(cmd); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	s.cache.Del(ctx, a.Slug)
	return a, nil
}

func (s *Service) Publish(ctx context.Context, id int64) (*domain.Article, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.Publish(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, id, a.Status, a.PublishedAt); err != nil {
		return nil, err
	}
	s.cache.Del(ctx, a.Slug)
	s.publishEvents(ctx, domain.NewArticlePublishedEvent(a))
	return a, nil
}

func (s *Service) Hide(ctx context.Context, id int64) (*domain.Article, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.Hide(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, id, a.Status, nil); err != nil {
		return nil, err
	}
	s.cache.Del(ctx, a.Slug)
	s.publishEvents(ctx, domain.NewArticleHiddenEvent(a))
	return a, nil
}

func (s *Service) Archive(ctx context.Context, id int64) (*domain.Article, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.Archive(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, id, a.Status, nil); err != nil {
		return nil, err
	}
	s.cache.Del(ctx, a.Slug)
	s.publishEvents(ctx, domain.NewArticleArchivedEvent(a))
	return a, nil
}
