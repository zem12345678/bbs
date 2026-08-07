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

func TestActiveUsersChartRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL active-users chart test")
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
	if err := tx.Exec("CREATE TEMP TABLE users (id BIGINT PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL) ON COMMIT DROP").Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := tx.Exec("CREATE TEMP TABLE user_login_events (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, success BOOLEAN NOT NULL, created_at TIMESTAMPTZ NOT NULL) ON COMMIT DROP").Error; err != nil {
		t.Fatalf("create login events: %v", err)
	}

	anchor := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	users := []activeUsersChartUserRow{
		{ID: 1, CreatedAt: anchor.Add(-2 * 24 * time.Hour)},
		{ID: 2, CreatedAt: anchor.Add(-20 * 24 * time.Hour)},
		{ID: 3, CreatedAt: anchor.Add(-400 * 24 * time.Hour)},
		{ID: 4, CreatedAt: anchor.Add(-40 * 24 * time.Hour)},
	}
	if err := tx.Table("users").Create(&users).Error; err != nil {
		t.Fatalf("insert users: %v", err)
	}
	events := []activeUsersChartLoginRow{
		{ID: 1, UserID: 1, Success: true, CreatedAt: anchor.Add(-2*time.Hour + 5*time.Minute)},
		{ID: 2, UserID: 1, Success: true, CreatedAt: anchor.Add(-2*time.Hour + 20*time.Minute)},
		{ID: 3, UserID: 2, Success: true, CreatedAt: anchor.Add(-2*time.Hour + 30*time.Minute)},
		{ID: 4, UserID: 1, Success: true, CreatedAt: anchor.Add(-time.Hour + 10*time.Minute)},
		{ID: 5, UserID: 3, Success: true, CreatedAt: anchor.Add(-time.Hour + 15*time.Minute)},
		{ID: 6, UserID: 4, Success: true, CreatedAt: anchor.Add(10 * time.Minute)},
		{ID: 7, UserID: 1, Success: false, CreatedAt: anchor.Add(20 * time.Minute)},
	}
	if err := tx.Table("user_login_events").Create(&events).Error; err != nil {
		t.Fatalf("insert login events: %v", err)
	}
	offset := anchor.UnixMilli()
	got, err := NewRepo(tx).GetActiveUsersChart(context.Background(), domain.UserChartQuery{Span: "hour", Limit: 3, Offset: &offset})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	if len(got.Buckets) != 3 {
		t.Fatalf("bucket count = %d", len(got.Buckets))
	}
	assertActiveUsersBucket(t, got.Buckets[0], []int64{4}, []int64{0, 0, 1, 1, 1, 0})
	assertActiveUsersBucket(t, got.Buckets[1], []int64{1, 3}, []int64{1, 1, 1, 1, 1, 1})
	assertActiveUsersBucket(t, got.Buckets[2], []int64{1, 2}, []int64{1, 2, 2, 1, 0, 0})
}

func assertActiveUsersBucket(t *testing.T, got domain.ActiveUsersChartBucket, read []int64, counts []int64) {
	t.Helper()
	actualCounts := []int64{got.RegisteredWithinWeek, got.RegisteredWithinMonth, got.RegisteredWithinYear, got.RegisteredOutsideWeek, got.RegisteredOutsideMonth, got.RegisteredOutsideYear}
	if !reflect.DeepEqual(got.ReadUserIDs, read) || !reflect.DeepEqual(actualCounts, counts) {
		t.Fatalf("bucket = %+v, want read=%v counts=%v", got, read, counts)
	}
}

type activeUsersChartUserRow struct {
	ID        int64
	CreatedAt time.Time
}

type activeUsersChartLoginRow struct {
	ID        int64
	UserID    int64
	Success   bool
	CreatedAt time.Time
}
