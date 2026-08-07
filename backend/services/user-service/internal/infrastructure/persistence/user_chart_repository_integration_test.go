package persistence

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUserChartRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL user-chart test")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Exec(`CREATE TEMP TABLE users (
  id BIGINT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  deleted_at TIMESTAMPTZ
) ON COMMIT DROP`).Error; err != nil {
		t.Fatalf("create temporary users table: %v", err)
	}

	anchor := time.Date(2026, time.August, 7, 12, 30, 0, 0, time.UTC)
	users := []userChartTimelineRow{
		{ID: 1, CreatedAt: anchor.Add(-3 * time.Hour)},
		{ID: 2, CreatedAt: anchor.Add(-2*time.Hour - 15*time.Minute)},
		{ID: 3, CreatedAt: anchor.Add(-2 * time.Hour), DeletedAt: timePointer(anchor.Add(-time.Hour))},
		{ID: 4, CreatedAt: anchor.Add(-20 * time.Minute), DeletedAt: timePointer(anchor.Add(-10 * time.Minute))},
		{ID: 5, CreatedAt: anchor.Add(-4 * time.Hour), DeletedAt: timePointer(anchor.Add(-3*time.Hour - 31*time.Minute))},
	}
	if err := tx.Table("users").Create(&users).Error; err != nil {
		t.Fatalf("create chart users: %v", err)
	}

	offset := anchor.Truncate(time.Hour).UnixMilli()
	got, err := NewRepo(tx).GetUserChart(context.Background(), domain.UserChartQuery{
		Span: domain.UserChartSpanHour, Limit: 3, Offset: &offset,
	})
	if err != nil {
		t.Fatalf("GetUserChart() error = %v", err)
	}
	assertUserChartSeries(t, got, []int64{2, 2, 3}, []int64{1, 0, 2}, []int64{1, 1, 0})

	if err := tx.Exec("TRUNCATE TABLE users").Error; err != nil {
		t.Fatalf("truncate temporary users table: %v", err)
	}
	dayAnchor := time.Date(2026, time.August, 7, 0, 0, 0, 0, time.UTC)
	users = []userChartTimelineRow{
		{ID: 11, CreatedAt: dayAnchor.Add(-3 * 24 * time.Hour)},
		{ID: 12, CreatedAt: dayAnchor.Add(-2*24*time.Hour + 8*time.Hour)},
		{ID: 13, CreatedAt: dayAnchor.Add(-2*24*time.Hour + 9*time.Hour), DeletedAt: timePointer(dayAnchor.Add(-24*time.Hour + 10*time.Hour))},
		{ID: 14, CreatedAt: dayAnchor.Add(8 * time.Hour), DeletedAt: timePointer(dayAnchor.Add(10 * time.Hour))},
		{ID: 15, CreatedAt: dayAnchor.Add(-4 * 24 * time.Hour), DeletedAt: timePointer(dayAnchor.Add(-3*24*time.Hour - time.Hour))},
	}
	if err := tx.Table("users").Create(&users).Error; err != nil {
		t.Fatalf("create daily chart users: %v", err)
	}
	offset = dayAnchor.UnixMilli()
	got, err = NewRepo(tx).GetUserChart(context.Background(), domain.UserChartQuery{
		Span: domain.UserChartSpanDay, Limit: 3, Offset: &offset,
	})
	if err != nil {
		t.Fatalf("GetUserChart(day) error = %v", err)
	}
	assertUserChartSeries(t, got, []int64{2, 2, 3}, []int64{1, 0, 2}, []int64{1, 1, 0})

	maxOffset := domain.MaxUserChartOffsetMillis
	got, err = NewRepo(tx).GetUserChart(context.Background(), domain.UserChartQuery{
		Span: domain.UserChartSpanDay, Limit: 1, Offset: &maxOffset,
	})
	if err != nil {
		t.Fatalf("GetUserChart(max offset) error = %v", err)
	}
	if len(got.Local.Total) != 1 || len(got.Remote.Total) != 1 {
		t.Fatalf("max offset series lengths = local %d, remote %d", len(got.Local.Total), len(got.Remote.Total))
	}
}

func assertUserChartSeries(t *testing.T, got domain.UserChart, total, inc, dec []int64) {
	t.Helper()
	if !reflect.DeepEqual(got.Local.Total, total) {
		t.Fatalf("local total = %v, want %v", got.Local.Total, total)
	}
	if !reflect.DeepEqual(got.Local.Inc, inc) {
		t.Fatalf("local inc = %v, want %v", got.Local.Inc, inc)
	}
	if !reflect.DeepEqual(got.Local.Dec, dec) {
		t.Fatalf("local dec = %v, want %v", got.Local.Dec, dec)
	}
	if want := make([]int64, len(total)); !reflect.DeepEqual(got.Remote.Total, want) || !reflect.DeepEqual(got.Remote.Inc, want) || !reflect.DeepEqual(got.Remote.Dec, want) {
		t.Fatalf("remote chart = %+v, want zero series", got.Remote)
	}
}

type userChartTimelineRow struct {
	ID        int64
	CreatedAt time.Time
	DeletedAt *time.Time
}

func timePointer(value time.Time) *time.Time { return &value }
