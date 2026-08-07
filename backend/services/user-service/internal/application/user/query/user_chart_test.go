package query

import (
	"context"
	"errors"
	"testing"

	domain "user-service/internal/domain/user"
)

func TestGetUserChartNormalizesAndForwardsQuery(t *testing.T) {
	zero := int64(0)
	repo := &userChartRepoStub{result: domain.UserChart{
		Local: domain.UserChartSeries{Total: []int64{2}, Inc: []int64{1}, Dec: []int64{0}},
	}}
	svc := NewService(repo, nil)

	got, err := svc.GetUserChart(context.Background(), domain.UserChartQuery{
		Span: " HOUR ", Offset: &zero,
	})
	if err != nil {
		t.Fatalf("GetUserChart() error = %v", err)
	}
	if repo.query.Span != domain.UserChartSpanHour || repo.query.Limit != domain.DefaultUserChartLimit {
		t.Fatalf("repository query = %+v", repo.query)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 {
		t.Fatalf("repository offset = %v, want explicit zero", repo.query.Offset)
	}
	if len(got.Local.Total) != 1 || got.Local.Total[0] != 2 {
		t.Fatalf("result = %+v", got)
	}
}

func TestGetUserChartRejectsInvalidParameters(t *testing.T) {
	negative := int64(-1)
	tooLarge := domain.MaxUserChartOffsetMillis + 1
	tests := []struct {
		name  string
		query domain.UserChartQuery
		want  error
	}{
		{name: "span", query: domain.UserChartQuery{Span: "week", Limit: 1}, want: domain.ErrUserChartSpanInvalid},
		{name: "negative limit", query: domain.UserChartQuery{Span: "day", Limit: -1}, want: domain.ErrUserChartLimitInvalid},
		{name: "large limit", query: domain.UserChartQuery{Span: "day", Limit: 501}, want: domain.ErrUserChartLimitInvalid},
		{name: "negative offset", query: domain.UserChartQuery{Span: "day", Limit: 1, Offset: &negative}, want: domain.ErrUserChartOffsetInvalid},
		{name: "large offset", query: domain.UserChartQuery{Span: "day", Limit: 1, Offset: &tooLarge}, want: domain.ErrUserChartOffsetInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&userChartRepoStub{}, nil)
			_, err := svc.GetUserChart(context.Background(), tt.query)
			if !errors.Is(err, tt.want) {
				t.Fatalf("GetUserChart() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGetUserChartFailsClosedWithoutRepository(t *testing.T) {
	svc := NewService(&repoStub{}, nil)
	_, err := svc.GetUserChart(context.Background(), domain.UserChartQuery{Span: domain.UserChartSpanDay, Limit: 1})
	if !errors.Is(err, domain.ErrUserChartRepositoryUnavailable) {
		t.Fatalf("GetUserChart() error = %v, want ErrUserChartRepositoryUnavailable", err)
	}
}

func TestGetUserChartAcceptsMaximumOffset(t *testing.T) {
	offset := domain.MaxUserChartOffsetMillis
	repo := &userChartRepoStub{}
	svc := NewService(repo, nil)
	if _, err := svc.GetUserChart(context.Background(), domain.UserChartQuery{
		Span: domain.UserChartSpanDay, Limit: domain.MaxUserChartLimit, Offset: &offset,
	}); err != nil {
		t.Fatalf("GetUserChart() maximum offset error = %v", err)
	}
	if repo.query.Offset == nil || *repo.query.Offset != offset {
		t.Fatalf("repository offset = %v, want %d", repo.query.Offset, offset)
	}
}

type userChartRepoStub struct {
	domain.Repository
	query  domain.UserChartQuery
	result domain.UserChart
	err    error
}

func (r *userChartRepoStub) GetUserChart(_ context.Context, q domain.UserChartQuery) (domain.UserChart, error) {
	r.query = q
	return r.result, r.err
}
