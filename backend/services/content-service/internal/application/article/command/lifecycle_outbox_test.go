package command

import (
	"context"
	"errors"
	"testing"
	"time"

	outboxapp "content-service/internal/application/outbox"
	domain "content-service/internal/domain/article"
	outboxDomain "content-service/internal/domain/outbox"
	"content-service/internal/infrastructure/messaging"
)

func TestArticleLifecycleStatusChangesPersistAndPublishOutboxEvents(t *testing.T) {
	tests := []struct {
		name       string
		article    func(*testing.T) *domain.Article
		transition func(*Service) (*domain.Article, error)
		eventType  string
		status     domain.Status
	}{
		{
			name:       "publish",
			article:    draftArticle,
			transition: func(service *Service) (*domain.Article, error) { return service.Publish(context.Background(), 101) },
			eventType:  "article.published.v1",
			status:     domain.StatusPublished,
		},
		{
			name:       "hide",
			article:    publishedArticle,
			transition: func(service *Service) (*domain.Article, error) { return service.Hide(context.Background(), 101) },
			eventType:  "article.hidden.v1",
			status:     domain.StatusHidden,
		},
		{
			name:       "archive",
			article:    publishedArticle,
			transition: func(service *Service) (*domain.Article, error) { return service.Archive(context.Background(), 101) },
			eventType:  "article.archived.v1",
			status:     domain.StatusArchived,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outbox := &articleLifecycleOutbox{}
			repo := &articleLifecycleRepo{article: test.article(t), outbox: outbox}
			lifecyclePublisher := &articleLifecyclePublisher{}
			dispatcher := outboxapp.NewLifecycleDispatcher(outbox, lifecyclePublisher, outboxapp.LifecycleDispatcherOptions{Owner: "test-owner"})
			directPublisher := &articleEventPublisher{}
			service := NewService(repo, nil, articleIDGenerator{}, directPublisher, nil, dispatcher)

			article, err := test.transition(service)
			if err != nil {
				t.Fatalf("transition error = %v", err)
			}
			if article.Status != test.status || repo.article.Status != test.status {
				t.Fatalf("status = returned:%d stored:%d, want %d", article.Status, repo.article.Status, test.status)
			}
			if len(lifecyclePublisher.events) != 1 || lifecyclePublisher.events[0].EventType != test.eventType {
				t.Fatalf("lifecycle events = %+v, want one %q event", lifecyclePublisher.events, test.eventType)
			}
			if len(outbox.published) != 1 {
				t.Fatalf("published outbox events = %d, want 1", len(outbox.published))
			}
			if len(directPublisher.events) != 0 {
				t.Fatalf("direct events = %d, want 0 when lifecycle outbox is enabled", len(directPublisher.events))
			}
		})
	}
}

func TestArticleRepublishQueuesAfterPendingHide(t *testing.T) {
	outbox := &articleLifecycleOutbox{deferClaims: true}
	repo := &articleLifecycleRepo{article: publishedArticle(t), outbox: outbox}
	lifecyclePublisher := &articleLifecyclePublisher{}
	dispatcher := outboxapp.NewLifecycleDispatcher(outbox, lifecyclePublisher, outboxapp.LifecycleDispatcherOptions{Owner: "test-owner"})
	directPublisher := &articleEventPublisher{}
	service := NewService(repo, nil, articleIDGenerator{}, directPublisher, nil, dispatcher)

	if _, err := service.Hide(context.Background(), 101); err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	if _, err := service.Publish(context.Background(), 101); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if repo.article.Status != domain.StatusPublished {
		t.Fatalf("stored status = %d, want published", repo.article.Status)
	}
	if len(outbox.pending) != 2 {
		t.Fatalf("pending outbox events = %d, want 2", len(outbox.pending))
	}
	if outbox.pending[0].EventType != "article.hidden.v1" || outbox.pending[1].EventType != "article.published.v1" {
		t.Fatalf("pending outbox event order = %q, %q, want hidden then published", outbox.pending[0].EventType, outbox.pending[1].EventType)
	}
	if len(directPublisher.events) != 0 {
		t.Fatalf("direct events = %d, want 0 when lifecycle outbox is enabled", len(directPublisher.events))
	}
}

func TestArticleLifecycleOutboxWriteFailureRollsBackStatus(t *testing.T) {
	outbox := &articleLifecycleOutbox{}
	repo := &articleLifecycleRepo{article: publishedArticle(t), outbox: outbox, outboxErr: errors.New("outbox unavailable")}
	lifecyclePublisher := &articleLifecyclePublisher{}
	dispatcher := outboxapp.NewLifecycleDispatcher(outbox, lifecyclePublisher, outboxapp.LifecycleDispatcherOptions{Owner: "test-owner"})
	directPublisher := &articleEventPublisher{}
	service := NewService(repo, nil, articleIDGenerator{}, directPublisher, nil, dispatcher)

	_, err := service.Hide(context.Background(), 101)
	if !errors.Is(err, repo.outboxErr) {
		t.Fatalf("Hide() error = %v, want outbox error", err)
	}
	if repo.article.Status != domain.StatusPublished {
		t.Fatalf("stored status = %d, want published after rollback", repo.article.Status)
	}
	if len(outbox.pending) != 0 || len(lifecyclePublisher.events) != 0 || len(directPublisher.events) != 0 {
		t.Fatalf("events persisted or published after rollback: pending:%d lifecycle:%d direct:%d", len(outbox.pending), len(lifecyclePublisher.events), len(directPublisher.events))
	}
}

type articleIDGenerator struct{}

func (articleIDGenerator) Generate() int64 { return 1 }

type articleLifecycleRepo struct {
	article   *domain.Article
	outbox    *articleLifecycleOutbox
	outboxErr error
}

func (r *articleLifecycleRepo) Create(_ context.Context, article *domain.Article) error {
	r.article = cloneArticle(article)
	return nil
}

func (r *articleLifecycleRepo) Update(_ context.Context, article *domain.Article) error {
	r.article = cloneArticle(article)
	return nil
}

func (r *articleLifecycleRepo) FindBySlug(_ context.Context, slug string) (*domain.Article, error) {
	if r.article == nil || r.article.Slug != slug {
		return nil, domain.ErrNotFound
	}
	return cloneArticle(r.article), nil
}

func (r *articleLifecycleRepo) FindByID(_ context.Context, id int64) (*domain.Article, error) {
	if r.article == nil || r.article.ID != id {
		return nil, domain.ErrNotFound
	}
	return cloneArticle(r.article), nil
}

func (r *articleLifecycleRepo) List(context.Context, domain.Status, string, int64, string, int, int) ([]*domain.Article, int64, error) {
	return nil, 0, nil
}

func (r *articleLifecycleRepo) ListTags(context.Context, domain.Status, string, int) ([]domain.TagStats, error) {
	return nil, nil
}

func (r *articleLifecycleRepo) UpdateStatus(_ context.Context, id int64, status domain.Status, publishedAt *time.Time) error {
	if r.article == nil || r.article.ID != id {
		return domain.ErrNotFound
	}
	r.article.Status = status
	r.article.PublishedAt = publishedAt
	return nil
}

func (r *articleLifecycleRepo) UpdateStatusWithOutbox(ctx context.Context, id int64, status domain.Status, publishedAt *time.Time, _ time.Time, event outboxDomain.LifecycleEvent) error {
	before := cloneArticle(r.article)
	if err := r.UpdateStatus(ctx, id, status, publishedAt); err != nil {
		return err
	}
	if r.outboxErr != nil {
		r.article = before
		return r.outboxErr
	}
	r.outbox.pending = append(r.outbox.pending, event)
	return nil
}

func (r *articleLifecycleRepo) FeedByTime(context.Context, int, int) ([]*domain.Article, error) {
	return nil, nil
}

func (r *articleLifecycleRepo) FindByIDs(context.Context, []int64) (map[int64]*domain.Article, error) {
	return nil, nil
}

func (r *articleLifecycleRepo) IncrementViewCount(context.Context, int64) (int64, error) {
	return 0, nil
}

type articleLifecycleOutbox struct {
	pending     []outboxDomain.LifecycleEvent
	published   []string
	deferClaims bool
}

func (r *articleLifecycleOutbox) ClaimPendingLifecycleEvents(context.Context, string, int, time.Duration) ([]outboxDomain.LifecycleEvent, error) {
	events := r.pending
	r.pending = nil
	return events, nil
}

func (r *articleLifecycleOutbox) ClaimLifecycleEvent(_ context.Context, eventID, _ string, _ time.Duration) (*outboxDomain.LifecycleEvent, error) {
	if r.deferClaims {
		return nil, nil
	}
	for index, event := range r.pending {
		if event.EventID != eventID {
			continue
		}
		r.pending = append(r.pending[:index], r.pending[index+1:]...)
		return &event, nil
	}
	return nil, nil
}

func (r *articleLifecycleOutbox) MarkLifecycleEventPublished(_ context.Context, eventID, _ string, _ int) error {
	r.published = append(r.published, eventID)
	return nil
}

func (r *articleLifecycleOutbox) MarkLifecycleEventFailed(context.Context, string, string, int, string, time.Time) error {
	return nil
}

type articleLifecyclePublisher struct {
	events []outboxDomain.LifecycleEvent
}

func (p *articleLifecyclePublisher) PublishLifecycleEvent(_ context.Context, event outboxDomain.LifecycleEvent) error {
	p.events = append(p.events, event)
	return nil
}

type articleEventPublisher struct {
	events []messaging.DomainEvent
}

func (p *articleEventPublisher) PublishDomainEvents(_ context.Context, events []messaging.DomainEvent) error {
	p.events = append(p.events, events...)
	return nil
}

func publishedArticle(t *testing.T) *domain.Article {
	t.Helper()
	article := draftArticle(t)
	if err := article.Publish(); err != nil {
		t.Fatal(err)
	}
	return article
}

func draftArticle(t *testing.T) *domain.Article {
	t.Helper()
	article, err := domain.New(101, domain.CreateCmd{
		Slug:     "lifecycle-article",
		Title:    "生命周期文章",
		Body:     "body",
		AuthorID: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return article
}

func cloneArticle(article *domain.Article) *domain.Article {
	if article == nil {
		return nil
	}
	clone := *article
	if len(article.Tags) > 0 {
		clone.Tags = append([]string(nil), article.Tags...)
	}
	if article.PublishedAt != nil {
		publishedAt := *article.PublishedAt
		clone.PublishedAt = &publishedAt
	}
	return &clone
}
