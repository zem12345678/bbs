package persistence

import (
	"context"
	"os"
	"strings"
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
		_ = db.Exec("DELETE FROM user_follow_lifecycles WHERE follower_id IN (?, ?) OR followee_id IN (?, ?)", alice.ID, bob.ID, alice.ID, bob.ID).Error
		_ = db.Exec("DELETE FROM users WHERE id IN (?, ?)", alice.ID, bob.ID).Error
	}()

	if err := repo.Create(ctx, alice); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := repo.Create(ctx, bob); err != nil {
		t.Fatalf("create bob: %v", err)
	}
	exactUsers, exactTotal, err := repo.ListUsers(ctx, domain.UserListQuery{
		Usernames: []string{strings.ToUpper(bob.Username), "missing_user"},
		Status:    int32(domain.StatusActive), Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list users by exact usernames: %v", err)
	}
	if exactTotal != 1 || len(exactUsers) != 1 || exactUsers[0].ID != bob.ID {
		t.Fatalf("exact username users=%v total=%d, want bob only", exactUsers, exactTotal)
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
	var closed, open int64
	if err := db.Table("user_follow_lifecycles").Where("follower_id = ? AND followee_id = ? AND unfollowed_at IS NOT NULL", alice.ID, bob.ID).Count(&closed).Error; err != nil {
		t.Fatalf("count closed lifecycle: %v", err)
	}
	if closed != 1 {
		t.Fatalf("closed lifecycle count = %d, want 1", closed)
	}
	if err := repo.Follow(ctx, alice.ID, bob.ID); err != nil {
		t.Fatalf("refollow: %v", err)
	}
	if err := db.Table("user_follow_lifecycles").Where("follower_id = ? AND followee_id = ? AND unfollowed_at IS NULL", alice.ID, bob.ID).Count(&open).Error; err != nil {
		t.Fatalf("count open lifecycle: %v", err)
	}
	if open != 1 {
		t.Fatalf("open lifecycle count = %d, want 1", open)
	}
}

func TestRepoPostgresFollowKeyset(t *testing.T) {
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
	suffix := shortSuffix(base)
	makeUser := func(offset int64, label string) *domain.User {
		u, err := domain.New(base+offset, domain.RegisterCmd{
			Username: "fk_" + label + "_" + suffix,
			Email:    "fk_" + label + "_" + suffix + "@example.com",
			Password: "password123",
			Nickname: label,
		}, "hash")
		if err != nil {
			t.Fatalf("new %s: %v", label, err)
		}
		return u
	}
	owner := makeUser(1, "owner")
	targets := []*domain.User{
		makeUser(2, "target1"),
		makeUser(3, "target2"),
		makeUser(4, "target3"),
		makeUser(5, "target4"),
		makeUser(6, "target5"),
	}
	userIDs := []int64{owner.ID}
	for _, target := range targets {
		userIDs = append(userIDs, target.ID)
	}
	defer func() {
		_ = db.Exec("DELETE FROM user_follows WHERE follower_id IN ? OR followee_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM user_follow_lifecycles WHERE follower_id IN ? OR followee_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM users WHERE id IN ?", userIDs).Error
	}()
	for _, u := range append([]*domain.User{owner}, targets...) {
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("create %s: %v", u.Username, err)
		}
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	relations := make([]followPO, 0, len(targets)+2)
	for index, target := range targets {
		relations = append(relations, followPO{
			FollowerID: owner.ID,
			FolloweeID: target.ID,
			CreatedAt:  now.Add(-time.Duration(index+1) * time.Minute),
		})
	}
	// Also exercise the symmetric followers keyset on the shared query type.
	relations = append(relations,
		followPO{FollowerID: targets[1].ID, FolloweeID: owner.ID, CreatedAt: now},
		followPO{FollowerID: targets[3].ID, FolloweeID: owner.ID, CreatedAt: now},
	)
	if err := db.Create(&relations).Error; err != nil {
		t.Fatalf("create follow fixtures: %v", err)
	}

	legacy, total, err := repo.ListFollowing(ctx, domain.FollowListQuery{UserID: owner.ID, Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("legacy list following: %v", err)
	}
	if total != 5 || len(legacy) != 2 || legacy[0].ID != targets[2].ID || legacy[1].ID != targets[3].ID {
		t.Fatalf("legacy page ids = %v total=%d, want [%d %d] total=5", followUserIDs(legacy), total, targets[2].ID, targets[3].ID)
	}

	sameCreatedAt := now.Add(-time.Hour)
	if err := db.Model(&followPO{}).Where("follower_id = ?", owner.ID).Update("created_at", sameCreatedAt).Error; err != nil {
		t.Fatalf("set identical follow timestamps: %v", err)
	}
	first, total, err := repo.ListFollowing(ctx, domain.FollowListQuery{
		UserID: owner.ID, PageSize: 2, AscendingByID: true,
	})
	if err != nil {
		t.Fatalf("first keyset page: %v", err)
	}
	if total != 5 || len(first) != 2 || first[0].ID != targets[0].ID || first[1].ID != targets[1].ID {
		t.Fatalf("first keyset page ids = %v total=%d", followUserIDs(first), total)
	}
	cursor := first[len(first)-1].ID
	if err := db.Where("follower_id = ? AND followee_id = ?", owner.ID, targets[0].ID).Delete(&followPO{}).Error; err != nil {
		t.Fatalf("delete prior-page follow: %v", err)
	}
	second, total, err := repo.ListFollowing(ctx, domain.FollowListQuery{
		UserID: owner.ID, PageSize: 2, AfterID: cursor, AscendingByID: true,
	})
	if err != nil {
		t.Fatalf("second keyset page: %v", err)
	}
	if total != 4 || len(second) != 2 || second[0].ID != targets[2].ID || second[1].ID != targets[3].ID {
		t.Fatalf("second keyset page ids = %v total=%d", followUserIDs(second), total)
	}
	third, _, err := repo.ListFollowing(ctx, domain.FollowListQuery{
		UserID: owner.ID, PageSize: 2, AfterID: second[len(second)-1].ID, AscendingByID: true,
	})
	if err != nil {
		t.Fatalf("third keyset page: %v", err)
	}
	if len(third) != 1 || third[0].ID != targets[4].ID {
		t.Fatalf("third keyset page ids = %v", followUserIDs(third))
	}

	followers, _, err := repo.ListFollowers(ctx, domain.FollowListQuery{
		UserID: owner.ID, PageSize: 2, AfterID: targets[1].ID, AscendingByID: true,
	})
	if err != nil {
		t.Fatalf("followers keyset page: %v", err)
	}
	if len(followers) != 1 || followers[0].ID != targets[3].ID {
		t.Fatalf("followers keyset page ids = %v", followUserIDs(followers))
	}
}

func followUserIDs(users []*domain.User) []int64 {
	ids := make([]int64, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return ids
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
