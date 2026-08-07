package query

import (
	"context"
	"strings"

	domain "user-service/internal/domain/user"
)

func (s *Service) GetUserFollowingChart(ctx context.Context, q domain.UserFollowingChartQuery) (domain.UserFollowingChart, error) {
	q.Span = strings.ToLower(strings.TrimSpace(q.Span))
	if q.Span != domain.UserChartSpanHour && q.Span != domain.UserChartSpanDay {
		return domain.UserFollowingChart{}, domain.ErrUserChartSpanInvalid
	}
	if q.Limit == 0 {
		q.Limit = domain.DefaultUserChartLimit
	}
	if q.Limit < 1 || q.Limit > domain.MaxUserChartLimit {
		return domain.UserFollowingChart{}, domain.ErrUserChartLimitInvalid
	}
	if q.Offset != nil && (*q.Offset < 0 || *q.Offset > domain.MaxUserChartOffsetMillis) {
		return domain.UserFollowingChart{}, domain.ErrUserChartOffsetInvalid
	}
	if q.UserID <= 0 {
		return domain.UserFollowingChart{}, domain.ErrInvalidID
	}
	repo, ok := s.repo.(domain.UserFollowingChartRepository)
	if !ok {
		return domain.UserFollowingChart{}, domain.ErrUserFollowingChartRepositoryUnavailable
	}
	return repo.GetUserFollowingChart(ctx, q)
}
