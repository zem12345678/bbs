package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "mall-service/api/proto/mallpb"
	app "mall-service/internal/application/mall"
	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestEraseUserDataMapsResponse(t *testing.T) {
	repo := &handlerAccountErasureRepositoryStub{result: domain.AccountErasureResult{
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
	}}
	handler := NewHandler(app.NewService(repo, nil, time.Minute))

	got, err := handler.EraseUserData(context.Background(), &pb.EraseUserDataRequest{
		UserId:        11,
		DeletionJobId: 22,
		PolicyVersion: 3,
	})
	if err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	want := &pb.EraseUserDataResponse{
		Completed:              true,
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
	if !proto.Equal(got, want) {
		t.Fatalf("EraseUserData() = %v, want %v", got, want)
	}
	if repo.userID != 11 || repo.deletionJobID != 22 || repo.policyVersion != 3 {
		t.Fatalf("repository request = user:%d job:%d policy:%d", repo.userID, repo.deletionJobID, repo.policyVersion)
	}
}

func TestToStatusErrorMapsAccountErasureErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid", err: domain.ErrInvalidAccountErasure, code: codes.InvalidArgument},
		{name: "unavailable", err: domain.ErrAccountErasureUnavailable, code: codes.Unavailable},
		{name: "erased", err: domain.ErrAccountErased, code: codes.FailedPrecondition},
		{name: "database guard", err: &pgconn.PgError{Code: "P0001", Message: "mall account erased"}, code: codes.FailedPrecondition},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := toStatusError(fmt.Errorf("wrapped: %w", test.err))
			if got := status.Code(err); got != test.code {
				t.Fatalf("status code = %s, want %s", got, test.code)
			}
		})
	}
}

func TestEraseUserDataMapsRepositoryError(t *testing.T) {
	repo := &handlerAccountErasureRepositoryStub{err: domain.ErrAccountErased}
	handler := NewHandler(app.NewService(repo, nil, time.Minute))

	_, err := handler.EraseUserData(context.Background(), &pb.EraseUserDataRequest{UserId: 1, DeletionJobId: 2, PolicyVersion: 1})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("status code = %s, want %s", got, codes.FailedPrecondition)
	}
}

type handlerAccountErasureRepositoryStub struct {
	domain.Repository
	result        domain.AccountErasureResult
	err           error
	userID        int64
	deletionJobID int64
	policyVersion int32
}

func (r *handlerAccountErasureRepositoryStub) EraseUserData(_ context.Context, userID, deletionJobID int64, policyVersion int32) (domain.AccountErasureResult, error) {
	r.userID = userID
	r.deletionJobID = deletionJobID
	r.policyVersion = policyVersion
	return r.result, r.err
}
