package persistence

import (
	"context"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "file-service/internal/domain/file"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDriveChartPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BBS_FILE_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("BBS_FILE_POSTGRES_TEST_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	repository := NewPostgresRepository(pool)
	if err := repository.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	seed := time.Now().UnixNano()
	ownerID := seed
	objectPrefix := fmt.Sprintf("drive-chart-integration/%d", seed)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM attachments WHERE object_key LIKE $1`, objectPrefix+"/%")
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM files WHERE object_key LIKE $1`, objectPrefix+"/%")
	})

	anchor := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
INSERT INTO files(owner_user_id, biz_type, object_key, original_name, content_type, size_bytes, status, created_at, updated_at, deleted_at)
VALUES
  ($1, 'drive', $2, 'initial.bin', 'application/octet-stream', 900, 'ACTIVE', $3, $3, NULL),
  ($1, 'drive', $4, 'deleted.bin', 'application/octet-stream', 2350, 'DELETED', $5, $6, $6)
`, ownerID, objectPrefix+"/initial.bin", anchor.Add(-3*time.Hour), objectPrefix+"/deleted.bin", anchor.Add(-time.Hour), anchor.Add(10*time.Minute)); err != nil {
		t.Fatalf("insert chart files: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO attachments(topic_id, owner_id, object_key, original_name, content_type, size_bytes, price_credits, status, created_at, updated_at, archived_at)
VALUES
  ($1, $1, $2, 'archived.bin', 'application/octet-stream', 1200, 0, 'ARCHIVED', $3, $4, $4),
  ($1, $1, $5, 'same-bucket.bin', 'application/octet-stream', 455, 0, 'ARCHIVED', $6, $7, $7)
`, ownerID, objectPrefix+"/archived.bin", anchor.Add(-2*time.Hour+15*time.Minute), anchor.Add(-time.Hour+10*time.Minute), objectPrefix+"/same-bucket.bin", anchor.Add(20*time.Minute), anchor.Add(40*time.Minute)); err != nil {
		t.Fatalf("insert chart attachments: %v", err)
	}

	offset := anchor.UnixMilli()
	got, err := repository.GetDriveChart(ctx, domain.DriveChartQuery{
		Span: domain.DriveChartSpanHour, Limit: 3, Offset: &offset, OwnerID: ownerID,
	})
	if err != nil {
		t.Fatalf("GetDriveChart() error = %v", err)
	}
	assertDriveChartCounts(t, got.Local.TotalCount, []int64{1, 2, 2}, "total count")
	assertDriveChartSizes(t, got.Local.TotalSize, []float64{0.9, 3.25, 2.1}, "total size")
	assertDriveChartCounts(t, got.Local.IncCount, []int64{1, 1, 1}, "increment count")
	assertDriveChartSizes(t, got.Local.IncSize, []float64{0.455, 2.35, 1.2}, "increment size")
	assertDriveChartCounts(t, got.Local.DecCount, []int64{2, 1, 0}, "decrement count")
	assertDriveChartSizes(t, got.Local.DecSize, []float64{2.805, 1.2, 0}, "decrement size")
	assertZeroDriveChart(t, got.Remote, 3)

	dayOffset := anchor.Truncate(24 * time.Hour).UnixMilli()
	daily, err := repository.GetDriveChart(ctx, domain.DriveChartQuery{
		Span: domain.DriveChartSpanDay, Limit: 2, Offset: &dayOffset, OwnerID: ownerID,
	})
	if err != nil {
		t.Fatalf("GetDriveChart(day) error = %v", err)
	}
	assertDriveChartCounts(t, daily.Local.TotalCount, []int64{1, 0}, "daily total count")
	assertDriveChartSizes(t, daily.Local.TotalSize, []float64{0.9, 0}, "daily total size")
	assertDriveChartCounts(t, daily.Local.IncCount, []int64{4, 0}, "daily increment count")
	assertDriveChartSizes(t, daily.Local.IncSize, []float64{4.905, 0}, "daily increment size")
	assertDriveChartCounts(t, daily.Local.DecCount, []int64{3, 0}, "daily decrement count")
	assertDriveChartSizes(t, daily.Local.DecSize, []float64{4.005, 0}, "daily decrement size")
	assertZeroDriveChart(t, daily.Remote, 2)

	maxOffset := domain.MaxDriveChartOffsetMillis
	maximum, err := repository.GetDriveChart(ctx, domain.DriveChartQuery{
		Span: domain.DriveChartSpanDay, Limit: 1, Offset: &maxOffset, OwnerID: ownerID,
	})
	if err != nil {
		t.Fatalf("GetDriveChart(max offset) error = %v", err)
	}
	if len(maximum.Local.TotalCount) != 1 || len(maximum.Remote.TotalCount) != 1 {
		t.Fatalf("max offset lengths = local %d remote %d, want 1", len(maximum.Local.TotalCount), len(maximum.Remote.TotalCount))
	}

	global, err := repository.GetDriveChart(ctx, domain.DriveChartQuery{
		Span: domain.DriveChartSpanHour, Limit: 3, Offset: &offset,
	})
	if err != nil {
		t.Fatalf("GetDriveChart(global) error = %v", err)
	}
	if len(global.Local.TotalCount) != 3 {
		t.Fatalf("global chart length = %d, want 3", len(global.Local.TotalCount))
	}
	for index := range got.Local.TotalCount {
		if global.Local.TotalCount[index] < got.Local.TotalCount[index] {
			t.Fatalf("global total count[%d] = %d, below owner count %d", index, global.Local.TotalCount[index], got.Local.TotalCount[index])
		}
	}
	assertZeroDriveChart(t, global.Remote, 3)
}

func assertDriveChartCounts(t *testing.T, got, want []int64, label string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func assertDriveChartSizes(t *testing.T, got, want []float64, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d", label, len(got), len(want))
	}
	for index := range want {
		if math.Abs(got[index]-want[index]) > 0.0000001 {
			t.Fatalf("%s[%d] = %v, want %v", label, index, got[index], want[index])
		}
	}
}

func assertZeroDriveChart(t *testing.T, got domain.DriveChartSeries, length int) {
	t.Helper()
	zeroCounts := make([]int64, length)
	zeroSizes := make([]float64, length)
	if !reflect.DeepEqual(got.TotalCount, zeroCounts) || !reflect.DeepEqual(got.IncCount, zeroCounts) ||
		!reflect.DeepEqual(got.DecCount, zeroCounts) || !reflect.DeepEqual(got.TotalSize, zeroSizes) ||
		!reflect.DeepEqual(got.IncSize, zeroSizes) || !reflect.DeepEqual(got.DecSize, zeroSizes) {
		t.Fatalf("remote chart = %+v, want zero series of length %d", got, length)
	}
}
