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

func TestGetUserChartMapsOptionalOffsetAndSeries(t *testing.T) {
	zero := int64(0)
	repo := &handlerUserChartRepo{result: domain.UserChart{
		Local: domain.UserChartSeries{
			Total: []int64{3, 2}, Inc: []int64{1, 0}, Dec: []int64{0, 1},
		},
		Remote: domain.UserChartSeries{
			Total: []int64{0, 0}, Inc: []int64{0, 0}, Dec: []int64{0, 0},
		},
	}}
	handler := NewHandler(nil, query.NewService(repo, nil))

	got, err := handler.GetUserChart(context.Background(), &pb.UserChartRequest{
		Span: domain.UserChartSpanHour, Limit: 2, Offset: &zero,
	})
	if err != nil {
		t.Fatalf("GetUserChart() error = %v", err)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 {
		t.Fatalf("repository offset = %v, want explicit zero", repo.query.Offset)
	}
	if got.GetLocal().GetTotal()[0] != 3 || got.GetLocal().GetDec()[1] != 1 || len(got.GetRemote().GetTotal()) != 2 {
		t.Fatalf("response = %+v", got)
	}
}

func TestGetUserChartMapsValidationAndRepositoryAvailability(t *testing.T) {
	handler := NewHandler(nil, query.NewService(&handlerUserChartRepo{}, nil))
	_, err := handler.GetUserChart(context.Background(), &pb.UserChartRequest{Span: "week", Limit: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid span status = %v, want InvalidArgument", status.Code(err))
	}

	handler = NewHandler(nil, query.NewService(handlerRepoWithoutUserChart{}, nil))
	_, err = handler.GetUserChart(context.Background(), &pb.UserChartRequest{Span: "day", Limit: 1})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("missing repository status = %v, want Unavailable", status.Code(err))
	}
}

type handlerUserChartRepo struct {
	domain.Repository
	query  domain.UserChartQuery
	result domain.UserChart
	err    error
}

func (r *handlerUserChartRepo) GetUserChart(_ context.Context, q domain.UserChartQuery) (domain.UserChart, error) {
	r.query = q
	return r.result, r.err
}

type handlerRepoWithoutUserChart struct{ domain.Repository }
