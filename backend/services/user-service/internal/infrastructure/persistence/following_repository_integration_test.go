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

func TestFollowingRepositoryPostgresIntegration(t *testing.T) {
	if os.Getenv("BBS_PG_SMOKE") != "1" {
		t.Skip("set BBS_PG_SMOKE=1 to run PostgreSQL following repository test")
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
	makeUser := func(offset int64, name string, approval bool) *domain.User {
		user, err := domain.New(base+offset, domain.RegisterCmd{
			Username: "fr_" + name + "_" + suffix,
			Email:    "fr_" + name + "_" + suffix + "@example.com",
			Password: "password123", Nickname: name,
		}, "hash")
		if err != nil {
			t.Fatalf("new user %s: %v", name, err)
		}
		user.FollowApprovalRequired = approval
		return user
	}
	follower := makeUser(1, "owner", false)
	targets := []*domain.User{
		makeUser(2, "public1", false), makeUser(3, "private", true),
		makeUser(4, "public2", false), makeUser(5, "public3", false),
	}
	userIDs := []int64{follower.ID}
	for _, target := range targets {
		userIDs = append(userIDs, target.ID)
	}
	defer func() {
		_ = db.Exec("DELETE FROM user_follow_requests WHERE requester_id IN ? OR target_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM user_follows WHERE follower_id IN ? OR followee_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM user_follow_lifecycles WHERE follower_id IN ? OR followee_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM user_blocks WHERE actor_id IN ? OR target_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM user_mutes WHERE actor_id IN ? OR target_id IN ?", userIDs, userIDs).Error
		_ = db.Exec("DELETE FROM users WHERE id IN ?", userIDs).Error
	}()
	for _, user := range append([]*domain.User{follower}, targets...) {
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("create %s: %v", user.Username, err)
		}
	}
	birthday := "1985-08-19"
	targets[0].Birthday = &birthday
	if err := repo.UpdateProfile(ctx, targets[0]); err != nil {
		t.Fatalf("set target birthday: %v", err)
	}

	publicID := base + 100
	pending, created, err := repo.FollowOrRequest(ctx, publicID, follower.ID, targets[0].ID, true)
	if err != nil || pending || !created {
		t.Fatalf("public follow pending=%v created=%v error=%v", pending, created, err)
	}
	edge, err := repo.GetFollowing(ctx, follower.ID, targets[0].ID)
	if err != nil || edge.ID != publicID || !edge.WithReplies || edge.Notify != domain.FollowNotifyNone || edge.Follower == nil || edge.Followee == nil {
		t.Fatalf("public edge = %+v, error = %v", edge, err)
	}
	defaultSubscribers, err := repo.ListNoteNotificationSubscribers(ctx, domain.NoteNotificationSubscribersQuery{FolloweeID: targets[0].ID, Limit: 10})
	if err != nil || len(defaultSubscribers) != 0 {
		t.Fatalf("default notification subscribers = %+v, error = %v", defaultSubscribers, err)
	}

	requestID := base + 101
	pending, created, err = repo.FollowOrRequest(ctx, requestID, follower.ID, targets[1].ID, true)
	if err != nil || !pending || !created {
		t.Fatalf("private request pending=%v created=%v error=%v", pending, created, err)
	}
	request, err := repo.GetFollowRequest(ctx, follower.ID, targets[1].ID)
	if err != nil || request.ID != requestID || !request.WithReplies {
		t.Fatalf("private request = %+v, error = %v", request, err)
	}
	approvedID := base + 102
	approved, err := repo.AcceptFollowRequest(ctx, approvedID, follower.ID, targets[1].ID)
	if err != nil || !approved {
		t.Fatalf("approve request approved=%v error=%v", approved, err)
	}
	edge, err = repo.GetFollowing(ctx, follower.ID, targets[1].ID)
	if err != nil || edge.ID != approvedID || !edge.WithReplies || edge.Notify != domain.FollowNotifyNone {
		t.Fatalf("approved edge = %+v, error = %v", edge, err)
	}

	normal := domain.FollowNotifyNormal
	withoutReplies := false
	edge, err = repo.UpdateFollowing(ctx, follower.ID, targets[0].ID, domain.FollowingPatch{WithReplies: &withoutReplies, Notify: &normal})
	if err != nil || edge.WithReplies || edge.Notify != normal {
		t.Fatalf("updated edge = %+v, error = %v", edge, err)
	}
	subscribers, err := repo.ListNoteNotificationSubscribers(ctx, domain.NoteNotificationSubscribersQuery{FolloweeID: targets[0].ID, Limit: 10})
	if err != nil || len(subscribers) != 1 || subscribers[0].EdgeID != publicID || subscribers[0].UserID != follower.ID {
		t.Fatalf("notification subscribers = %+v, error = %v", subscribers, err)
	}
	cutoffSubscribers, err := repo.ListNoteNotificationSubscribers(ctx, domain.NoteNotificationSubscribersQuery{FolloweeID: targets[0].ID, SinceID: publicID, Limit: 10})
	if err != nil || len(cutoffSubscribers) != 0 {
		t.Fatalf("notification subscribers after cursor = %+v, error = %v", cutoffSubscribers, err)
	}
	if err := repo.Mute(ctx, follower.ID, targets[0].ID); err != nil {
		t.Fatalf("mute follower preference: %v", err)
	}
	mutedSubscribers, err := repo.ListNoteNotificationSubscribers(ctx, domain.NoteNotificationSubscribersQuery{FolloweeID: targets[0].ID, Limit: 10})
	if err != nil || len(mutedSubscribers) != 0 {
		t.Fatalf("muted notification subscribers = %+v, error = %v", mutedSubscribers, err)
	}
	if err := repo.Unmute(ctx, follower.ID, targets[0].ID); err != nil {
		t.Fatalf("unmute follower preference: %v", err)
	}
	for index, target := range targets[2:] {
		id := base + 103 + int64(index)
		pending, created, err = repo.FollowOrRequest(ctx, id, follower.ID, target.ID, false)
		if err != nil || pending || !created {
			t.Fatalf("extra public follow %d pending=%v created=%v error=%v", index, pending, created, err)
		}
	}
	withReplies := true
	none := domain.FollowNotifyNone
	if err := repo.UpdateAllFollowings(ctx, follower.ID, domain.FollowingPatch{WithReplies: &withReplies, Notify: &none}); err != nil {
		t.Fatalf("update all: %v", err)
	}
	filtered, err := repo.ListFollowingEdges(ctx, domain.FollowingQuery{UserID: follower.ID, ViewerID: follower.ID, BirthdayMMDD: "08-19", Limit: 10})
	if err != nil || len(filtered) != 1 || filtered[0].FolloweeID != targets[0].ID {
		t.Fatalf("birthday-filtered following = %+v, error = %v", filtered, err)
	}
	follower.FollowingVisibility = domain.UserVisibilityPrivate
	if err := repo.UpdateProfile(ctx, follower); err != nil {
		t.Fatalf("set following visibility: %v", err)
	}
	if _, err := repo.ListFollowingEdges(ctx, domain.FollowingQuery{UserID: follower.ID, ViewerID: 0, Limit: 10}); err != domain.ErrFollowingListForbidden {
		t.Fatalf("anonymous private following error = %v", err)
	}
	follower.FollowingVisibility = domain.UserVisibilityPublic
	if err := repo.UpdateProfile(ctx, follower); err != nil {
		t.Fatalf("restore following visibility: %v", err)
	}

	latest, err := repo.ListFollowingEdges(ctx, domain.FollowingQuery{UserID: follower.ID, Limit: 2})
	if err != nil || len(latest) != 2 || latest[0].ID != base+104 || latest[1].ID != base+103 {
		t.Fatalf("latest ids = %v, error = %v", followingIDs(latest), err)
	}
	older, err := repo.ListFollowingEdges(ctx, domain.FollowingQuery{UserID: follower.ID, UntilID: latest[1].ID, Limit: 10})
	if err != nil || len(older) != 2 || older[0].ID != approvedID || older[1].ID != publicID {
		t.Fatalf("older ids = %v, error = %v", followingIDs(older), err)
	}
	newer, err := repo.ListFollowingEdges(ctx, domain.FollowingQuery{UserID: follower.ID, SinceID: publicID, UntilID: base + 104, Limit: 10})
	if err != nil || len(newer) != 2 || newer[0].ID != base+103 || newer[1].ID != approvedID {
		t.Fatalf("bounded ids = %v, error = %v", followingIDs(newer), err)
	}

	incoming, err := repo.ListFollowerEdges(ctx, domain.FollowingQuery{UserID: targets[0].ID, Limit: 10})
	if err != nil || len(incoming) != 1 || incoming[0].Follower == nil || incoming[0].Followee == nil {
		t.Fatalf("incoming edges = %+v, error = %v", incoming, err)
	}

	requesters := []*domain.User{targets[0], targets[2], targets[3]}
	for index, requester := range requesters {
		pending, created, err = repo.FollowOrRequest(ctx, base+200+int64(index), requester.ID, targets[1].ID, index%2 == 0)
		if err != nil || !pending || !created {
			t.Fatalf("pending request %d pending=%v created=%v error=%v", index, pending, created, err)
		}
	}
	requests, total, err := repo.ListReceivedFollowRequests(ctx, domain.FollowRequestQuery{ActorID: targets[1].ID, Limit: 2})
	if err != nil || total != 3 || len(requests) != 2 || requests[0].ID != base+202 || requests[1].ID != base+201 {
		t.Fatalf("latest request ids = %v total=%d error=%v", followRequestIDs(requests), total, err)
	}
	requests, _, err = repo.ListReceivedFollowRequests(ctx, domain.FollowRequestQuery{ActorID: targets[1].ID, UntilID: base + 201, Limit: 10})
	if err != nil || len(requests) != 1 || requests[0].ID != base+200 {
		t.Fatalf("older request ids = %v error=%v", followRequestIDs(requests), err)
	}
	requests, _, err = repo.ListReceivedFollowRequests(ctx, domain.FollowRequestQuery{ActorID: targets[1].ID, SinceID: base + 200, UntilID: base + 202, Limit: 10})
	if err != nil || len(requests) != 1 || requests[0].ID != base+201 || requests[0].Requester == nil || requests[0].Target != nil {
		t.Fatalf("bounded requests = %+v error=%v", requests, err)
	}
}

func followingIDs(edges []*domain.Following) []int64 {
	ids := make([]int64, 0, len(edges))
	for _, edge := range edges {
		ids = append(ids, edge.ID)
	}
	return ids
}

func followRequestIDs(requests []*domain.FollowRequest) []int64 {
	ids := make([]int64, 0, len(requests))
	for _, request := range requests {
		ids = append(ids, request.ID)
	}
	return ids
}
