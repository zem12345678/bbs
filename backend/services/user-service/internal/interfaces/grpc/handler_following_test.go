package grpc

import (
	"context"
	"testing"
	"time"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	domain "user-service/internal/domain/user"
)

func TestFollowingRPCPreservesOptionalPreferencesAndEdgeCursor(t *testing.T) {
	users := map[int64]*domain.User{
		42: {ID: 42, Username: "alice", Nickname: "Alice", Status: domain.StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		77: {ID: 77, Username: "bob", Nickname: "Bob", Status: domain.StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	repo := &followingHandlerRepo{
		users: users,
		edges: make(map[[2]int64]*domain.Following),
	}
	handler := NewHandler(
		command.NewService(repo, &followingHandlerIDGenerator{next: 900}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil),
		query.NewService(repo, nil),
	)
	includeReplies := true
	created, err := handler.Follow(context.Background(), &pb.FollowRequest{
		FollowerId: 42, FolloweeId: 77, WithReplies: &includeReplies,
	})
	if err != nil || created.GetPending() || repo.created == nil || !repo.created.WithReplies || repo.created.Notify != domain.FollowNotifyNone {
		t.Fatalf("created response=%+v edge=%+v error=%v", created, repo.created, err)
	}

	withoutReplies := false
	normal := "normal"
	updated, err := handler.UpdateFollowing(context.Background(), &pb.UpdateFollowingRequest{
		FollowerId: 42, FolloweeId: 77, WithReplies: &withoutReplies, Notify: &normal,
	})
	if err != nil || updated.GetFollowing().GetWithReplies() || updated.GetFollowing().GetNotify() != normal {
		t.Fatalf("updated response=%+v error=%v", updated, err)
	}
	if repo.lastPatch.WithReplies == nil || *repo.lastPatch.WithReplies || repo.lastPatch.Notify == nil || *repo.lastPatch.Notify != domain.FollowNotifyNormal {
		t.Fatalf("forwarded patch=%+v", repo.lastPatch)
	}

	listed, err := handler.ListFollowingEdges(context.Background(), &pb.ListFollowingEdgesRequest{
		UserId: 42, SinceId: 800, UntilId: 1000, Limit: 7,
	})
	if err != nil || len(listed.GetItems()) != 1 || listed.GetItems()[0].GetFollower().GetId() != 42 || listed.GetItems()[0].GetFollowee().GetId() != 77 {
		t.Fatalf("listed response=%+v error=%v", listed, err)
	}
	if repo.lastQuery != (domain.FollowingQuery{UserID: 42, SinceID: 800, UntilID: 1000, Limit: 7}) {
		t.Fatalf("forwarded query=%+v", repo.lastQuery)
	}
}

type followingHandlerIDGenerator struct{ next int64 }

func (generator *followingHandlerIDGenerator) Generate() int64 {
	generator.next++
	return generator.next
}

type followingHandlerRepo struct {
	domain.Repository
	domain.SafetyRepository
	users     map[int64]*domain.User
	edges     map[[2]int64]*domain.Following
	created   *domain.Following
	lastPatch domain.FollowingPatch
	lastQuery domain.FollowingQuery
}

func (repo *followingHandlerRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	user, exists := repo.users[id]
	if !exists {
		return nil, domain.ErrNotFound
	}
	copy := *user
	return &copy, nil
}

func (repo *followingHandlerRepo) GetSafetyRelation(context.Context, int64, int64) (domain.SafetyRelation, error) {
	return domain.SafetyRelation{}, nil
}

func (repo *followingHandlerRepo) CreateFollowing(_ context.Context, edge *domain.Following) error {
	copy := *edge
	copy.Follower = repo.users[edge.FollowerID]
	copy.Followee = repo.users[edge.FolloweeID]
	repo.created = &copy
	repo.edges[[2]int64{edge.FollowerID, edge.FolloweeID}] = &copy
	return nil
}

func (repo *followingHandlerRepo) GetFollowing(_ context.Context, followerID, followeeID int64) (*domain.Following, error) {
	edge := repo.edges[[2]int64{followerID, followeeID}]
	if edge == nil {
		return nil, domain.ErrNotFollowing
	}
	copy := *edge
	return &copy, nil
}

func (repo *followingHandlerRepo) UpdateFollowing(ctx context.Context, followerID, followeeID int64, patch domain.FollowingPatch) (*domain.Following, error) {
	edge, err := repo.GetFollowing(ctx, followerID, followeeID)
	if err != nil {
		return nil, err
	}
	repo.lastPatch = patch
	if patch.WithReplies != nil {
		edge.WithReplies = *patch.WithReplies
	}
	if patch.Notify != nil {
		edge.Notify = *patch.Notify
	}
	edge.Follower = repo.users[followerID]
	edge.Followee = repo.users[followeeID]
	repo.edges[[2]int64{followerID, followeeID}] = edge
	return edge, nil
}

func (repo *followingHandlerRepo) UpdateAllFollowings(_ context.Context, _ int64, patch domain.FollowingPatch) error {
	repo.lastPatch = patch
	return nil
}

func (repo *followingHandlerRepo) ListFollowerEdges(context.Context, domain.FollowingQuery) ([]*domain.Following, error) {
	return nil, nil
}

func (repo *followingHandlerRepo) ListFollowingEdges(_ context.Context, input domain.FollowingQuery) ([]*domain.Following, error) {
	repo.lastQuery = input
	edge, err := repo.GetFollowing(context.Background(), 42, 77)
	if err != nil {
		return nil, err
	}
	edge.Follower = repo.users[42]
	edge.Followee = repo.users[77]
	return []*domain.Following{edge}, nil
}
