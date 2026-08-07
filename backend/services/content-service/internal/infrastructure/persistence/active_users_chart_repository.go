package persistence

import (
	"context"
	"encoding/json"
	"time"

	domain "content-service/internal/domain/article"
)

type activeUsersChartRow struct {
	WriterUserIDsJSON string `gorm:"column:writer_user_ids"`
}

func (r *Repo) GetActiveUsersChart(ctx context.Context, q domain.NoteChartQuery) (domain.ActiveUsersChart, error) {
	firstStart, lastStart, _, unit := noteChartWindow(q, time.Now().UTC())
	interval := "1 hour"
	if unit == domain.NoteChartSpanDay {
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
events AS (
  SELECT author_id, created_at FROM topics
  UNION ALL
  SELECT author_id, created_at FROM articles
),
writers AS (
  SELECT
    (date_trunc(?, events.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    events.author_id
  FROM events
  CROSS JOIN params p
  WHERE events.created_at >= p.first_start
    AND events.created_at < p.last_start + p.step
  GROUP BY 1, events.author_id
)
SELECT
  COALESCE(json_agg(writers.author_id ORDER BY writers.author_id) FILTER (WHERE writers.author_id IS NOT NULL), '[]'::json)::text AS writer_user_ids
FROM buckets
LEFT JOIN writers USING (bucket_start)
GROUP BY buckets.bucket_start
ORDER BY buckets.bucket_start DESC
`, firstStart, lastStart, interval, unit).Scan(&rows).Error
	if err != nil {
		return domain.ActiveUsersChart{}, err
	}
	buckets := make([]domain.ActiveUsersChartBucket, 0, len(rows))
	for _, row := range rows {
		var writerUserIDs []int64
		if err := json.Unmarshal([]byte(row.WriterUserIDsJSON), &writerUserIDs); err != nil {
			return domain.ActiveUsersChart{}, err
		}
		buckets = append(buckets, domain.ActiveUsersChartBucket{WriterUserIDs: writerUserIDs})
	}
	return domain.ActiveUsersChart{Buckets: buckets}, nil
}

var _ domain.ActiveUsersChartRepository = (*Repo)(nil)
