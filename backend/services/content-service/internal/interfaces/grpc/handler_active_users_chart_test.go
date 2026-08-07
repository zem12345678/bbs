package grpc

import (
	"context"
	"testing"

	pb "content-service/api/proto/contentpb"
	articlequery "content-service/internal/application/article/query"
	domain "content-service/internal/domain/article"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetActiveUsersChartMapsRequestAndBuckets(t *testing.T) {
	zero := int64(0)
	repo := &handlerActiveUsersChartRepo{result: domain.ActiveUsersChart{Buckets: []domain.ActiveUsersChartBucket{{WriterUserIDs: []int64{1, 2}}}}}
	handler := NewHandler(nil, articlequery.NewService(repo, nil, nil, nil), nil, nil, nil, nil)
	got, err := handler.GetActiveUsersChart(context.Background(), &pb.ActiveUsersChartRequest{Span: "hour", Limit: 1, Offset: &zero})
	if err != nil {
		t.Fatalf("GetActiveUsersChart() error = %v", err)
	}
	if repo.query.Offset == nil || *repo.query.Offset != 0 || len(got.GetBuckets()) != 1 || len(got.GetBuckets()[0].GetWriterUserIds()) != 2 {
		t.Fatalf("request/response = %+v / %+v", repo.query, got)
	}
}

func TestGetActiveUsersChartMapsValidationAndAvailability(t *testing.T) {
	_, err := NewHandler(nil, articlequery.NewService(&handlerActiveUsersChartRepo{}, nil, nil, nil), nil, nil, nil, nil).GetActiveUsersChart(context.Background(), &pb.ActiveUsersChartRequest{Span: "week", Limit: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid span status = %v", status.Code(err))
	}
	_, err = NewHandler(nil, nil, nil, nil, nil, nil).GetActiveUsersChart(context.Background(), &pb.ActiveUsersChartRequest{Span: "day", Limit: 1})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("missing service status = %v", status.Code(err))
	}
}

type handlerActiveUsersChartRepo struct {
	domain.Repository
	query  domain.NoteChartQuery
	result domain.ActiveUsersChart
}

func (r *handlerActiveUsersChartRepo) GetActiveUsersChart(_ context.Context, q domain.NoteChartQuery) (domain.ActiveUsersChart, error) {
	r.query = q
	return r.result, nil
}
