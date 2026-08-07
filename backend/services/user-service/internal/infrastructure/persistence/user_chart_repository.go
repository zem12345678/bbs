package persistence

import (
	"context"
	"time"

	domain "user-service/internal/domain/user"
)

type userChartRow struct {
	Total int64
	Inc   int64
	Dec   int64
}

func (r *Repo) GetUserChart(ctx context.Context, q domain.UserChartQuery) (domain.UserChart, error) {
	firstStart, lastStart, _, unit := userChartWindow(q, time.Now().UTC())
	interval := "1 hour"
	if unit == domain.UserChartSpanDay {
		interval = "1 day"
	}
	var rows []userChartRow
	err := r.db.WithContext(ctx).Raw(`
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
  FROM users u
  CROSS JOIN params p
  WHERE u.created_at < p.first_start
    AND (u.deleted_at IS NULL OR u.deleted_at >= p.first_start)
),
increments AS (
  SELECT
    (date_trunc(?, u.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS value
  FROM users u
  CROSS JOIN params p
  WHERE u.created_at >= p.first_start
    AND u.created_at < p.last_start + p.step
  GROUP BY 1
),
decrements AS (
  SELECT
    (date_trunc(?, u.deleted_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS value
  FROM users u
  CROSS JOIN params p
  WHERE u.deleted_at >= p.first_start
    AND u.deleted_at < p.last_start + p.step
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
`, firstStart, lastStart, interval, unit, unit).Scan(&rows).Error
	if err != nil {
		return domain.UserChart{}, err
	}

	local := domain.UserChartSeries{
		Total: make([]int64, 0, len(rows)),
		Inc:   make([]int64, 0, len(rows)),
		Dec:   make([]int64, 0, len(rows)),
	}
	for _, row := range rows {
		local.Total = append(local.Total, row.Total)
		local.Inc = append(local.Inc, row.Inc)
		local.Dec = append(local.Dec, row.Dec)
	}
	remote := domain.UserChartSeries{
		Total: make([]int64, len(rows)),
		Inc:   make([]int64, len(rows)),
		Dec:   make([]int64, len(rows)),
	}
	return domain.UserChart{Local: local, Remote: remote}, nil
}

func userChartWindow(q domain.UserChartQuery, now time.Time) (time.Time, time.Time, time.Duration, string) {
	step := time.Hour
	unit := domain.UserChartSpanHour
	if q.Span == domain.UserChartSpanDay {
		step = 24 * time.Hour
		unit = domain.UserChartSpanDay
	}
	reference := now.UTC()
	if q.Offset != nil {
		reference = time.UnixMilli(*q.Offset).UTC()
	}
	lastStart := reference.Truncate(step)
	if q.Offset != nil && !lastStart.Equal(reference) {
		lastStart = lastStart.Add(step)
	}
	firstStart := lastStart.Add(-time.Duration(q.Limit-1) * step)
	return firstStart, lastStart, step, unit
}
