package persistence

import (
	"context"
	"time"

	domain "file-service/internal/domain/file"
)

type driveChartRow struct {
	TotalCount int64
	TotalSize  float64
	IncCount   int64
	IncSize    float64
	DecCount   int64
	DecSize    float64
}

func (r *PostgresRepository) GetDriveChart(ctx context.Context, query domain.DriveChartQuery) (domain.DriveChart, error) {
	firstStart, lastStart, _, unit := driveChartWindow(query, time.Now().UTC())
	interval := "1 hour"
	if unit == domain.DriveChartSpanDay {
		interval = "1 day"
	}

	rows, err := r.pool.Query(ctx, `
WITH params AS (
  SELECT
    $1::timestamptz AS first_start,
    $2::timestamptz AS last_start,
    $3::interval AS step
),
items AS (
  SELECT created_at, deleted_at AS removed_at, size_bytes
  FROM files
  WHERE $4::bigint = 0 OR owner_user_id = $4
  UNION ALL
  SELECT created_at, archived_at AS removed_at, size_bytes
  FROM attachments
  WHERE $4::bigint = 0 OR owner_id = $4
),
buckets AS (
  SELECT generate_series(p.first_start, p.last_start, p.step) AS bucket_start
  FROM params p
),
initial AS (
  SELECT
    COUNT(*)::bigint AS total_count,
    COALESCE(SUM(i.size_bytes), 0)::bigint AS total_size
  FROM items i
  CROSS JOIN params p
  WHERE i.created_at < p.first_start
    AND (i.removed_at IS NULL OR i.removed_at >= p.first_start)
),
increments AS (
  SELECT
    (date_trunc($5, i.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS item_count,
    COALESCE(SUM(i.size_bytes), 0)::bigint AS item_size
  FROM items i
  CROSS JOIN params p
  WHERE i.created_at >= p.first_start
    AND i.created_at < p.last_start + p.step
  GROUP BY 1
),
decrements AS (
  SELECT
    (date_trunc($5, i.removed_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC') AS bucket_start,
    COUNT(*)::bigint AS item_count,
    COALESCE(SUM(i.size_bytes), 0)::bigint AS item_size
  FROM items i
  CROSS JOIN params p
  WHERE i.removed_at >= p.first_start
    AND i.removed_at < p.last_start + p.step
  GROUP BY 1
)
SELECT
  (
    initial.total_count + SUM(COALESCE(increments.item_count, 0) - COALESCE(decrements.item_count, 0))
      OVER (ORDER BY buckets.bucket_start ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)
  )::bigint AS total_count,
  (
    initial.total_size + SUM(COALESCE(increments.item_size, 0) - COALESCE(decrements.item_size, 0))
      OVER (ORDER BY buckets.bucket_start ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)
  )::double precision / 1000.0 AS total_size,
  COALESCE(increments.item_count, 0)::bigint AS inc_count,
  COALESCE(increments.item_size, 0)::double precision / 1000.0 AS inc_size,
  COALESCE(decrements.item_count, 0)::bigint AS dec_count,
  COALESCE(decrements.item_size, 0)::double precision / 1000.0 AS dec_size
FROM buckets
CROSS JOIN initial
LEFT JOIN increments USING (bucket_start)
LEFT JOIN decrements USING (bucket_start)
ORDER BY buckets.bucket_start DESC
`, firstStart, lastStart, interval, query.OwnerID, unit)
	if err != nil {
		return domain.DriveChart{}, err
	}
	defer rows.Close()

	local := domain.DriveChartSeries{
		TotalCount: make([]int64, 0, query.Limit),
		TotalSize:  make([]float64, 0, query.Limit),
		IncCount:   make([]int64, 0, query.Limit),
		IncSize:    make([]float64, 0, query.Limit),
		DecCount:   make([]int64, 0, query.Limit),
		DecSize:    make([]float64, 0, query.Limit),
	}
	for rows.Next() {
		var row driveChartRow
		if err := rows.Scan(&row.TotalCount, &row.TotalSize, &row.IncCount, &row.IncSize, &row.DecCount, &row.DecSize); err != nil {
			return domain.DriveChart{}, err
		}
		local.TotalCount = append(local.TotalCount, row.TotalCount)
		local.TotalSize = append(local.TotalSize, row.TotalSize)
		local.IncCount = append(local.IncCount, row.IncCount)
		local.IncSize = append(local.IncSize, row.IncSize)
		local.DecCount = append(local.DecCount, row.DecCount)
		local.DecSize = append(local.DecSize, row.DecSize)
	}
	if err := rows.Err(); err != nil {
		return domain.DriveChart{}, err
	}

	length := len(local.TotalCount)
	remote := domain.DriveChartSeries{
		TotalCount: make([]int64, length),
		TotalSize:  make([]float64, length),
		IncCount:   make([]int64, length),
		IncSize:    make([]float64, length),
		DecCount:   make([]int64, length),
		DecSize:    make([]float64, length),
	}
	return domain.DriveChart{Local: local, Remote: remote}, nil
}

func driveChartWindow(query domain.DriveChartQuery, now time.Time) (time.Time, time.Time, time.Duration, string) {
	step := time.Hour
	unit := domain.DriveChartSpanHour
	if query.Span == domain.DriveChartSpanDay {
		step = 24 * time.Hour
		unit = domain.DriveChartSpanDay
	}
	reference := now.UTC()
	if query.Offset != nil {
		reference = time.UnixMilli(*query.Offset).UTC()
	}
	lastStart := reference.Truncate(step)
	if query.Offset != nil && !lastStart.Equal(reference) {
		lastStart = lastStart.Add(step)
	}
	firstStart := lastStart.Add(-time.Duration(query.Limit-1) * step)
	return firstStart, lastStart, step, unit
}

var _ domain.DriveChartRepository = (*PostgresRepository)(nil)
