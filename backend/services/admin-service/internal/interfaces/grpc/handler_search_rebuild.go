package grpc

import (
	"context"

	pb "admin/api/proto/adminpb"
	domain "admin/internal/domain/admin"
)

func (h *Handler) StartSearchRebuild(ctx context.Context, req *pb.SearchRebuildRequest) (*pb.SearchRebuildStatusResponse, error) {
	result, err := h.service.StartSearchRebuild(ctx, toActor(req.GetActor()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SearchRebuildStatusResponse{Success: true, Message: "ok", Status: toPbSearchRebuildStatus(result)}, nil
}

func (h *Handler) GetSearchRebuildStatus(ctx context.Context, req *pb.SearchRebuildStatusRequest) (*pb.SearchRebuildStatusResponse, error) {
	result, err := h.service.GetSearchRebuildStatus(ctx, toActor(req.GetActor()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.SearchRebuildStatusResponse{Success: true, Message: "ok", Status: toPbSearchRebuildStatus(result)}, nil
}

func toPbSearchRebuildStatus(status domain.SearchRebuildStatus) *pb.SearchRebuildStatus {
	return &pb.SearchRebuildStatus{
		JobId:          status.JobID,
		State:          status.State,
		RequestedBy:    status.RequestedBy,
		ArticleTotal:   status.ArticleTotal,
		ArticleIndexed: status.ArticleIndexed,
		TopicTotal:     status.TopicTotal,
		TopicIndexed:   status.TopicIndexed,
		StartedAt:      status.StartedAt,
		CompletedAt:    status.CompletedAt,
		UpdatedAt:      status.UpdatedAt,
		Error:          status.Error,
	}
}
