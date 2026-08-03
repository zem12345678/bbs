package grpc

import (
	"context"
	"testing"

	pb "content-service/api/proto/contentpb"
	accountcommand "content-service/internal/application/account"
	accountDomain "content-service/internal/domain/account"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestArchiveAccountContent(t *testing.T) {
	repo := &handlerErasureRepository{result: accountDomain.ErasureResult{
		ArchivedArticles: 2, ArchivedTopics: 3, DeletedPollBallots: 4,
	}}
	handler := NewHandler(nil, nil, nil, nil, nil, nil, accountcommand.NewService(repo, nil))

	response, err := handler.ArchiveAccountContent(context.Background(), &pb.ArchiveAccountContentRequest{
		UserId: 42, DeletionJobId: 1001, PolicyVersion: 3,
	})
	if err != nil {
		t.Fatalf("ArchiveAccountContent() error = %v", err)
	}
	if !response.GetCompleted() || response.GetArchivedArticles() != 2 || response.GetArchivedTopics() != 3 || response.GetDeletedPollBallots() != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if repo.userID != 42 || repo.jobID != 1001 || repo.policyVersion != 3 {
		t.Fatalf("repository request = user:%d job:%d policy:%d", repo.userID, repo.jobID, repo.policyVersion)
	}
}

func TestArchiveAccountContentRejectsInvalidRequest(t *testing.T) {
	handler := NewHandler(nil, nil, nil, nil, nil, nil, accountcommand.NewService(&handlerErasureRepository{}, nil))
	_, err := handler.ArchiveAccountContent(context.Background(), &pb.ArchiveAccountContentRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want InvalidArgument", status.Code(err))
	}
}

type handlerErasureRepository struct {
	result        accountDomain.ErasureResult
	err           error
	userID        int64
	jobID         int64
	policyVersion int32
}

func (r *handlerErasureRepository) ArchiveAccountContent(_ context.Context, userID, jobID int64, policyVersion int32) (accountDomain.ErasureResult, error) {
	r.userID = userID
	r.jobID = jobID
	r.policyVersion = policyVersion
	return r.result, r.err
}
