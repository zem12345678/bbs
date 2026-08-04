package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domain "chat-service/internal/domain/chat"
)

func TestEraseUserDataValidatesInput(t *testing.T) {
	service := NewService(&erasureRepositoryStub{})
	for _, testCase := range []struct {
		userID        int64
		jobID         int64
		policyVersion int32
	}{
		{userID: 0, jobID: 1, policyVersion: 1},
		{userID: 1, jobID: 0, policyVersion: 1},
		{userID: 1, jobID: 1, policyVersion: 0},
	} {
		if _, err := service.EraseUserData(t.Context(), testCase.userID, testCase.jobID, testCase.policyVersion); !errors.Is(err, domain.ErrInvalidErasure) {
			t.Fatalf("EraseUserData(%d, %d, %d) error = %v, want ErrInvalidErasure", testCase.userID, testCase.jobID, testCase.policyVersion, err)
		}
	}
}

func TestEraseUserDataDelegatesToRepository(t *testing.T) {
	want := domain.AccountErasureResult{
		RedactedMessages: 2, DeletedMemberships: 3, DeletedGroups: 4,
		TransferredRooms: 5, ClosedRooms: 6, SuppressedOutboxEvents: 7,
	}
	repository := &erasureRepositoryStub{result: want}
	service := NewService(repository)

	got, err := service.EraseUserData(t.Context(), 42, 9001, 3)
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EraseUserData() = %+v, want %+v", got, want)
	}
	if repository.userID != 42 || repository.jobID != 9001 || repository.policyVersion != 3 {
		t.Fatalf("repository request = user %d job %d policy %d", repository.userID, repository.jobID, repository.policyVersion)
	}
}

type erasureRepositoryStub struct {
	result        domain.AccountErasureResult
	userID        int64
	jobID         int64
	policyVersion int32
}

func (r *erasureRepositoryStub) EraseUserData(_ context.Context, userID, jobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	r.userID, r.jobID, r.policyVersion = userID, jobID, policyVersion
	return r.result, nil
}
