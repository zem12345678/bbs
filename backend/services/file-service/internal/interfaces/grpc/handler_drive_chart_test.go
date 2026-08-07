package grpc

import (
	"context"
	"reflect"
	"testing"

	pb "file-service/api/proto/filepb"
	app "file-service/internal/application/file"
	domain "file-service/internal/domain/file"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetDriveChartMapsOptionalOffsetAndSeries(t *testing.T) {
	zero := int64(0)
	repository := &handlerDriveChartRepository{result: domain.DriveChart{
		Local: domain.DriveChartSeries{
			TotalCount: []int64{3}, TotalSize: []float64{1.25},
			IncCount: []int64{2}, IncSize: []float64{0.75},
			DecCount: []int64{1}, DecSize: []float64{0.25},
		},
		Remote: domain.DriveChartSeries{
			TotalCount: []int64{0}, TotalSize: []float64{0},
			IncCount: []int64{0}, IncSize: []float64{0},
			DecCount: []int64{0}, DecSize: []float64{0},
		},
	}}
	handler := NewHandler(app.NewService(repository, nil, nil, nil))

	got, err := handler.GetDriveChart(context.Background(), &pb.DriveChartRequest{
		Span: "hour", Limit: 1, Offset: &zero, OwnerId: 9,
	})
	if err != nil {
		t.Fatalf("GetDriveChart() error = %v", err)
	}
	if repository.query.Offset == nil || *repository.query.Offset != 0 || repository.query.OwnerID != 9 {
		t.Fatalf("forwarded query = %+v", repository.query)
	}
	if !reflect.DeepEqual(got.GetLocal().GetTotalCount(), []int64{3}) ||
		!reflect.DeepEqual(got.GetLocal().GetTotalSize(), []float64{1.25}) ||
		!reflect.DeepEqual(got.GetLocal().GetIncSize(), []float64{0.75}) ||
		!reflect.DeepEqual(got.GetLocal().GetDecSize(), []float64{0.25}) ||
		!reflect.DeepEqual(got.GetRemote().GetTotalCount(), []int64{0}) {
		t.Fatalf("GetDriveChart() = %+v", got)
	}
}

func TestGetDriveChartMapsValidationError(t *testing.T) {
	handler := NewHandler(app.NewService(&handlerDriveChartRepository{}, nil, nil, nil))
	_, err := handler.GetDriveChart(context.Background(), &pb.DriveChartRequest{Span: "week", Limit: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("GetDriveChart() code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

type handlerDriveChartRepository struct {
	domain.Repository
	query  domain.DriveChartQuery
	result domain.DriveChart
}

func (r *handlerDriveChartRepository) GetDriveChart(_ context.Context, query domain.DriveChartQuery) (domain.DriveChart, error) {
	r.query = query
	return r.result, nil
}
