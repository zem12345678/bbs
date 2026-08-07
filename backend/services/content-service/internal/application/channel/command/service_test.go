package command

import (
	"context"
	"errors"
	"testing"

	categoryDomain "content-service/internal/domain/category"
	domain "content-service/internal/domain/channel"
)

func TestServiceEnforcesChannelOwner(t *testing.T) {
	repo := newFakeChannelRepo()
	service := NewService(repo, fixedIDGenerator(100), &fakeCategoryReader{})
	created, err := service.Create(context.Background(), domain.CreateCmd{OwnerID: 20, Name: "name"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != 100 {
		t.Fatalf("ID = %d, want 100", created.ID)
	}
	if _, err := service.Update(context.Background(), created.ID, 21, domain.UpdateCmd{Name: "renamed"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Update error = %v, want %v", err, domain.ErrForbidden)
	}
	updated, err := service.Update(context.Background(), created.ID, 20, domain.UpdateCmd{Name: "renamed", Color: "#abcdef"})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Name != "renamed" || updated.Color != "#abcdef" {
		t.Fatalf("unexpected updated channel: %#v", updated)
	}
}

func TestServiceDelegatesIdempotentRelations(t *testing.T) {
	repo := newFakeChannelRepo()
	service := NewService(repo, fixedIDGenerator(100), &fakeCategoryReader{})
	for range 2 {
		if err := service.Follow(context.Background(), 10, 20); err != nil {
			t.Fatalf("Follow returned error: %v", err)
		}
		if err := service.Favorite(context.Background(), 10, 20); err != nil {
			t.Fatalf("Favorite returned error: %v", err)
		}
	}
	if repo.followCalls != 2 || repo.favoriteCalls != 2 {
		t.Fatalf("relation calls = %d/%d, want 2/2", repo.followCalls, repo.favoriteCalls)
	}
}

func TestServiceValidatesCategoryOnCreate(t *testing.T) {
	tests := []struct {
		name       string
		categoryID int64
		categories map[int64]*categoryDomain.Category
		wantErr    error
		wantCalls  int
	}{
		{name: "uncategorized", categoryID: 0},
		{name: "enabled", categoryID: 7, categories: map[int64]*categoryDomain.Category{7: {ID: 7, Status: categoryDomain.StatusEnabled}}, wantCalls: 1},
		{name: "missing", categoryID: 8, wantErr: categoryDomain.ErrNotFound, wantCalls: 1},
		{name: "disabled", categoryID: 9, categories: map[int64]*categoryDomain.Category{9: {ID: 9, Status: categoryDomain.StatusDisabled}}, wantErr: domain.ErrCategoryDisabled, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeChannelRepo()
			categories := &fakeCategoryReader{categories: tt.categories}
			service := NewService(repo, fixedIDGenerator(100), categories)

			_, err := service.Create(context.Background(), domain.CreateCmd{OwnerID: 20, CategoryID: tt.categoryID, Name: "name"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create error = %v, want %v", err, tt.wantErr)
			}
			if categories.calls != tt.wantCalls {
				t.Fatalf("category reader calls = %d, want %d", categories.calls, tt.wantCalls)
			}
			wantCreated := 1
			if tt.wantErr != nil {
				wantCreated = 0
			}
			if len(repo.channels) != wantCreated {
				t.Fatalf("created channels = %d, want %d", len(repo.channels), wantCreated)
			}
		})
	}
}

func TestServiceValidatesCategoryOnUpdate(t *testing.T) {
	tests := []struct {
		name       string
		categoryID int64
		categories map[int64]*categoryDomain.Category
		wantErr    error
		wantCalls  int
	}{
		{name: "uncategorized", categoryID: 0},
		{name: "enabled", categoryID: 7, categories: map[int64]*categoryDomain.Category{7: {ID: 7, Status: categoryDomain.StatusEnabled}}, wantCalls: 1},
		{name: "missing", categoryID: 8, wantErr: categoryDomain.ErrNotFound, wantCalls: 1},
		{name: "disabled", categoryID: 9, categories: map[int64]*categoryDomain.Category{9: {ID: 9, Status: categoryDomain.StatusDisabled}}, wantErr: domain.ErrCategoryDisabled, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeChannelRepo()
			categories := &fakeCategoryReader{categories: tt.categories}
			service := NewService(repo, fixedIDGenerator(100), categories)
			created, err := service.Create(context.Background(), domain.CreateCmd{OwnerID: 20, Name: "name"})
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}

			_, err = service.Update(context.Background(), created.ID, created.OwnerID, domain.UpdateCmd{CategoryID: tt.categoryID, Name: "renamed"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Update error = %v, want %v", err, tt.wantErr)
			}
			if categories.calls != tt.wantCalls {
				t.Fatalf("category reader calls = %d, want %d", categories.calls, tt.wantCalls)
			}
			stored := repo.channels[created.ID]
			if tt.wantErr != nil && (stored.CategoryID != 0 || stored.Name != "name") {
				t.Fatalf("rejected update mutated stored channel: %#v", stored)
			}
		})
	}
}

type fixedIDGenerator int64

func (g fixedIDGenerator) Generate() int64 { return int64(g) }

type fakeCategoryReader struct {
	categories map[int64]*categoryDomain.Category
	calls      int
}

func (r *fakeCategoryReader) FindCategoryByID(_ context.Context, id int64) (*categoryDomain.Category, error) {
	r.calls++
	category := r.categories[id]
	if category == nil {
		return nil, categoryDomain.ErrNotFound
	}
	copy := *category
	return &copy, nil
}

type fakeChannelRepo struct {
	channels      map[int64]*domain.Channel
	followCalls   int
	favoriteCalls int
}

func newFakeChannelRepo() *fakeChannelRepo {
	return &fakeChannelRepo{channels: make(map[int64]*domain.Channel)}
}

func (r *fakeChannelRepo) CreateChannel(_ context.Context, channel *domain.Channel) error {
	copy := *channel
	r.channels[channel.ID] = &copy
	return nil
}

func (r *fakeChannelRepo) UpdateChannel(_ context.Context, channel *domain.Channel) error {
	copy := *channel
	r.channels[channel.ID] = &copy
	return nil
}

func (r *fakeChannelRepo) ArchiveChannel(ctx context.Context, channel *domain.Channel) error {
	return r.UpdateChannel(ctx, channel)
}

func (r *fakeChannelRepo) FindChannelByID(_ context.Context, id, _ int64, includeArchived bool) (*domain.Channel, error) {
	channel := r.channels[id]
	if channel == nil || (channel.IsArchived && !includeArchived) {
		return nil, domain.ErrNotFound
	}
	copy := *channel
	return &copy, nil
}

func (r *fakeChannelRepo) ListChannels(context.Context, domain.ListFilter) ([]*domain.Channel, int64, error) {
	return nil, 0, nil
}

func (r *fakeChannelRepo) FollowChannel(context.Context, int64, int64) error {
	r.followCalls++
	return nil
}

func (*fakeChannelRepo) UnfollowChannel(context.Context, int64, int64) error { return nil }

func (r *fakeChannelRepo) FavoriteChannel(context.Context, int64, int64) error {
	r.favoriteCalls++
	return nil
}

func (*fakeChannelRepo) UnfavoriteChannel(context.Context, int64, int64) error { return nil }

func (*fakeChannelRepo) ListChannelCategoryAggregates(context.Context, bool) ([]domain.CategoryAggregate, error) {
	return nil, nil
}
