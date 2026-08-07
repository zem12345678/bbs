package query

import (
	"context"
	"testing"

	domain "content-service/internal/domain/channel"
)

func TestServicePassesListFilter(t *testing.T) {
	repo := &fakeChannelQueryRepo{channels: []*domain.Channel{{ID: 1}}, total: 3}
	service := NewService(repo)
	filter := domain.ListFilter{FollowerUserID: 10, FavoritedUserID: 11, ViewerID: 12, Uncategorized: true, Featured: true}
	channels, total, err := service.List(context.Background(), filter)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(channels) != 1 || total != 3 {
		t.Fatalf("List = %d/%d, want 1/3", len(channels), total)
	}
	if repo.filter != filter {
		t.Fatalf("filter = %#v, want %#v", repo.filter, filter)
	}
}

type fakeChannelQueryRepo struct {
	channels []*domain.Channel
	total    int64
	filter   domain.ListFilter
}

func (*fakeChannelQueryRepo) CreateChannel(context.Context, *domain.Channel) error  { return nil }
func (*fakeChannelQueryRepo) UpdateChannel(context.Context, *domain.Channel) error  { return nil }
func (*fakeChannelQueryRepo) ArchiveChannel(context.Context, *domain.Channel) error { return nil }
func (*fakeChannelQueryRepo) FindChannelByID(context.Context, int64, int64, bool) (*domain.Channel, error) {
	return nil, domain.ErrNotFound
}
func (r *fakeChannelQueryRepo) ListChannels(_ context.Context, filter domain.ListFilter) ([]*domain.Channel, int64, error) {
	r.filter = filter
	return r.channels, r.total, nil
}
func (*fakeChannelQueryRepo) FollowChannel(context.Context, int64, int64) error     { return nil }
func (*fakeChannelQueryRepo) UnfollowChannel(context.Context, int64, int64) error   { return nil }
func (*fakeChannelQueryRepo) FavoriteChannel(context.Context, int64, int64) error   { return nil }
func (*fakeChannelQueryRepo) UnfavoriteChannel(context.Context, int64, int64) error { return nil }
func (*fakeChannelQueryRepo) ListChannelCategoryAggregates(context.Context, bool) ([]domain.CategoryAggregate, error) {
	return nil, nil
}
