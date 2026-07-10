package query

import (
	"context"
	"testing"
	"time"

	domain "content-service/internal/domain/article"
)

func TestGetByIDIncrementsViewCount(t *testing.T) {
	repo := &fakeArticleRepo{
		article: &domain.Article{
			ID:        1,
			Slug:      "first",
			Title:     "First article",
			Body:      "body",
			AuthorID:  10,
			Status:    domain.StatusPublished,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ViewCount: 4,
		},
		nextViewCount: 5,
	}
	service := NewService(repo, nil)

	view, err := service.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if repo.incrementedID != 1 {
		t.Fatalf("expected increment for article 1, got %d", repo.incrementedID)
	}
	if got := view.Article.ViewCount; got != 5 {
		t.Fatalf("expected returned view count 5, got %d", got)
	}
}

type fakeArticleRepo struct {
	article       *domain.Article
	nextViewCount int64
	incrementedID int64
}

func (f *fakeArticleRepo) Create(context.Context, *domain.Article) error { return nil }
func (f *fakeArticleRepo) Update(context.Context, *domain.Article) error { return nil }
func (f *fakeArticleRepo) FindBySlug(context.Context, string) (*domain.Article, error) {
	return f.article, nil
}
func (f *fakeArticleRepo) FindByID(context.Context, int64) (*domain.Article, error) {
	return f.article, nil
}
func (f *fakeArticleRepo) List(context.Context, domain.Status, string, int64, int, int) ([]*domain.Article, error) {
	return nil, nil
}
func (f *fakeArticleRepo) ListTags(context.Context, domain.Status, string, int) ([]domain.TagStats, error) {
	return nil, nil
}
func (f *fakeArticleRepo) UpdateStatus(context.Context, int64, domain.Status, *time.Time) error {
	return nil
}
func (f *fakeArticleRepo) FeedByTime(context.Context, int, int) ([]*domain.Article, error) {
	return nil, nil
}
func (f *fakeArticleRepo) FindByIDs(context.Context, []int64) (map[int64]*domain.Article, error) {
	return nil, nil
}
func (f *fakeArticleRepo) IncrementViewCount(_ context.Context, id int64) (int64, error) {
	f.incrementedID = id
	return f.nextViewCount, nil
}
