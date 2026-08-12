package grpc

import (
	"context"
	"errors"

	pb "comment-service/api/proto/commentpb"
	"comment-service/internal/application/comment/command"
	"comment-service/internal/application/comment/query"
	domain "comment-service/internal/domain/comment"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedCommentServiceServer
	cmd *command.Service
	qry *query.Service
}

func NewHandler(cmd *command.Service, qry *query.Service) *Handler {
	return &Handler{cmd: cmd, qry: qry}
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, domain.ErrAlreadyHidden),
		errors.Is(err, domain.ErrAlreadyVisible),
		errors.Is(err, domain.ErrInvalidStatusChange),
		errors.Is(err, domain.ErrAuthorErased):
		code = codes.FailedPrecondition
	case errors.Is(err, domain.ErrPermissionDenied):
		code = codes.PermissionDenied
	case errors.Is(err, domain.ErrInvalidID),
		errors.Is(err, domain.ErrInvalidEntityType),
		errors.Is(err, domain.ErrInvalidEntityID),
		errors.Is(err, domain.ErrInvalidAuthorID),
		errors.Is(err, domain.ErrContentRequired),
		errors.Is(err, domain.ErrContentTooLong),
		errors.Is(err, domain.ErrInvalidParent),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidUserErasure),
		errors.Is(err, domain.ErrNoteChartSpanInvalid),
		errors.Is(err, domain.ErrNoteChartLimitInvalid),
		errors.Is(err, domain.ErrNoteChartOffsetInvalid),
		errors.Is(err, domain.ErrNoteChartUserInvalid),
		errors.Is(err, domain.ErrConversationLimitInvalid),
		errors.Is(err, domain.ErrConversationOffsetInvalid):
		code = codes.InvalidArgument
	case errors.Is(err, domain.ErrNoteChartRepositoryUnavailable),
		errors.Is(err, domain.ErrActiveUsersChartRepositoryUnavailable):
		code = codes.Unavailable
	}
	return status.Error(code, err.Error())
}

func (h *Handler) GetNoteChart(ctx context.Context, req *pb.NoteChartRequest) (*pb.NoteChartResponse, error) {
	if h == nil || h.qry == nil {
		return nil, status.Error(codes.Unavailable, "note chart service unavailable")
	}
	result, err := h.qry.GetNoteChart(ctx, domain.NoteChartQuery{
		Span: req.GetSpan(), Limit: int(req.GetLimit()), Offset: req.Offset, UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.NoteChartResponse{Local: toPbNoteChartSeries(result.Local), Remote: toPbNoteChartSeries(result.Remote)}, nil
}

func (h *Handler) GetActiveUsersChart(ctx context.Context, req *pb.ActiveUsersChartRequest) (*pb.ActiveUsersChartResponse, error) {
	if h == nil || h.qry == nil {
		return nil, status.Error(codes.Unavailable, "active users chart service unavailable")
	}
	result, err := h.qry.GetActiveUsersChart(ctx, domain.NoteChartQuery{
		Span: req.GetSpan(), Limit: int(req.GetLimit()), Offset: req.Offset,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	buckets := make([]*pb.ActiveUsersChartBucket, 0, len(result.Buckets))
	for _, bucket := range result.Buckets {
		buckets = append(buckets, &pb.ActiveUsersChartBucket{WriterUserIds: bucket.WriterUserIDs})
	}
	return &pb.ActiveUsersChartResponse{Buckets: buckets}, nil
}

func toPbNoteChartSeries(series domain.NoteChartSeries) *pb.NoteChartSeries {
	return &pb.NoteChartSeries{
		Total: series.Total, Inc: series.Inc, Dec: series.Dec,
		Diffs: &pb.NoteChartDiffs{
			Normal: series.Diffs.Normal, Reply: series.Diffs.Reply,
			Renote: series.Diffs.Renote, WithFile: series.Diffs.WithFile,
		},
	}
}

func toPb(c *domain.Comment) *pb.CommentInfo {
	if c == nil {
		return nil
	}
	return &pb.CommentInfo{
		Id:         c.ID,
		EntityType: c.EntityType,
		EntityId:   c.EntityID,
		RootId:     c.RootID,
		ParentId:   c.ParentID,
		AuthorId:   c.AuthorID,
		Content:    c.Content,
		Status:     int32(c.Status),
		ReplyCount: c.ReplyCount,
		LikeCount:  c.LikeCount,
		CreatedAt:  c.CreatedAt.UnixMilli(),
		UpdatedAt:  c.UpdatedAt.UnixMilli(),
	}
}

func toPbList(rows []*domain.Comment) []*pb.CommentInfo {
	out := make([]*pb.CommentInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPb(row))
	}
	return out
}

func (h *Handler) CreateComment(ctx context.Context, req *pb.CreateCommentRequest) (*pb.CommentResponse, error) {
	c, err := h.cmd.Create(ctx, domain.CreateCmd{
		EntityType: req.GetEntityType(),
		EntityID:   req.GetEntityId(),
		ParentID:   req.GetParentId(),
		AuthorID:   req.GetAuthorId(),
		Content:    req.GetContent(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentResponse{Success: true, Message: "ok", Comment: toPb(c)}, nil
}

func (h *Handler) DeleteComment(ctx context.Context, req *pb.DeleteCommentRequest) (*pb.SimpleResponse, error) {
	if _, err := h.cmd.Delete(ctx, req.GetId(), req.GetActorId(), req.GetModerator()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) RestoreComment(ctx context.Context, req *pb.RestoreCommentRequest) (*pb.SimpleResponse, error) {
	if _, err := h.cmd.Restore(ctx, req.GetId(), req.GetActorId(), req.GetModerator()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) GetComment(ctx context.Context, req *pb.GetCommentRequest) (*pb.CommentResponse, error) {
	c, err := h.qry.Get(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentResponse{Success: true, Message: "ok", Comment: toPb(c)}, nil
}

func (h *Handler) GetCommentConversation(ctx context.Context, req *pb.GetCommentConversationRequest) (*pb.CommentListResponse, error) {
	items, err := h.qry.Conversation(ctx, domain.ConversationQuery{
		CommentID: req.GetCommentId(),
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentListResponse{Items: toPbList(items), Total: int64(len(items))}, nil
}

func (h *Handler) ListComments(ctx context.Context, req *pb.ListCommentsRequest) (*pb.CommentListResponse, error) {
	result, err := h.qry.ListByEntity(ctx, domain.ListQuery{
		EntityType: req.GetEntityType(),
		EntityID:   req.GetEntityId(),
		Page:       int(req.GetPage()),
		PageSize:   int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListReplies(ctx context.Context, req *pb.ListRepliesRequest) (*pb.CommentListResponse, error) {
	result, err := h.qry.ListReplies(ctx, domain.ReplyListQuery{
		RootID:   req.GetRootId(),
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) ListRecentComments(ctx context.Context, req *pb.ListRecentCommentsRequest) (*pb.CommentListResponse, error) {
	result, err := h.qry.ListForModeration(ctx, domain.ModerationListQuery{
		EntityType: req.GetEntityType(),
		EntityID:   req.GetEntityId(),
		AuthorID:   req.GetAuthorId(),
		Status:     req.GetStatus(),
		Page:       int(req.GetPage()),
		PageSize:   int(req.GetPageSize()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CommentListResponse{Items: toPbList(result.Items), Total: result.Total}, nil
}

func (h *Handler) RedactAccountComments(ctx context.Context, req *pb.RedactAccountCommentsRequest) (*pb.RedactAccountCommentsResponse, error) {
	redacted, err := h.cmd.RedactAccountComments(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.RedactAccountCommentsResponse{Completed: true, RedactedComments: redacted}, nil
}
