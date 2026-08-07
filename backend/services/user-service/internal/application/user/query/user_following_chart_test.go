package query

import (
	"context"
	"errors"
	"testing"

	domain "user-service/internal/domain/user"
)

func TestGetUserFollowingChartNormalizesAndForwardsQuery(t *testing.T) {
	zero := int64(0)
	repo := &userFollowingChartRepoStub{result: domain.UserFollowingChart{
		Local: domain.UserFollowingChartScope{Followings: domain.UserChartSeries{Total: []int64{2}}},
	}}
	svc := NewService(repo, nil)
	got, err := svc.GetUserFollowingChart(context.Background(), domain.UserFollowingChartQuery{
		Span: " HOUR ", Offset: &zero, UserID: 42,
	})
	if err != nil {
		t.Fatalf("GetUserFollowingChart() error = %v", err)
	}
	if repo.query.Span != domain.UserChartSpanHour || repo.query.Limit != domain.DefaultUserChartLimit || repo.query.UserID != 42 {
		t.Fatalf("repository query = %+v", repo.query)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 {
		t.Fatalf("repository offset = %v, want explicit zero", repo.query.Offset)
	}
	if got.Local.Followings.Total[0] != 2 {
		t.Fatalf("result = %+v", got)
	}
}

func TestGetUserFollowingChartRejectsInvalidParameters(t *testing.T) {
	negative := int64(-1)
	tooLarge := domain.MaxUserChartOffsetMillis + 1
	tests := []struct {
		name  string
		query domain.UserFollowingChartQuery
		want  error
	}{
		{name: "span", query: domain.UserFollowingChartQuery{Span: "week", Limit: 1, UserID: 1}, want: domain.ErrUserChartSpanInvalid},
		{name: "negative limit", query: domain.UserFollowingChartQuery{Span: "day", Limit: -1, UserID: 1}, want: domain.ErrUserChartLimitInvalid},
		{name: "large limit", query: domain.UserFollowingChartQuery{Span: "day", Limit: 501, UserID: 1}, want: domain.ErrUserChartLimitInvalid},
		{name: "negative offset", query: domain.UserFollowingChartQuery{Span: "day", Limit: 1, Offset: &negative, UserID: 1}, want: domain.ErrUserChartOffsetInvalid},
		{name: "large offset", query: domain.UserFollowingChartQuery{Span: "day", Limit: 1, Offset: &tooLarge, UserID: 1}, want: domain.ErrUserChartOffsetInvalid},
		{name: "missing user", query: domain.UserFollowingChartQuery{Span: "day", Limit: 1}, want: domain.ErrInvalidID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := NewService(&userFollowingChartRepoStub{}, nil)
			_, err := svc.GetUserFollowingChart(context.Background(), test.query)
			if !errors.Is(err, test.want) {
				t.Fatalf("GetUserFollowingChart() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestGetUserFollowingChartFailsClosedWithoutRepository(t *testing.T) {
	svc := NewService(&repoStub{}, nil)
	_, err := svc.GetUserFollowingChart(context.Background(), domain.UserFollowingChartQuery{
		Span: domain.UserChartSpanDay, Limit: 1, UserID: 1,
	})
	if !errors.Is(err, domain.ErrUserFollowingChartRepositoryUnavailable) {
		t.Fatalf("GetUserFollowingChart() error = %v, want repository unavailable", err)
	}
}

type userFollowingChartRepoStub struct {
	domain.Repository
	query  domain.UserFollowingChartQuery
	result domain.UserFollowingChart
	err    error
}

func (r *userFollowingChartRepoStub) GetUserFollowingChart(_ context.Context, q domain.UserFollowingChartQuery) (domain.UserFollowingChart, error) {
	r.query = q
	return r.result, r.err
}
