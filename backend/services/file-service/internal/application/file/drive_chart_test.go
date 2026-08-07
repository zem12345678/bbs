package file

import (
	"context"
	"errors"
	"reflect"
	"testing"

	domain "file-service/internal/domain/file"
)

func TestGetDriveChartNormalizesAndForwardsQuery(t *testing.T) {
	offset := int64(0)
	repository := &driveChartRepositoryStub{result: domain.DriveChart{
		Local: domain.DriveChartSeries{TotalCount: []int64{2}, TotalSize: []float64{1.25}},
	}}
	service := NewService(repository, nil, nil, nil)

	got, err := service.GetDriveChart(context.Background(), domain.DriveChartQuery{
		Span: " HOUR ", Limit: 2, Offset: &offset, OwnerID: 42,
	})
	if err != nil {
		t.Fatalf("GetDriveChart() error = %v", err)
	}
	if repository.query.Span != domain.DriveChartSpanHour || repository.query.Limit != 2 ||
		repository.query.Offset == nil || *repository.query.Offset != 0 || repository.query.OwnerID != 42 {
		t.Fatalf("forwarded query = %+v", repository.query)
	}
	if !reflect.DeepEqual(got, repository.result) {
		t.Fatalf("GetDriveChart() = %+v, want %+v", got, repository.result)
	}
}

func TestGetDriveChartRejectsInvalidParameters(t *testing.T) {
	negative := int64(-1)
	tooLarge := domain.MaxDriveChartOffsetMillis + 1
	tests := []struct {
		name  string
		query domain.DriveChartQuery
		want  error
	}{
		{name: "span", query: domain.DriveChartQuery{Span: "week", Limit: 1}, want: domain.ErrDriveChartSpanInvalid},
		{name: "zero limit", query: domain.DriveChartQuery{Span: "day"}, want: domain.ErrDriveChartLimitInvalid},
		{name: "large limit", query: domain.DriveChartQuery{Span: "day", Limit: 501}, want: domain.ErrDriveChartLimitInvalid},
		{name: "negative offset", query: domain.DriveChartQuery{Span: "day", Limit: 1, Offset: &negative}, want: domain.ErrDriveChartOffsetInvalid},
		{name: "large offset", query: domain.DriveChartQuery{Span: "day", Limit: 1, Offset: &tooLarge}, want: domain.ErrDriveChartOffsetInvalid},
		{name: "negative owner", query: domain.DriveChartQuery{Span: "day", Limit: 1, OwnerID: -1}, want: domain.ErrDriveChartOwnerInvalid},
	}
	service := NewService(&driveChartRepositoryStub{}, nil, nil, nil)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.GetDriveChart(context.Background(), test.query)
			if !errors.Is(err, test.want) {
				t.Fatalf("GetDriveChart() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestGetDriveChartFailsClosedWithoutRepository(t *testing.T) {
	service := NewService(driveChartBaseRepository{}, nil, nil, nil)
	_, err := service.GetDriveChart(context.Background(), domain.DriveChartQuery{Span: "day", Limit: 1})
	if !errors.Is(err, domain.ErrDriveChartRepositoryUnavailable) {
		t.Fatalf("GetDriveChart() error = %v, want repository unavailable", err)
	}
}

type driveChartBaseRepository struct{ domain.Repository }

type driveChartRepositoryStub struct {
	domain.Repository
	query  domain.DriveChartQuery
	result domain.DriveChart
	err    error
}

func (r *driveChartRepositoryStub) GetDriveChart(_ context.Context, query domain.DriveChartQuery) (domain.DriveChart, error) {
	r.query = query
	return r.result, r.err
}
