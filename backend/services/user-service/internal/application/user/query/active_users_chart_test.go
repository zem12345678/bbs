package query

import (
	"context"
	"errors"
	"testing"

	domain "user-service/internal/domain/user"
)

func TestGetActiveUsersChartNormalizesAndForwardsQuery(t *testing.T) {
	zero := int64(0)
	repo := &activeUsersChartRepoStub{result: domain.ActiveUsersChart{Buckets: []domain.ActiveUsersChartBucket{{ReadUserIDs: []int64{42}}}}}
	got, err := NewService(repo, nil).GetActiveUsersChart(context.Background(), domain.UserChartQuery{Span: " HOUR ", Offset: &zero})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	if repo.query.Span != domain.UserChartSpanHour || repo.query.Limit != domain.DefaultUserChartLimit {
		t.Fatalf("repository query = %+v", repo.query)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 || got.Buckets[0].ReadUserIDs[0] != 42 {
		t.Fatalf("query/result = %+v / %+v", repo.query, got)
	}
}

func TestGetActiveUsersChartRejectsInvalidParametersAndMissingRepository(t *testing.T) {
	negative := int64(-1)
	tooLarge := domain.MaxUserChartOffsetMillis + 1
	tests := []struct {
		name  string
		query domain.UserChartQuery
		want  error
	}{
		{name: "span", query: domain.UserChartQuery{Span: "week", Limit: 1}, want: domain.ErrUserChartSpanInvalid},
		{name: "limit", query: domain.UserChartQuery{Span: "day", Limit: 501}, want: domain.ErrUserChartLimitInvalid},
		{name: "negative offset", query: domain.UserChartQuery{Span: "day", Limit: 1, Offset: &negative}, want: domain.ErrUserChartOffsetInvalid},
		{name: "large offset", query: domain.UserChartQuery{Span: "day", Limit: 1, Offset: &tooLarge}, want: domain.ErrUserChartOffsetInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(&activeUsersChartRepoStub{}, nil).GetActiveUsersChart(context.Background(), test.query)
			if !errors.Is(err, test.want) {
				t.Fatalf("GetActiveUsersChart() error = %v, want %v", err, test.want)
			}
		})
	}
	_, err := NewService(&repoStub{}, nil).GetActiveUsersChart(context.Background(), domain.UserChartQuery{Span: "day", Limit: 1})
	if !errors.Is(err, domain.ErrActiveUsersChartRepositoryUnavailable) {
		t.Fatalf("missing repository error = %v", err)
	}
}

type activeUsersChartRepoStub struct {
	domain.Repository
	query  domain.UserChartQuery
	result domain.ActiveUsersChart
}

func (r *activeUsersChartRepoStub) GetActiveUsersChart(_ context.Context, q domain.UserChartQuery) (domain.ActiveUsersChart, error) {
	r.query = q
	return r.result, nil
}
