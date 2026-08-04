package mall

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestEraseUserDataValidatesRequest(t *testing.T) {
	repo := &accountErasureRepositoryStub{}
	service := NewService(repo, nil, time.Minute)

	tests := []struct {
		name          string
		userID        int64
		deletionJobID int64
		policyVersion int32
	}{
		{name: "missing user", deletionJobID: 2, policyVersion: 1},
		{name: "missing deletion job", userID: 1, policyVersion: 1},
		{name: "missing policy version", userID: 1, deletionJobID: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.EraseUserData(context.Background(), test.userID, test.deletionJobID, test.policyVersion)
			if !errors.Is(err, domain.ErrInvalidAccountErasure) {
				t.Fatalf("EraseUserData() error = %v, want ErrInvalidAccountErasure", err)
			}
		})
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repo.calls)
	}
}

func TestEraseUserDataReturnsUnavailableWithoutRepository(t *testing.T) {
	service := NewService(nil, nil, time.Minute)

	_, err := service.EraseUserData(context.Background(), 1, 2, 1)
	if !errors.Is(err, domain.ErrAccountErasureUnavailable) {
		t.Fatalf("EraseUserData() error = %v, want ErrAccountErasureUnavailable", err)
	}
}

func TestEraseUserDataDelegatesToRepository(t *testing.T) {
	want := domain.AccountErasureResult{
		AnonymizedOrders:       1,
		AnonymizedPayments:     2,
		AnonymizedRefunds:      3,
		AnonymizedCouponUsages: 4,
		ClosedOrders:           5,
		FailedPayments:         6,
		ReleasedCouponUsages:   7,
		CanceledRefunds:        8,
		RevokedEntitlements:    9,
		RedactedReviews:        10,
		DeletedAddresses:       11,
		DeletedCartItems:       12,
		DeletedFavorites:       13,
		DeletedCouponClaims:    14,
		SuppressedOutboxEvents: 15,
	}
	repo := &accountErasureRepositoryStub{result: want}
	service := NewService(repo, nil, time.Minute)

	got, err := service.EraseUserData(context.Background(), 11, 22, 3)
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	if got != want {
		t.Fatalf("EraseUserData() = %+v, want %+v", got, want)
	}
	if repo.calls != 1 || repo.userID != 11 || repo.deletionJobID != 22 || repo.policyVersion != 3 {
		t.Fatalf("repository call = calls:%d user:%d job:%d policy:%d", repo.calls, repo.userID, repo.deletionJobID, repo.policyVersion)
	}
}

type accountErasureRepositoryStub struct {
	domain.Repository
	result        domain.AccountErasureResult
	err           error
	calls         int
	userID        int64
	deletionJobID int64
	policyVersion int32
}

func (r *accountErasureRepositoryStub) EraseUserData(_ context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	r.calls++
	r.userID = userID
	r.deletionJobID = deletionJobID
	r.policyVersion = policyVersion
	return r.result, r.err
}
