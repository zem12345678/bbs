package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRepoPostgresSmoke(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL smoke test")
	}
	dsn := os.Getenv("BBS_USER_PG_DSN")
	if dsn == "" {
		dsn = "postgres://bbs_user_app:local_user_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_user"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()

	base := time.Now().UnixNano()
	alice, err := domain.New(base, domain.RegisterCmd{
		Username: "alice_smoke",
		Email:    "alice_smoke@example.com",
		Password: "password123",
		Nickname: "Alice",
	}, "hash")
	if err != nil {
		t.Fatalf("new alice: %v", err)
	}
	alice.Username = alice.Username + "_" + shortSuffix(base)
	alice.Email = shortSuffix(base) + "_" + alice.Email
	bob, err := domain.New(base+1, domain.RegisterCmd{
		Username: "bob_smoke",
		Email:    "bob_smoke@example.com",
		Password: "password123",
		Nickname: "Bob",
	}, "hash")
	if err != nil {
		t.Fatalf("new bob: %v", err)
	}
	bob.Username = bob.Username + "_" + shortSuffix(base)
	bob.Email = shortSuffix(base) + "_" + bob.Email

	defer func() {
		_ = db.Exec("DELETE FROM user_follows WHERE follower_id IN (?, ?) OR followee_id IN (?, ?)", alice.ID, bob.ID, alice.ID, bob.ID).Error
		_ = db.Exec("DELETE FROM users WHERE id IN (?, ?)", alice.ID, bob.ID).Error
	}()

	if err := repo.Create(ctx, alice); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := repo.Create(ctx, bob); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	if err := repo.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("follow: %v", err)
	}
	ok, err := repo.IsFollowing(ctx, alice.ID, bob.ID)
	if err != nil || !ok {
		t.Fatalf("is following ok=%v err=%v", ok, err)
	}
	following, total, err := repo.ListFollowing(ctx, domain.FollowListQuery{UserID: alice.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list following: %v", err)
	}
	if total != 1 || len(following) != 1 || following[0].ID != bob.ID {
		t.Fatalf("following total=%d len=%d", total, len(following))
	}
	if err := repo.Unfollow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("unfollow: %v", err)
	}
}

func shortSuffix(v int64) string {
	s := "0000000000"
	raw := strconvFormat(v)
	if len(raw) >= 10 {
		return raw[len(raw)-10:]
	}
	return s[:10-len(raw)] + raw
}

func strconvFormat(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
