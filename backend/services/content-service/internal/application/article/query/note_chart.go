package query

import (
	"context"
	"strings"

	domain "content-service/internal/domain/article"
)

func (s *Service) GetNoteChart(ctx context.Context, q domain.NoteChartQuery) (domain.NoteChart, error) {
	q.Span = strings.ToLower(strings.TrimSpace(q.Span))
	if q.Span != domain.NoteChartSpanHour && q.Span != domain.NoteChartSpanDay {
		return domain.NoteChart{}, domain.ErrNoteChartSpanInvalid
	}
	if q.Limit == 0 {
		q.Limit = domain.DefaultNoteChartLimit
	}
	if q.Limit < 1 || q.Limit > domain.MaxNoteChartLimit {
		return domain.NoteChart{}, domain.ErrNoteChartLimitInvalid
	}
	if q.Offset != nil && (*q.Offset < 0 || *q.Offset > domain.MaxNoteChartOffsetMillis) {
		return domain.NoteChart{}, domain.ErrNoteChartOffsetInvalid
	}
	if q.UserID < 0 {
		return domain.NoteChart{}, domain.ErrNoteChartUserInvalid
	}
	repo, ok := s.repo.(domain.NoteChartRepository)
	if !ok {
		return domain.NoteChart{}, domain.ErrNoteChartRepositoryUnavailable
	}
	return repo.GetNoteChart(ctx, q)
}
