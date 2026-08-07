package grpc

import (
	"context"
	"testing"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/query"
	domain "user-service/internal/domain/user"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetActiveUsersChartMapsRequestAndBuckets(t *testing.T) {
	zero := int64(0)
	repo := &handlerActiveUsersChartRepo{result: domain.ActiveUsersChart{Buckets: []domain.ActiveUsersChartBucket{{
		ReadUserIDs: []int64{1, 2}, RegisteredWithinWeek: 1, RegisteredOutsideWeek: 1,
	}}}}
	handler := NewHandler(nil, query.NewService(repo, nil))
	got, err := handler.GetActiveUsersChart(context.Background(), &pb.ActiveUsersChartRequest{Span: "hour", Limit: 1, Offset: &zero})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 || len(got.GetBuckets()) != 1 || got.GetBuckets()[0].GetRegisteredWithinWeek() != 1 {
		t.Fatalf("request/response = %+v / %+v", repo.query, got)
	}
}

func TestGetActiveUsersChartMapsValidationAndAvailability(t *testing.T) {
	_, err := NewHandler(nil, query.NewService(&handlerActiveUsersChartRepo{}, nil)).GetActiveUsersChart(context.Background(), &pb.ActiveUsersChartRequest{Span: "week", Limit: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid span status = %v", status.Code(err))
	}
	_, err = NewHandler(nil, query.NewService(handlerRepoWithoutUserChart{}, nil)).GetActiveUsersChart(context.Background(), &pb.ActiveUsersChartRequest{Span: "day", Limit: 1})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("missing repository status = %v", status.Code(err))
	}
}

type handlerActiveUsersChartRepo struct {
	domain.Repository
	query  domain.UserChartQuery
	result domain.ActiveUsersChart
}

func (r *handlerActiveUsersChartRepo) GetActiveUsersChart(_ context.Context, q domain.UserChartQuery) (domain.ActiveUsersChart, error) {
	r.query = q
	return r.result, nil
}
