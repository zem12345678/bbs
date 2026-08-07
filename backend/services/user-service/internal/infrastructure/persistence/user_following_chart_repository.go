package persistence

import (
	"context"
	"fmt"
	"time"

	domain "user-service/internal/domain/user"
)

type userFollowingChartRow struct {
	Total int64
	Inc   int64
	Dec   int64
}

func (r *Repo) GetUserFollowingChart(ctx context.Context, q domain.UserFollowingChartQuery) (domain.UserFollowingChart, error) {
	windowQuery := domain.UserChartQuery{Span: q.Span, Limit: q.Limit, Offset: q.Offset}
	firstStart, lastStart, _, unit := userChartWindow(windowQuery, time.Now().UTC())
	interval := "1 hour"
	if unit == domain.UserChartSpanDay {
		interval = "1 day"
	}

	followings, err := r.getUserFollowingChartSeries(ctx, q.UserID, "follower_id", firstStart, lastStart, interval, unit)
	if err != nil {
		return domain.UserFollowingChart{}, err
	}
	followers, err := r.getUserFollowingChartSeries(ctx, q.UserID, "followee_id", firstStart, lastStart, interval, unit)
	if err != nil {
		return domain.UserFollowingChart{}, err
	}
	remote := domain.UserFollowingChartScope{
		Followings: zeroUserChartSeries(len(followings.Total)),
		Followers:  zeroUserChartSeries(len(followers.Total)),
	}
	return domain.UserFollowingChart{
		Local:  domain.UserFollowingChartScope{Followings: followings, Followers: followers},
		Remote: remote,
	}, nil
}

func (r *Repo) getUserFollowingChartSeries(ctx context.Context, userID int64, ownerColumn string, firstStart, lastStart time.Time, interval, unit string) (domain.UserChartSeries, error) {
	if ownerColumn != "follower_id" && ownerColumn != "followee_id" {
		return domain.UserChartSeries{}, fmt.Errorf("invalid follow chart owner column %q", ownerColumn)
	}
	query := fmt.Sprintf(`
WITH params AS (
  SELECT
    CAST(? AS timestamptz) AS first_start,
    CAST(? AS timestamptz) AS last_start,
    CAST(? AS interval) AS step
),
buckets AS (
  SELECT generate_series(p.first_start, p.last_start, p.step) AS bucket_start
  FROM params p
),
initial AS (
  SELECT COUNT(*)::bigint AS total
  FROM user_follow_lifecycles l
  CROSS JOIN params p
  WHERE l.%s = ?
    AND l.followed_at < p.first_start
    AND (l.unfollowed_at IS NULL OR l.unfollowed_at >= p.first_start)
),
increments AS (
  SELECT
    (date_trunc(?, l.followed_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS value
  FROM user_follow_lifecycles l
  CROSS JOIN params p
  WHERE l.%s = ?
    AND l.followed_at >= p.first_start
    AND l.followed_at < p.last_start + p.step
  GROUP BY 1
),
decrements AS (
  SELECT
    (date_trunc(?, l.unfollowed_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS value
  FROM user_follow_lifecycles l
  CROSS JOIN params p
  WHERE l.%s = ?
    AND l.unfollowed_at >= p.first_start
    AND l.unfollowed_at < p.last_start + p.step
  GROUP BY 1
)
SELECT
  (
    initial.total + SUM(COALESCE(increments.value, 0) - COALESCE(decrements.value, 0))
      OVER (ORDER BY buckets.bucket_start ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)
  )::bigint AS total,
  COALESCE(increments.value, 0)::bigint AS inc,
  COALESCE(decrements.value, 0)::bigint AS dec
FROM buckets
CROSS JOIN initial
LEFT JOIN increments USING (bucket_start)
LEFT JOIN decrements USING (bucket_start)
ORDER BY buckets.bucket_start DESC
`, ownerColumn, ownerColumn, ownerColumn)

	var rows []userFollowingChartRow
	err := r.db.WithContext(ctx).Raw(
		query,
		firstStart, lastStart, interval,
		userID,
		unit, userID,
		unit, userID,
	).Scan(&rows).Error
	if err != nil {
		return domain.UserChartSeries{}, err
	}
	series := domain.UserChartSeries{
		Total: make([]int64, 0, len(rows)),
		Inc:   make([]int64, 0, len(rows)),
		Dec:   make([]int64, 0, len(rows)),
	}
	for _, row := range rows {
		series.Total = append(series.Total, row.Total)
		series.Inc = append(series.Inc, row.Inc)
		series.Dec = append(series.Dec, row.Dec)
	}
	return series, nil
}

func zeroUserChartSeries(length int) domain.UserChartSeries {
	return domain.UserChartSeries{
		Total: make([]int64, length),
		Inc:   make([]int64, length),
		Dec:   make([]int64, length),
	}
}

var _ domain.UserFollowingChartRepository = (*Repo)(nil)
