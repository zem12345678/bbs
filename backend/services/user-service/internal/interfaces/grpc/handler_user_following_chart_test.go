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

func TestGetUserFollowingChartMapsRequestAndScopes(t *testing.T) {
	zero := int64(0)
	repo := &handlerUserFollowingChartRepo{result: domain.UserFollowingChart{
		Local: domain.UserFollowingChartScope{
			Followings: domain.UserChartSeries{Total: []int64{3}, Inc: []int64{1}, Dec: []int64{0}},
			Followers:  domain.UserChartSeries{Total: []int64{4}, Inc: []int64{2}, Dec: []int64{1}},
		},
		Remote: domain.UserFollowingChartScope{
			Followings: domain.UserChartSeries{Total: []int64{0}, Inc: []int64{0}, Dec: []int64{0}},
			Followers:  domain.UserChartSeries{Total: []int64{0}, Inc: []int64{0}, Dec: []int64{0}},
		},
	}}
	handler := NewHandler(nil, query.NewService(repo, nil))
	got, err := handler.GetUserFollowingChart(context.Background(), &pb.UserFollowingChartRequest{
		Span: "hour", Limit: 1, Offset: &zero, UserId: 42,
	})
	if err != nil {
		t.Fatalf("GetUserFollowingChart() error = %v", err)
	}
	if repo.query.UserID != 42 || repo.query.Offset == nil || *repo.query.Offset != 0 {
		t.Fatalf("repository query = %+v", repo.query)
	}
	if got.GetLocal().GetFollowings().GetTotal()[0] != 3 || got.GetLocal().GetFollowers().GetDec()[0] != 1 || got.GetRemote().GetFollowers().GetTotal()[0] != 0 {
		t.Fatalf("response = %+v", got)
	}
}

func TestGetUserFollowingChartMapsValidationAndAvailability(t *testing.T) {
	handler := NewHandler(nil, query.NewService(&handlerUserFollowingChartRepo{}, nil))
	_, err := handler.GetUserFollowingChart(context.Background(), &pb.UserFollowingChartRequest{Span: "week", Limit: 1, UserId: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid span status = %v, want InvalidArgument", status.Code(err))
	}
	handler = NewHandler(nil, query.NewService(handlerRepoWithoutUserChart{}, nil))
	_, err = handler.GetUserFollowingChart(context.Background(), &pb.UserFollowingChartRequest{Span: "day", Limit: 1, UserId: 1})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("missing repository status = %v, want Unavailable", status.Code(err))
	}
}

type handlerUserFollowingChartRepo struct {
	domain.Repository
	query  domain.UserFollowingChartQuery
	result domain.UserFollowingChart
}

func (r *handlerUserFollowingChartRepo) GetUserFollowingChart(_ context.Context, q domain.UserFollowingChartQuery) (domain.UserFollowingChart, error) {
	r.query = q
	return r.result, nil
}
