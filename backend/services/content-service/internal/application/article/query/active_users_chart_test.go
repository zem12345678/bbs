package query

import (
	"context"
	"errors"
	"testing"

	domain "content-service/internal/domain/article"
)

func TestGetActiveUsersChartNormalizesAndForwardsQuery(t *testing.T) {
	zero := int64(0)
	repo := &activeUsersChartRepoStub{result: domain.ActiveUsersChart{Buckets: []domain.ActiveUsersChartBucket{{WriterUserIDs: []int64{42}}}}}
	got, err := NewService(repo, nil, nil, nil).GetActiveUsersChart(context.Background(), domain.NoteChartQuery{Span: " HOUR ", Offset: &zero})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	if repo.query.Span != domain.NoteChartSpanHour || repo.query.Limit != domain.DefaultNoteChartLimit {
		t.Fatalf("repository query = %+v", repo.query)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 || got.Buckets[0].WriterUserIDs[0] != 42 {
		t.Fatalf("query/result = %+v / %+v", repo.query, got)
	}
}

func TestGetActiveUsersChartRejectsInvalidParametersAndMissingRepository(t *testing.T) {
	negative := int64(-1)
	tests := []struct {
		query domain.NoteChartQuery
		want  error
	}{
		{query: domain.NoteChartQuery{Span: "week", Limit: 1}, want: domain.ErrNoteChartSpanInvalid},
		{query: domain.NoteChartQuery{Span: "day", Limit: 501}, want: domain.ErrNoteChartLimitInvalid},
		{query: domain.NoteChartQuery{Span: "day", Limit: 1, Offset: &negative}, want: domain.ErrNoteChartOffsetInvalid},
	}
	for _, test := range tests {
		_, err := NewService(&activeUsersChartRepoStub{}, nil, nil, nil).GetActiveUsersChart(context.Background(), test.query)
		if !errors.Is(err, test.want) {
			t.Fatalf("GetActiveUsersChart() error = %v, want %v", err, test.want)
		}
	}
	_, err := NewService(&articleRepoWithoutActiveUsersChart{}, nil, nil, nil).GetActiveUsersChart(context.Background(), domain.NoteChartQuery{Span: "day", Limit: 1})
	if !errors.Is(err, domain.ErrActiveUsersChartRepositoryUnavailable) {
		t.Fatalf("missing repository error = %v", err)
	}
}

type activeUsersChartRepoStub struct {
	domain.Repository
	query  domain.NoteChartQuery
	result domain.ActiveUsersChart
}

func (r *activeUsersChartRepoStub) GetActiveUsersChart(_ context.Context, q domain.NoteChartQuery) (domain.ActiveUsersChart, error) {
	r.query = q
	return r.result, nil
}

type articleRepoWithoutActiveUsersChart struct{ domain.Repository }
