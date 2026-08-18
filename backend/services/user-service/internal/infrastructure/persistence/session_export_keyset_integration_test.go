package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestLoginEventExportKeysetPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL login-event export keyset test")
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
	if err := tx.Exec(`CREATE TEMP TABLE user_login_events (
		id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, session_id VARCHAR(128), ip_address VARCHAR(64) NOT NULL,
		user_agent VARCHAR(512) NOT NULL, success BOOLEAN NOT NULL, failure_reason VARCHAR(64) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	) ON COMMIT DROP`).Error; err != nil {
		t.Fatalf("create login events: %v", err)
	}
	base := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	for id := int64(1); id <= 105; id++ {
		row := userLoginEventPO{ID: id, UserID: 42, IPAddress: "127.0.0.1", UserAgent: "integration", Success: true, CreatedAt: base.Add(-time.Duration(id) * time.Minute)}
		if err := tx.Create(&row).Error; err != nil {
			t.Fatalf("insert login event %d: %v", id, err)
		}
	}
	if err := tx.Create(&userLoginEventPO{ID: 106, UserID: 43, IPAddress: "127.0.0.2", UserAgent: "other", Success: true, CreatedAt: base}).Error; err != nil {
		t.Fatalf("insert other login event: %v", err)
	}
	repo := NewRepo(tx)
	first, err := repo.ListLoginEventsAfterID(context.Background(), 42, 0, 100)
	if err != nil || len(first) != 100 || first[0].ID != 1 || first[99].ID != 100 {
		t.Fatalf("first page = %+v, error = %v", first, err)
	}
	if err := tx.Delete(&userLoginEventPO{}, 1).Error; err != nil {
		t.Fatalf("delete first page row: %v", err)
	}
	second, err := repo.ListLoginEventsAfterID(context.Background(), 42, 100, 100)
	if err != nil || len(second) != 5 || second[0].ID != 101 || second[4].ID != 105 {
		t.Fatalf("second page = %+v, error = %v", second, err)
	}
}
