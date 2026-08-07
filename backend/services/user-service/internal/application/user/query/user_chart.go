package query

import (
	"context"
	"strings"

	domain "user-service/internal/domain/user"
)

func (s *Service) GetUserChart(ctx context.Context, q domain.UserChartQuery) (domain.UserChart, error) {
	q.Span = strings.ToLower(strings.TrimSpace(q.Span))
	if q.Span != domain.UserChartSpanHour && q.Span != domain.UserChartSpanDay {
		return domain.UserChart{}, domain.ErrUserChartSpanInvalid
	}
	if q.Limit == 0 {
		q.Limit = domain.DefaultUserChartLimit
	}
	if q.Limit < 1 || q.Limit > domain.MaxUserChartLimit {
		return domain.UserChart{}, domain.ErrUserChartLimitInvalid
	}
	if q.Offset != nil && (*q.Offset < 0 || *q.Offset > domain.MaxUserChartOffsetMillis) {
		return domain.UserChart{}, domain.ErrUserChartOffsetInvalid
	}
	repo, ok := s.repo.(domain.UserChartRepository)
	if !ok {
		return domain.UserChart{}, domain.ErrUserChartRepositoryUnavailable
	}
	return repo.GetUserChart(ctx, q)
}
