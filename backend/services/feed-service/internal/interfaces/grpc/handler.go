package grpc

import (
	"context"

	pb "feed-service/api/proto/feedpb"
	"feed-service/internal/application/feed/query"
	domain "feed-service/internal/domain/feed"
)

type Handler struct {
	pb.UnimplementedFeedServiceServer
	qry *query.Service
}

func NewHandler(qry *query.Service) *Handler {
	return &Handler{qry: qry}
}

func (h *Handler) ListLatest(ctx context.Context, req *pb.ListFeedRequest) (*pb.FeedListResponse, error) {
	items, err := h.qry.ListLatest(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}
	return &pb.FeedListResponse{Items: toPbList(items)}, nil
}

func (h *Handler) ListHot(ctx context.Context, req *pb.ListFeedRequest) (*pb.FeedListResponse, error) {
	items, err := h.qry.ListHot(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}
	return &pb.FeedListResponse{Items: toPbList(items)}, nil
}

func toPbList(items []domain.Item) []*pb.FeedItem {
	out := make([]*pb.FeedItem, 0, len(items))
	for _, item := range items {
		out = append(out, toPb(item))
	}
	return out
}

func toPb(item domain.Item) *pb.FeedItem {
	return &pb.FeedItem{
		EntityType:    item.EntityType,
		Id:            item.ID,
		Slug:          item.Slug,
		Title:         item.Title,
		Summary:       item.Summary,
		Body:          item.Body,
		CoverUrl:      item.CoverURL,
		Tags:          item.Tags,
		AuthorId:      item.AuthorID,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		PublishedAt:   item.PublishedAt,
		LikeCount:     item.LikeCount,
		FavoriteCount: item.FavoriteCount,
		CommentCount:  item.CommentCount,
		HotScore:      item.HotScore,
	}
}
