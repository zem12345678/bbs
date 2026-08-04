package grpc

import (
	"context"
	"testing"

	"chat-service/api/proto/chatpb"
	accountapp "chat-service/internal/application/account"
	domain "chat-service/internal/domain/chat"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEraseUserDataDelegatesAndReturnsCounts(t *testing.T) {
	repository := &handlerErasureRepository{result: domain.AccountErasureResult{
		RedactedMessages: 2, DeletedMemberships: 3, DeletedGroups: 4,
		TransferredRooms: 5, ClosedRooms: 6, SuppressedOutboxEvents: 7,
	}}
	handler := NewHandler(nil, accountapp.NewService(repository))

	response, err := handler.EraseUserData(t.Context(), &chatpb.EraseUserDataRequest{UserId: 42, DeletionJobId: 9001, PolicyVersion: 3})
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	if !response.GetCompleted() || response.GetRedactedMessages() != 2 || response.GetDeletedMemberships() != 3 ||
		response.GetDeletedGroups() != 4 || response.GetTransferredRooms() != 5 || response.GetClosedRooms() != 6 ||
		response.GetSuppressedOutboxEvents() != 7 {
		t.Fatalf("EraseUserData() response = %+v", response)
	}
	if repository.userID != 42 || repository.jobID != 9001 || repository.policyVersion != 3 {
		t.Fatalf("repository request = user %d job %d policy %d", repository.userID, repository.jobID, repository.policyVersion)
	}
}

func TestEraseUserDataRejectsInvalidRequest(t *testing.T) {
	handler := NewHandler(nil, accountapp.NewService(&handlerErasureRepository{}))
	_, err := handler.EraseUserData(t.Context(), &chatpb.EraseUserDataRequest{UserId: 42, DeletionJobId: 9001})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("EraseUserData() code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

type handlerErasureRepository struct {
	result        domain.AccountErasureResult
	userID        int64
	jobID         int64
	policyVersion int32
}

func (r *handlerErasureRepository) EraseUserData(_ context.Context, userID, jobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	r.userID, r.jobID, r.policyVersion = userID, jobID, policyVersion
	return r.result, nil
}
