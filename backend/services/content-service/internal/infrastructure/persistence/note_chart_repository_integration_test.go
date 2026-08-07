package persistence

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	domain "content-service/internal/domain/article"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNoteChartRepoPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("BBS_CONTENT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("BBS_CONTENT_POSTGRES_TEST_DSN is not set")
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
	for _, table := range []string{"topics", "articles"} {
		if err := tx.Exec("CREATE TEMP TABLE " + table + " (author_id BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL) ON COMMIT DROP").Error; err != nil {
			t.Fatalf("create temporary %s: %v", table, err)
		}
	}

	anchor := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	if err := tx.Table("topics").Create(&[]noteChartEventRow{
		{AuthorID: 42, CreatedAt: anchor.Add(-3 * time.Hour)},
		{AuthorID: 42, CreatedAt: anchor.Add(-2*time.Hour + 15*time.Minute)},
	}).Error; err != nil {
		t.Fatalf("create topic events: %v", err)
	}
	if err := tx.Table("articles").Create(&[]noteChartEventRow{
		{AuthorID: 7, CreatedAt: anchor.Add(-time.Hour + 15*time.Minute)},
		{AuthorID: 42, CreatedAt: anchor.Add(20 * time.Minute)},
	}).Error; err != nil {
		t.Fatalf("create article events: %v", err)
	}
	offset := anchor.UnixMilli()
	repo := NewRepo(tx)
	global, err := repo.GetNoteChart(context.Background(), domain.NoteChartQuery{Span: "hour", Limit: 3, Offset: &offset})
	if err != nil {
		t.Fatalf("GetNoteChart(global) error = %v", err)
	}
	assertContentNoteSeries(t, global.Local, []int64{4, 3, 2}, []int64{1, 1, 1})
	assertContentNoteSeries(t, global.Remote, []int64{0, 0, 0}, []int64{0, 0, 0})
	user, err := repo.GetNoteChart(context.Background(), domain.NoteChartQuery{Span: "hour", Limit: 3, Offset: &offset, UserID: 42})
	if err != nil {
		t.Fatalf("GetNoteChart(user) error = %v", err)
	}
	assertContentNoteSeries(t, user.Local, []int64{3, 2, 2}, []int64{1, 0, 1})
}

func assertContentNoteSeries(t *testing.T, got domain.NoteChartSeries, total, inc []int64) {
	t.Helper()
	zeros := make([]int64, len(total))
	if !reflect.DeepEqual(got.Total, total) || !reflect.DeepEqual(got.Inc, inc) || !reflect.DeepEqual(got.Dec, zeros) ||
		!reflect.DeepEqual(got.Diffs.Normal, inc) || !reflect.DeepEqual(got.Diffs.Reply, zeros) ||
		!reflect.DeepEqual(got.Diffs.Renote, zeros) || !reflect.DeepEqual(got.Diffs.WithFile, zeros) {
		t.Fatalf("series = %+v, want total=%v inc=%v", got, total, inc)
	}
}

type noteChartEventRow struct {
	AuthorID  int64
	CreatedAt time.Time
}
