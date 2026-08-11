package grpc

import (
	"context"

	pb "feed-service/api/proto/feedpb"
	"feed-service/internal/application/feed/command"
	"feed-service/internal/application/feed/query"
	domain "feed-service/internal/domain/feed"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedFeedServiceServer
	cmd *command.Service
	qry *query.Service
}

func NewHandler(qry *query.Service, commandServices ...*command.Service) *Handler {
	h := &Handler{qry: qry}
	if len(commandServices) > 0 {
		h.cmd = commandServices[0]
	}
	return h
}

func (h *Handler) PurgeAccountFeed(ctx context.Context, req *pb.PurgeAccountFeedRequest) (*pb.PurgeAccountFeedResponse, error) {
	if req.GetUserId() <= 0 || req.GetDeletionJobId() <= 0 || req.GetPolicyVersion() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id, deletion_job_id, and policy_version must be greater than zero")
	}
	if h.cmd == nil {
		return nil, status.Error(codes.Unimplemented, "account feed purge is not configured")
	}

	purgedItems, err := h.cmd.PurgeAccountFeed(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
	if err != nil {
		return nil, status.Error(codes.Internal, "purge account feed failed")
	}
	return &pb.PurgeAccountFeedResponse{Completed: true, PurgedItems: purgedItems}, nil
}

func (h *Handler) ListLatest(ctx context.Context, req *pb.ListFeedRequest) (*pb.FeedListResponse, error) {
	items, err := h.qry.ListLatest(ctx, int(req.GetLimit()), int(req.GetOffset()), req.GetAuthorIds())
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

func (h *Handler) ListActive(ctx context.Context, req *pb.ListFeedRequest) (*pb.FeedListResponse, error) {
	items, err := h.qry.ListActive(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}
	return &pb.FeedListResponse{Items: toPbList(items)}, nil
}

func (h *Handler) ListFiltered(ctx context.Context, req *pb.FilteredFeedRequest) (*pb.FeedListResponse, error) {
	items, err := h.qry.ListFiltered(ctx, domain.Filter{
		Limit: int(req.GetLimit()), Offset: int(req.GetOffset()), AuthorIDs: req.GetAuthorIds(), ExcludedAuthorIDs: req.GetExcludedAuthorIds(),
		Keywords: fromPbFilterKeywords(req.GetKeywords()), ExcludeKeywords: fromPbFilterKeywords(req.GetExcludeKeywords()),
		CaseSensitive: req.GetCaseSensitive(), WithFile: req.GetWithFile(), RestrictAuthors: req.GetRestrictAuthors(), SincePublishedAt: req.GetSincePublishedAt(), UntilPublishedAt: req.GetUntilPublishedAt(), SinceID: req.GetSinceId(), UntilID: req.GetUntilId(),
	})
	if err != nil {
		return nil, err
	}
	return &pb.FeedListResponse{Items: toPbList(items)}, nil
}

func fromPbFilterKeywords(groups []*pb.FeedKeywordGroup) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			out = append(out, append([]string(nil), group.GetTerms()...))
		}
	}
	return out
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
		ViewCount:     item.ViewCount,
		CategoryId:    item.CategoryID,
	}
}
