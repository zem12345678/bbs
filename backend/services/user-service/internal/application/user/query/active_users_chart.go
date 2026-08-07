package query

import (
	"context"
	"strings"

	domain "user-service/internal/domain/user"
)

func (s *Service) GetActiveUsersChart(ctx context.Context, q domain.UserChartQuery) (domain.ActiveUsersChart, error) {
	q.Span = strings.ToLower(strings.TrimSpace(q.Span))
	if q.Span != domain.UserChartSpanHour && q.Span != domain.UserChartSpanDay {
		return domain.ActiveUsersChart{}, domain.ErrUserChartSpanInvalid
	}
	if q.Limit == 0 {
		q.Limit = domain.DefaultUserChartLimit
	}
	if q.Limit < 1 || q.Limit > domain.MaxUserChartLimit {
		return domain.ActiveUsersChart{}, domain.ErrUserChartLimitInvalid
	}
	if q.Offset != nil && (*q.Offset < 0 || *q.Offset > domain.MaxUserChartOffsetMillis) {
		return domain.ActiveUsersChart{}, domain.ErrUserChartOffsetInvalid
	}
	repo, ok := s.repo.(domain.ActiveUsersChartRepository)
	if !ok {
		return domain.ActiveUsersChart{}, domain.ErrActiveUsersChartRepositoryUnavailable
	}
	return repo.GetActiveUsersChart(ctx, q)
}
