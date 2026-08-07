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

func TestUserFollowingChartRepoPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL user-following-chart test")
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
	if err := tx.Exec(`CREATE TEMP TABLE user_follow_lifecycles (
  id BIGSERIAL PRIMARY KEY,
  follower_id BIGINT NOT NULL,
  followee_id BIGINT NOT NULL,
  followed_at TIMESTAMPTZ NOT NULL,
  unfollowed_at TIMESTAMPTZ
) ON COMMIT DROP`).Error; err != nil {
		t.Fatalf("create temporary lifecycle table: %v", err)
	}

	anchor := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	rows := []followChartLifecycleRow{
		{FollowerID: 42, FolloweeID: 100, FollowedAt: anchor.Add(-3 * time.Hour)},
		{FollowerID: 42, FolloweeID: 101, FollowedAt: anchor.Add(-2*time.Hour + 15*time.Minute), UnfollowedAt: followChartTime(anchor.Add(-time.Hour + 10*time.Minute))},
		{FollowerID: 42, FolloweeID: 102, FollowedAt: anchor.Add(20 * time.Minute), UnfollowedAt: followChartTime(anchor.Add(40 * time.Minute))},
		{FollowerID: 200, FolloweeID: 42, FollowedAt: anchor.Add(-3 * time.Hour)},
		{FollowerID: 201, FolloweeID: 42, FollowedAt: anchor.Add(-time.Hour + 15*time.Minute)},
	}
	if err := tx.Table("user_follow_lifecycles").Create(&rows).Error; err != nil {
		t.Fatalf("create lifecycle rows: %v", err)
	}
	offset := anchor.UnixMilli()
	got, err := NewRepo(tx).GetUserFollowingChart(context.Background(), domain.UserFollowingChartQuery{
		Span: domain.UserChartSpanHour, Limit: 3, Offset: &offset, UserID: 42,
	})
	if err != nil {
		t.Fatalf("GetUserFollowingChart() error = %v", err)
	}
	assertUserFollowingSeries(t, got.Local.Followings, []int64{1, 1, 2}, []int64{1, 0, 1}, []int64{1, 1, 0})
	assertUserFollowingSeries(t, got.Local.Followers, []int64{2, 2, 1}, []int64{0, 1, 0}, []int64{0, 0, 0})
	assertUserFollowingSeries(t, got.Remote.Followings, []int64{0, 0, 0}, []int64{0, 0, 0}, []int64{0, 0, 0})
	assertUserFollowingSeries(t, got.Remote.Followers, []int64{0, 0, 0}, []int64{0, 0, 0}, []int64{0, 0, 0})

	maxOffset := domain.MaxUserChartOffsetMillis
	max, err := NewRepo(tx).GetUserFollowingChart(context.Background(), domain.UserFollowingChartQuery{
		Span: domain.UserChartSpanDay, Limit: 1, Offset: &maxOffset, UserID: 42,
	})
	if err != nil {
		t.Fatalf("GetUserFollowingChart(max offset) error = %v", err)
	}
	if len(max.Local.Followings.Total) != 1 || len(max.Remote.Followers.Total) != 1 {
		t.Fatalf("max offset series lengths = %+v", max)
	}
}

func assertUserFollowingSeries(t *testing.T, got domain.UserChartSeries, total, inc, dec []int64) {
	t.Helper()
	if !reflect.DeepEqual(got.Total, total) || !reflect.DeepEqual(got.Inc, inc) || !reflect.DeepEqual(got.Dec, dec) {
		t.Fatalf("series = %+v, want total=%v inc=%v dec=%v", got, total, inc, dec)
	}
}

type followChartLifecycleRow struct {
	FollowerID   int64
	FolloweeID   int64
	FollowedAt   time.Time
	UnfollowedAt *time.Time
}

func followChartTime(value time.Time) *time.Time { return &value }
