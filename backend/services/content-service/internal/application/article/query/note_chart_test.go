package query

import (
	"context"
	"errors"
	"testing"

	domain "content-service/internal/domain/article"
)

func TestGetNoteChartNormalizesAndForwardsQuery(t *testing.T) {
	zero := int64(0)
	repo := &noteChartRepoStub{result: domain.NoteChart{Local: domain.NoteChartSeries{Total: []int64{2}}}}
	svc := NewService(repo, nil, nil, nil)
	got, err := svc.GetNoteChart(context.Background(), domain.NoteChartQuery{Span: " HOUR ", Offset: &zero, UserID: 42})
	if err != nil {
		t.Fatalf("GetNoteChart() error = %v", err)
	}
	if repo.query.Span != domain.NoteChartSpanHour || repo.query.Limit != domain.DefaultNoteChartLimit || repo.query.UserID != 42 {
		t.Fatalf("repository query = %+v", repo.query)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 || got.Local.Total[0] != 2 {
		t.Fatalf("query/result = %+v / %+v", repo.query, got)
	}
}

func TestGetNoteChartRejectsInvalidParametersAndMissingRepository(t *testing.T) {
	negative := int64(-1)
	tooLarge := domain.MaxNoteChartOffsetMillis + 1
	tests := []struct {
		name  string
		query domain.NoteChartQuery
		want  error
	}{
		{name: "span", query: domain.NoteChartQuery{Span: "week", Limit: 1}, want: domain.ErrNoteChartSpanInvalid},
		{name: "limit", query: domain.NoteChartQuery{Span: "day", Limit: 501}, want: domain.ErrNoteChartLimitInvalid},
		{name: "negative offset", query: domain.NoteChartQuery{Span: "day", Limit: 1, Offset: &negative}, want: domain.ErrNoteChartOffsetInvalid},
		{name: "large offset", query: domain.NoteChartQuery{Span: "day", Limit: 1, Offset: &tooLarge}, want: domain.ErrNoteChartOffsetInvalid},
		{name: "user", query: domain.NoteChartQuery{Span: "day", Limit: 1, UserID: -1}, want: domain.ErrNoteChartUserInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(&noteChartRepoStub{}, nil, nil, nil).GetNoteChart(context.Background(), test.query)
			if !errors.Is(err, test.want) {
				t.Fatalf("GetNoteChart() error = %v, want %v", err, test.want)
			}
		})
	}
	_, err := NewService(&articleRepoWithoutNoteChart{}, nil, nil, nil).GetNoteChart(context.Background(), domain.NoteChartQuery{Span: "day", Limit: 1})
	if !errors.Is(err, domain.ErrNoteChartRepositoryUnavailable) {
		t.Fatalf("missing repository error = %v", err)
	}
}

type noteChartRepoStub struct {
	domain.Repository
	query  domain.NoteChartQuery
	result domain.NoteChart
}

func (r *noteChartRepoStub) GetNoteChart(_ context.Context, q domain.NoteChartQuery) (domain.NoteChart, error) {
	r.query = q
	return r.result, nil
}

type articleRepoWithoutNoteChart struct{ domain.Repository }
