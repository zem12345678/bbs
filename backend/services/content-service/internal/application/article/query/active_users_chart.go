package query

import (
	"context"
	"strings"

	domain "content-service/internal/domain/article"
)

func (s *Service) GetActiveUsersChart(ctx context.Context, q domain.NoteChartQuery) (domain.ActiveUsersChart, error) {
	q.Span = strings.ToLower(strings.TrimSpace(q.Span))
	if q.Span != domain.NoteChartSpanHour && q.Span != domain.NoteChartSpanDay {
		return domain.ActiveUsersChart{}, domain.ErrNoteChartSpanInvalid
	}
	if q.Limit == 0 {
		q.Limit = domain.DefaultNoteChartLimit
	}
	if q.Limit < 1 || q.Limit > domain.MaxNoteChartLimit {
		return domain.ActiveUsersChart{}, domain.ErrNoteChartLimitInvalid
	}
	if q.Offset != nil && (*q.Offset < 0 || *q.Offset > domain.MaxNoteChartOffsetMillis) {
		return domain.ActiveUsersChart{}, domain.ErrNoteChartOffsetInvalid
	}
	repo, ok := s.repo.(domain.ActiveUsersChartRepository)
	if !ok {
		return domain.ActiveUsersChart{}, domain.ErrActiveUsersChartRepositoryUnavailable
	}
	return repo.GetActiveUsersChart(ctx, q)
}
