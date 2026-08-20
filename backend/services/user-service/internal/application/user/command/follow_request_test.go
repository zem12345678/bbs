package command

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

// followRequestMemoryRepo layers pending follow requests on top of memoryRepo so
// the approval flow can be exercised without a database.
type followRequestMemoryRepo struct {
	*memoryRepo
	requests map[[2]int64]*domain.FollowRequest
}

type recordingFollowEventPublisher struct {
	events []domain.DomainEvent
}

func (p *recordingFollowEventPublisher) PublishDomainEvents(_ context.Context, events []domain.DomainEvent) error {
	p.events = append(p.events, events...)
	return nil
}

func newFollowRequestMemoryRepo() *followRequestMemoryRepo {
	return &followRequestMemoryRepo{
		memoryRepo: newMemoryRepo(),
		requests:   map[[2]int64]*domain.FollowRequest{},
	}
}

func (r *followRequestMemoryRepo) FollowOrRequest(ctx context.Context, requestID, requesterID, targetID int64, withReplies bool) (bool, bool, error) {
	requester, requesterOK := r.users[requesterID]
	target, targetOK := r.users[targetID]
	if !requesterOK || !targetOK {
		return false, false, domain.ErrNotFound
	}
	if err := requester.EnsureActive(); err != nil {
		return false, false, err
	}
	if err := target.EnsureActive(); err != nil {
		return false, false, err
	}
	relation, err := r.GetSafetyRelation(ctx, requesterID, targetID)
	if err != nil {
		return false, false, err
	}
	if relation.Blocked || relation.BlockedBy {
		return false, false, domain.ErrFollowBlocked
	}
	key := [2]int64{requesterID, targetID}
	if _, ok := r.follows[key]; ok {
		return false, false, domain.ErrAlreadyFollowing
	}
	if target.FollowApprovalRequired {
		if _, ok := r.requests[key]; ok {
			return true, false, nil
		}
		request, err := domain.NewFollowRequest(requestID, requesterID, targetID, withReplies)
		if err != nil {
			return false, false, err
		}
		r.requests[key] = request
		return true, true, nil
	}
	delete(r.requests, key)
	if err := r.memoryRepo.Follow(ctx, requesterID, targetID); err != nil {
		return false, false, err
	}
	return false, true, nil
}

func (r *followRequestMemoryRepo) CreateFollowRequest(_ context.Context, req *domain.FollowRequest) error {
	key := [2]int64{req.RequesterID, req.TargetID}
	if _, ok := r.requests[key]; ok {
		return domain.ErrFollowRequestAlreadyExists
	}
	clone := *req
	r.requests[key] = &clone
	return nil
}

func (r *followRequestMemoryRepo) DeleteFollowRequest(_ context.Context, requesterID, targetID int64) error {
	key := [2]int64{requesterID, targetID}
	if _, ok := r.requests[key]; !ok {
		return domain.ErrFollowRequestNotFound
	}
	delete(r.requests, key)
	return nil
}

func (r *followRequestMemoryRepo) AcceptFollowRequest(ctx context.Context, _ int64, requesterID, targetID int64) (bool, error) {
	if err := r.DeleteFollowRequest(ctx, requesterID, targetID); err != nil {
		return false, err
	}
	if err := r.memoryRepo.Follow(ctx, requesterID, targetID); err != nil {
		if errors.Is(err, domain.ErrAlreadyFollowing) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *followRequestMemoryRepo) GetFollowRequest(_ context.Context, requesterID, targetID int64) (*domain.FollowRequest, error) {
	req, ok := r.requests[[2]int64{requesterID, targetID}]
	if !ok {
		return nil, domain.ErrFollowRequestNotFound
	}
	return req, nil
}

func (r *followRequestMemoryRepo) ListReceivedFollowRequests(_ context.Context, q domain.FollowRequestQuery) ([]*domain.FollowRequest, int64, error) {
	return r.collect(q.ActorID, false)
}

func (r *followRequestMemoryRepo) ListSentFollowRequests(_ context.Context, q domain.FollowRequestQuery) ([]*domain.FollowRequest, int64, error) {
	return r.collect(q.ActorID, true)
}

func (r *followRequestMemoryRepo) collect(actorID int64, sent bool) ([]*domain.FollowRequest, int64, error) {
	out := make([]*domain.FollowRequest, 0, len(r.requests))
	for key, req := range r.requests {
		owner := key[1]
		if sent {
			owner = key[0]
		}
		if owner == actorID {
			out = append(out, req)
		}
	}
	return out, int64(len(out)), nil
}

func (r *followRequestMemoryRepo) SetFollowApprovalRequired(_ context.Context, userID int64, required bool) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	u.FollowApprovalRequired = required
	return nil
}

func newFollowRequestService(t *testing.T, repo domain.Repository, firstID int64) *Service {
	t.Helper()
	return NewService(repo, &fakeIDGen{next: firstID}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil)
}

func newFollowRequestServiceWithPublisher(t *testing.T, repo domain.Repository, firstID int64, publisher *recordingFollowEventPublisher) *Service {
	t.Helper()
	return NewService(repo, &fakeIDGen{next: firstID}, publisher, nil, "test-secret", time.Hour, 8, nil, nil, nil)
}

func registerFollowPair(t *testing.T, ctx context.Context, svc *Service) (*domain.User, *domain.User) {
	t.Helper()
	requester, _, err := svc.Register(ctx, domain.RegisterCmd{Username: "carol", Email: "carol@example.com", Password: "password1", Nickname: "Carol"})
	if err != nil {
		t.Fatalf("register carol: %v", err)
	}
	target, _, err := svc.Register(ctx, domain.RegisterCmd{Username: "dave", Email: "dave@example.com", Password: "password1", Nickname: "Dave"})
	if err != nil {
		t.Fatalf("register dave: %v", err)
	}
	return requester, target
}

// TestFollowPrivateAccountCreatesPendingRequest is the core guard: a private
// account must not gain a follower until it approves.
func TestFollowPrivateAccountCreatesPendingRequest(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3000)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)

	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}

	pending, err := svc.Follow(ctx, requester.ID, target.ID)
	if err != nil {
		t.Fatalf("follow private account: %v", err)
	}
	if !pending {
		t.Fatal("follow of a private account must report pending")
	}
	if _, ok := repo.follows[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("private account must not gain a follower before approval")
	}
	if _, ok := repo.requests[[2]int64{requester.ID, target.ID}]; !ok {
		t.Fatal("pending follow request was not recorded")
	}
}

// TestFollowPublicAccountStaysImmediate pins that the approval branch does not
// leak into the default path.
func TestFollowPublicAccountStaysImmediate(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3100)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)

	pending, err := svc.Follow(ctx, requester.ID, target.ID)
	if err != nil {
		t.Fatalf("follow public account: %v", err)
	}
	if pending {
		t.Fatal("follow of a public account must not be pending")
	}
	if _, ok := repo.follows[[2]int64{requester.ID, target.ID}]; !ok {
		t.Fatal("public follow was not applied")
	}
	if len(repo.requests) != 0 {
		t.Fatalf("public follow created %d pending requests", len(repo.requests))
	}
}

func TestAcceptFollowRequestAppliesFollow(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	publisher := &recordingFollowEventPublisher{}
	svc := newFollowRequestServiceWithPublisher(t, repo, 3200, publisher)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	publisher.events = nil
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}
	if _, err := svc.Follow(ctx, requester.ID, target.ID); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0].EventName() != "user.follow_requested" {
		t.Fatalf("follow request events = %+v", eventNames(publisher.events))
	}
	publisher.events = nil

	if err := svc.AcceptFollowRequest(ctx, target.ID, requester.ID); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, ok := repo.follows[[2]int64{requester.ID, target.ID}]; !ok {
		t.Fatal("accepting must apply the follow")
	}
	if _, ok := repo.requests[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("accepting must consume the pending request")
	}
	if got := eventNames(publisher.events); len(got) != 2 || got[0] != "user.follow_request_accepted" || got[1] != "user.followed" {
		t.Fatalf("accepted events = %+v", got)
	}
	if err := svc.AcceptFollowRequest(ctx, target.ID, requester.ID); !errors.Is(err, domain.ErrFollowRequestNotFound) {
		t.Fatalf("second accept = %v, want ErrFollowRequestNotFound", err)
	}
}

func TestAcceptStaleFollowRequestDoesNotRepublishFollow(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	publisher := &recordingFollowEventPublisher{}
	svc := newFollowRequestServiceWithPublisher(t, repo, 3250, publisher)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	publisher.events = nil

	request, err := domain.NewFollowRequest(999, requester.ID, target.ID)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	repo.requests[[2]int64{requester.ID, target.ID}] = request
	if err := repo.memoryRepo.Follow(ctx, requester.ID, target.ID); err != nil {
		t.Fatalf("seed live follow: %v", err)
	}

	if err := svc.AcceptFollowRequest(ctx, target.ID, requester.ID); err != nil {
		t.Fatalf("accept stale request: %v", err)
	}
	if _, ok := repo.requests[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("stale request was not consumed")
	}
	if len(publisher.events) != 0 {
		t.Fatalf("stale request published events = %+v", eventNames(publisher.events))
	}
}

func eventNames(events []domain.DomainEvent) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, event.EventName())
	}
	return names
}

func TestRejectFollowRequestDropsRequest(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3300)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}
	if _, err := svc.Follow(ctx, requester.ID, target.ID); err != nil {
		t.Fatalf("follow: %v", err)
	}

	if err := svc.RejectFollowRequest(ctx, target.ID, requester.ID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, ok := repo.requests[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("rejecting must drop the pending request")
	}
	if _, ok := repo.follows[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("rejecting must not apply the follow")
	}
}

func TestCancelFollowRequestWithdrawsOwnRequest(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3400)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}
	if _, err := svc.Follow(ctx, requester.ID, target.ID); err != nil {
		t.Fatalf("follow: %v", err)
	}

	if err := svc.CancelFollowRequest(ctx, requester.ID, target.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, ok := repo.requests[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("cancelling must drop the pending request")
	}
}

// TestFollowPrivateAccountTwiceStaysPending keeps a repeated tap on Follow
// idempotent instead of surfacing a conflict to the caller.
func TestFollowPrivateAccountTwiceStaysPending(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3500)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		pending, err := svc.Follow(ctx, requester.ID, target.ID)
		if err != nil {
			t.Fatalf("follow attempt %d: %v", attempt, err)
		}
		if !pending {
			t.Fatalf("follow attempt %d must be pending", attempt)
		}
	}
	if len(repo.requests) != 1 {
		t.Fatalf("pending requests = %d, want 1", len(repo.requests))
	}
}

func TestFollowAfterPrivateAccountOpensConsumesPendingRequest(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3550)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("enable approval: %v", err)
	}
	if pending, err := svc.Follow(ctx, requester.ID, target.ID); err != nil || !pending {
		t.Fatalf("private follow pending=%v error=%v", pending, err)
	}
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, false); err != nil {
		t.Fatalf("disable approval: %v", err)
	}

	pending, err := svc.Follow(ctx, requester.ID, target.ID)
	if err != nil {
		t.Fatalf("follow after opening account: %v", err)
	}
	if pending {
		t.Fatal("open account follow must be immediate")
	}
	key := [2]int64{requester.ID, target.ID}
	if _, ok := repo.requests[key]; ok {
		t.Fatal("live follow retained a stale pending request")
	}
	if _, ok := repo.follows[key]; !ok {
		t.Fatal("open account follow was not created")
	}
}

// TestFollowRequestRejectsBlockedRequester keeps the safety gate ahead of the
// approval branch: a blocked user must not even queue a request.
func TestFollowRequestRejectsBlockedRequester(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3600)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}
	if err := svc.Block(ctx, target.ID, requester.ID); err != nil {
		t.Fatalf("block requester: %v", err)
	}

	if _, err := svc.Follow(ctx, requester.ID, target.ID); !errors.Is(err, domain.ErrFollowBlocked) {
		t.Fatalf("follow while blocked = %v, want ErrFollowBlocked", err)
	}
	if len(repo.requests) != 0 {
		t.Fatalf("blocked follow created %d pending requests", len(repo.requests))
	}
}

func TestAcceptFollowRequestRechecksBlockState(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3650)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("set approval required: %v", err)
	}
	if _, err := svc.Follow(ctx, requester.ID, target.ID); err != nil {
		t.Fatalf("follow: %v", err)
	}
	if err := svc.Block(ctx, target.ID, requester.ID); err != nil {
		t.Fatalf("block requester: %v", err)
	}

	if err := svc.AcceptFollowRequest(ctx, target.ID, requester.ID); !errors.Is(err, domain.ErrFollowBlocked) {
		t.Fatalf("accept blocked request = %v, want ErrFollowBlocked", err)
	}
	if _, ok := repo.follows[[2]int64{requester.ID, target.ID}]; ok {
		t.Fatal("blocked request must not become a follow")
	}
}

func TestSetFollowApprovalRequiredTogglesFlag(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3700)
	ctx := context.Background()
	_, target := registerFollowPair(t, ctx, svc)

	if err := svc.SetFollowApprovalRequired(ctx, target.ID, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !repo.users[target.ID].FollowApprovalRequired {
		t.Fatal("flag was not enabled")
	}
	if err := svc.SetFollowApprovalRequired(ctx, target.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if repo.users[target.ID].FollowApprovalRequired {
		t.Fatal("flag was not disabled")
	}
}

func TestFollowRejectsAnonymizedAccount(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3800)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	repo.users[target.ID].AccountState = domain.AccountStateAnonymized

	if _, err := svc.Follow(ctx, requester.ID, target.ID); !errors.Is(err, domain.ErrAccountAnonymized) {
		t.Fatalf("follow anonymized account = %v, want ErrAccountAnonymized", err)
	}
	if len(repo.requests) != 0 || len(repo.follows) != 0 {
		t.Fatal("anonymized account gained a follow relationship")
	}
}

func TestAcceptFollowRequestRejectsAnonymizedRequester(t *testing.T) {
	repo := newFollowRequestMemoryRepo()
	svc := newFollowRequestService(t, repo, 3900)
	ctx := context.Background()
	requester, target := registerFollowPair(t, ctx, svc)
	repo.requests[[2]int64{requester.ID, target.ID}] = &domain.FollowRequest{
		ID: 3999, RequesterID: requester.ID, TargetID: target.ID, CreatedAt: time.Now(),
	}
	repo.users[requester.ID].AccountState = domain.AccountStateAnonymized

	if err := svc.AcceptFollowRequest(ctx, target.ID, requester.ID); !errors.Is(err, domain.ErrAccountAnonymized) {
		t.Fatalf("accept anonymized requester = %v, want ErrAccountAnonymized", err)
	}
	if _, ok := repo.requests[[2]int64{requester.ID, target.ID}]; !ok {
		t.Fatal("failed acceptance must leave the pending request untouched")
	}
	if len(repo.follows) != 0 {
		t.Fatal("anonymized requester became a follower")
	}
}
