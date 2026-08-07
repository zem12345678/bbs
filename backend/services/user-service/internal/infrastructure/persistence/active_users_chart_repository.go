package persistence

import (
	"context"
	"encoding/json"
	"time"

	domain "user-service/internal/domain/user"
)

type activeUsersChartRow struct {
	ReadUserIDsJSON        string `gorm:"column:read_user_ids"`
	RegisteredWithinWeek   int64
	RegisteredWithinMonth  int64
	RegisteredWithinYear   int64
	RegisteredOutsideWeek  int64
	RegisteredOutsideMonth int64
	RegisteredOutsideYear  int64
}

func (r *Repo) GetActiveUsersChart(ctx context.Context, q domain.UserChartQuery) (domain.ActiveUsersChart, error) {
	firstStart, lastStart, _, unit := userChartWindow(q, time.Now().UTC())
	interval := "1 hour"
	if unit == domain.UserChartSpanDay {
		interval = "1 day"
	}

	var rows []activeUsersChartRow
	err := r.db.WithContext(ctx).Raw(`
WITH params AS (
  SELECT CAST(? AS timestamptz) AS first_start, CAST(? AS timestamptz) AS last_start, CAST(? AS interval) AS step
),
buckets AS (
  SELECT generate_series(p.first_start, p.last_start, p.step) AS bucket_start
  FROM params p
),
activity AS (
  SELECT
    (date_trunc(?, e.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    e.user_id,
    BOOL_OR(e.created_at - u.created_at < INTERVAL '7 days') AS within_week,
    BOOL_OR(e.created_at - u.created_at < INTERVAL '30 days') AS within_month,
    BOOL_OR(e.created_at - u.created_at < INTERVAL '365 days') AS within_year,
    BOOL_OR(e.created_at - u.created_at > INTERVAL '7 days') AS outside_week,
    BOOL_OR(e.created_at - u.created_at > INTERVAL '30 days') AS outside_month,
    BOOL_OR(e.created_at - u.created_at > INTERVAL '365 days') AS outside_year
  FROM user_login_events e
  JOIN users u ON u.id = e.user_id
  CROSS JOIN params p
  WHERE e.success
    AND e.created_at >= p.first_start
    AND e.created_at < p.last_start + p.step
  GROUP BY 1, e.user_id
)
SELECT
  COALESCE(json_agg(activity.user_id ORDER BY activity.user_id) FILTER (WHERE activity.user_id IS NOT NULL), '[]'::json)::text AS read_user_ids,
  COUNT(*) FILTER (WHERE activity.within_week)::bigint AS registered_within_week,
  COUNT(*) FILTER (WHERE activity.within_month)::bigint AS registered_within_month,
  COUNT(*) FILTER (WHERE activity.within_year)::bigint AS registered_within_year,
  COUNT(*) FILTER (WHERE activity.outside_week)::bigint AS registered_outside_week,
  COUNT(*) FILTER (WHERE activity.outside_month)::bigint AS registered_outside_month,
  COUNT(*) FILTER (WHERE activity.outside_year)::bigint AS registered_outside_year
FROM buckets
LEFT JOIN activity USING (bucket_start)
GROUP BY buckets.bucket_start
ORDER BY buckets.bucket_start DESC
`, firstStart, lastStart, interval, unit).Scan(&rows).Error
	if err != nil {
		return domain.ActiveUsersChart{}, err
	}

	buckets := make([]domain.ActiveUsersChartBucket, 0, len(rows))
	for _, row := range rows {
		var readUserIDs []int64
		if err := json.Unmarshal([]byte(row.ReadUserIDsJSON), &readUserIDs); err != nil {
			return domain.ActiveUsersChart{}, err
		}
		buckets = append(buckets, domain.ActiveUsersChartBucket{
			ReadUserIDs:          readUserIDs,
			RegisteredWithinWeek: row.RegisteredWithinWeek, RegisteredWithinMonth: row.RegisteredWithinMonth,
			RegisteredWithinYear: row.RegisteredWithinYear, RegisteredOutsideWeek: row.RegisteredOutsideWeek,
			RegisteredOutsideMonth: row.RegisteredOutsideMonth, RegisteredOutsideYear: row.RegisteredOutsideYear,
		})
	}
	return domain.ActiveUsersChart{Buckets: buckets}, nil
}

var _ domain.ActiveUsersChartRepository = (*Repo)(nil)
