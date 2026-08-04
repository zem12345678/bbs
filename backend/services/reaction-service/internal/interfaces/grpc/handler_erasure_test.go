package grpc

import (
	"context"
	"testing"

	pb "reaction-service/api/proto/reactionpb"
	accountcommand "reaction-service/internal/application/account"
	accountDomain "reaction-service/internal/domain/account"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEraseAccountReactionsDelegatesAndReturnsCounts(t *testing.T) {
	repo := &handlerErasureRepository{result: accountDomain.ErasureResult{
		DeletedLikes: 2, DeletedFavorites: 3, DeletedCollections: 4, AnonymizedReports: 5, AnonymizedHandledReports: 6,
	}}
	handler := NewHandler(nil, nil, accountcommand.NewService(repo, handlerErasureCache{}))

	response, err := handler.EraseAccountReactions(t.Context(), &pb.EraseAccountReactionsRequest{UserId: 42, DeletionJobId: 9001, PolicyVersion: 3})
	if err != nil {
		t.Fatalf("EraseAccountReactions() error = %v", err)
	}
	if !response.GetCompleted() || response.GetDeletedLikes() != 2 || response.GetDeletedFavorites() != 3 || response.GetDeletedCollections() != 4 || response.GetAnonymizedReports() != 5 || response.GetAnonymizedHandledReports() != 6 {
		t.Fatalf("EraseAccountReactions() response = %+v", response)
	}
	if repo.userID != 42 || repo.jobID != 9001 || repo.policyVersion != 3 {
		t.Fatalf("repository request = user %d job %d policy %d", repo.userID, repo.jobID, repo.policyVersion)
	}
}

func TestEraseAccountReactionsRejectsInvalidRequest(t *testing.T) {
	handler := NewHandler(nil, nil, accountcommand.NewService(&handlerErasureRepository{}, handlerErasureCache{}))
	_, err := handler.EraseAccountReactions(t.Context(), &pb.EraseAccountReactionsRequest{UserId: 42, DeletionJobId: 9001})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("EraseAccountReactions() code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

type handlerErasureRepository struct {
	result        accountDomain.ErasureResult
	userID        int64
	jobID         int64
	policyVersion int32
}

func (r *handlerErasureRepository) EraseAccountReactions(_ context.Context, userID, jobID int64, policyVersion int32) (accountDomain.ErasureResult, error) {
	r.userID, r.jobID, r.policyVersion = userID, jobID, policyVersion
	return r.result, nil
}

type handlerErasureCache struct{}

func (handlerErasureCache) TombstoneAccount(context.Context, int64, int64, int32) error { return nil }
func (handlerErasureCache) PurgeAccount(context.Context, int64) error                   { return nil }
