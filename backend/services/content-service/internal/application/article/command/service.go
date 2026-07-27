package command

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	outboxapp "content-service/internal/application/outbox"
	domain "content-service/internal/domain/article"
	outboxDomain "content-service/internal/domain/outbox"
	"content-service/internal/infrastructure/cache"
	"content-service/internal/infrastructure/messaging"
	"content-service/pkg/logger"
)

type IDGenerator interface {
	Generate() int64
}

type Service struct {
	repo            domain.Repository
	cache           *cache.ArticleCache
	idgen           IDGenerator
	publisher       messaging.EventPublisher
	lifecycleOutbox *outboxapp.LifecycleDispatcher
	log             logger.Logger
}

type lifecycleStatusRepository interface {
	UpdateStatusWithOutbox(ctx context.Context, id int64, status domain.Status, publishedAt *time.Time, updatedAt time.Time, event outboxDomain.LifecycleEvent) error
}

func NewService(repo domain.Repository, c *cache.ArticleCache, idgen IDGenerator, publisher messaging.EventPublisher, log logger.Logger, lifecycleOutboxes ...*outboxapp.LifecycleDispatcher) *Service {
	var lifecycleOutbox *outboxapp.LifecycleDispatcher
	if len(lifecycleOutboxes) > 0 {
		lifecycleOutbox = lifecycleOutboxes[0]
	}
	return &Service{repo: repo, cache: c, idgen: idgen, publisher: publisher, lifecycleOutbox: lifecycleOutbox, log: log}
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
	if err := s.persistLifecycleStatus(ctx, a, domain.NewArticlePublishedEvent(a)); err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.Del(ctx, a.Slug)
	}
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
	if err := s.persistLifecycleStatus(ctx, a, domain.NewArticleHiddenEvent(a)); err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.Del(ctx, a.Slug)
	}
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
	if err := s.persistLifecycleStatus(ctx, a, domain.NewArticleArchivedEvent(a)); err != nil {
		return nil, err
	}
	if s.cache != nil {
		s.cache.Del(ctx, a.Slug)
	}
	return a, nil
}

func (s *Service) persistLifecycleStatus(ctx context.Context, article *domain.Article, event domain.DomainEvent) error {
	if s.lifecycleOutbox == nil {
		if err := s.repo.UpdateStatus(ctx, article.ID, article.Status, article.PublishedAt); err != nil {
			return err
		}
		s.publishEvents(ctx, event)
		return nil
	}

	identified, ok := event.(interface{ EventID() string })
	if !ok || strings.TrimSpace(identified.EventID()) == "" {
		return errors.New("content lifecycle event must have an event id")
	}
	payload, err := messaging.EncodeDomainEvent(ctx, event)
	if err != nil {
		return err
	}
	updater, ok := s.repo.(lifecycleStatusRepository)
	if !ok {
		return errors.New("content lifecycle outbox status repository is unavailable")
	}
	outboxEvent := outboxDomain.LifecycleEvent{
		EventID:    identified.EventID(),
		MessageKey: strconv.FormatInt(article.ID, 10),
		EventType:  event.EventName(),
		Payload:    payload,
	}
	if err := updater.UpdateStatusWithOutbox(ctx, article.ID, article.Status, article.PublishedAt, article.UpdatedAt, outboxEvent); err != nil {
		return err
	}
	if _, err := s.lifecycleOutbox.DispatchEvent(ctx, outboxEvent.EventID); err != nil && s.log != nil {
		s.log.Warn("publish content lifecycle outbox event failed", logger.Error(err))
	}
	return nil
}
