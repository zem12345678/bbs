package persistence

import (
	"context"
	"time"

	domain "content-service/internal/domain/article"
)

type noteChartRow struct {
	Total int64
	Inc   int64
}

func (r *Repo) GetNoteChart(ctx context.Context, q domain.NoteChartQuery) (domain.NoteChart, error) {
	firstStart, lastStart, _, unit := noteChartWindow(q, time.Now().UTC())
	interval := "1 hour"
	if unit == domain.NoteChartSpanDay {
		interval = "1 day"
	}

	var rows []noteChartRow
	err := r.db.WithContext(ctx).Raw(`
WITH params AS (
  SELECT
    CAST(? AS timestamptz) AS first_start,
    CAST(? AS timestamptz) AS last_start,
    CAST(? AS interval) AS step,
    CAST(? AS bigint) AS user_id
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
initial AS (
  SELECT COUNT(*)::bigint AS total
  FROM events e
  CROSS JOIN params p
  WHERE e.created_at < p.first_start
    AND (p.user_id = 0 OR e.author_id = p.user_id)
),
increments AS (
  SELECT
    (date_trunc(?, e.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS value
  FROM events e
  CROSS JOIN params p
  WHERE e.created_at >= p.first_start
    AND e.created_at < p.last_start + p.step
    AND (p.user_id = 0 OR e.author_id = p.user_id)
  GROUP BY 1
)
SELECT
  (initial.total + SUM(COALESCE(increments.value, 0))
    OVER (ORDER BY buckets.bucket_start ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW))::bigint AS total,
  COALESCE(increments.value, 0)::bigint AS inc
FROM buckets
CROSS JOIN initial
LEFT JOIN increments USING (bucket_start)
ORDER BY buckets.bucket_start DESC
`, firstStart, lastStart, interval, q.UserID, unit).Scan(&rows).Error
	if err != nil {
		return domain.NoteChart{}, err
	}

	local := newNoteChartSeries(len(rows))
	for _, row := range rows {
		local.Total = append(local.Total, row.Total)
		local.Inc = append(local.Inc, row.Inc)
		local.Dec = append(local.Dec, 0)
		local.Diffs.Normal = append(local.Diffs.Normal, row.Inc)
		local.Diffs.Reply = append(local.Diffs.Reply, 0)
		local.Diffs.Renote = append(local.Diffs.Renote, 0)
		local.Diffs.WithFile = append(local.Diffs.WithFile, 0)
	}
	return domain.NoteChart{Local: local, Remote: zeroNoteChartSeries(len(rows))}, nil
}

func noteChartWindow(q domain.NoteChartQuery, now time.Time) (time.Time, time.Time, time.Duration, string) {
	step := time.Hour
	unit := domain.NoteChartSpanHour
	if q.Span == domain.NoteChartSpanDay {
		step = 24 * time.Hour
		unit = domain.NoteChartSpanDay
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

func newNoteChartSeries(capacity int) domain.NoteChartSeries {
	return domain.NoteChartSeries{
		Total: make([]int64, 0, capacity),
		Inc:   make([]int64, 0, capacity),
		Dec:   make([]int64, 0, capacity),
		Diffs: domain.NoteChartDiffs{
			Normal: make([]int64, 0, capacity), Reply: make([]int64, 0, capacity),
			Renote: make([]int64, 0, capacity), WithFile: make([]int64, 0, capacity),
		},
	}
}

func zeroNoteChartSeries(length int) domain.NoteChartSeries {
	return domain.NoteChartSeries{
		Total: make([]int64, length), Inc: make([]int64, length), Dec: make([]int64, length),
		Diffs: domain.NoteChartDiffs{
			Normal: make([]int64, length), Reply: make([]int64, length),
			Renote: make([]int64, length), WithFile: make([]int64, length),
		},
	}
}

var _ domain.NoteChartRepository = (*Repo)(nil)
