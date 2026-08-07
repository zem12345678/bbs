package grpc

import (
	"context"
	"testing"

	pb "comment-service/api/proto/commentpb"
	commentquery "comment-service/internal/application/comment/query"
	domain "comment-service/internal/domain/comment"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetNoteChartMapsRequestAndResponse(t *testing.T) {
	zero := int64(0)
	repo := &handlerNoteChartRepo{result: domain.NoteChart{
		Local:  domain.NoteChartSeries{Total: []int64{3}, Inc: []int64{1}, Dec: []int64{0}, Diffs: domain.NoteChartDiffs{Normal: []int64{0}, Reply: []int64{1}, Renote: []int64{0}, WithFile: []int64{0}}},
		Remote: domain.NoteChartSeries{Total: []int64{0}, Inc: []int64{0}, Dec: []int64{0}, Diffs: domain.NoteChartDiffs{Normal: []int64{0}, Reply: []int64{0}, Renote: []int64{0}, WithFile: []int64{0}}},
	}}
	handler := NewHandler(nil, commentquery.NewService(repo))
	got, err := handler.GetNoteChart(context.Background(), &pb.NoteChartRequest{Span: "hour", Limit: 1, Offset: &zero, UserId: 42})
	if err != nil {
		t.Fatalf("GetNoteChart() error = %v", err)
	}
	if repo.query.UserID != 42 || repo.query.Offset == nil || *repo.query.Offset != 0 || got.GetLocal().GetTotal()[0] != 3 || got.GetLocal().GetDiffs().GetReply()[0] != 1 {
		t.Fatalf("request/response = %+v / %+v", repo.query, got)
	}
}

func TestGetNoteChartMapsValidationAndAvailability(t *testing.T) {
	handler := NewHandler(nil, commentquery.NewService(&handlerNoteChartRepo{}))
	_, err := handler.GetNoteChart(context.Background(), &pb.NoteChartRequest{Span: "week", Limit: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid span status = %v", status.Code(err))
	}
	_, err = NewHandler(nil, nil).GetNoteChart(context.Background(), &pb.NoteChartRequest{Span: "day", Limit: 1})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("missing service status = %v", status.Code(err))
	}
}

type handlerNoteChartRepo struct {
	domain.Repository
	query  domain.NoteChartQuery
	result domain.NoteChart
}

func (r *handlerNoteChartRepo) GetNoteChart(_ context.Context, q domain.NoteChartQuery) (domain.NoteChart, error) {
	r.query = q
	return r.result, nil
}
