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

func TestActiveUsersChartRepoPostgresIntegration(t *testing.T) {
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
		{AuthorID: 1, CreatedAt: anchor.Add(-2*time.Hour + 5*time.Minute)},
		{AuthorID: 2, CreatedAt: anchor.Add(-time.Hour + 15*time.Minute)},
		{AuthorID: 2, CreatedAt: anchor.Add(10 * time.Minute)},
	}).Error; err != nil {
		t.Fatalf("insert topics: %v", err)
	}
	if err := tx.Table("articles").Create(&[]noteChartEventRow{
		{AuthorID: 1, CreatedAt: anchor.Add(-2*time.Hour + 20*time.Minute)},
		{AuthorID: 3, CreatedAt: anchor.Add(-time.Hour + 25*time.Minute)},
		{AuthorID: 2, CreatedAt: anchor.Add(30 * time.Minute)},
	}).Error; err != nil {
		t.Fatalf("insert articles: %v", err)
	}
	offset := anchor.UnixMilli()
	got, err := NewRepo(tx).GetActiveUsersChart(context.Background(), domain.NoteChartQuery{Span: "hour", Limit: 3, Offset: &offset})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	want := [][]int64{{2}, {2, 3}, {1}}
	for index, bucket := range got.Buckets {
		if !reflect.DeepEqual(bucket.WriterUserIDs, want[index]) {
			t.Fatalf("bucket %d writer ids = %v, want %v", index, bucket.WriterUserIDs, want[index])
		}
	}
}
