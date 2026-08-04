package grpc

import (
	"context"
	"errors"

	pb "reaction-service/api/proto/reactionpb"
	accountcommand "reaction-service/internal/application/account"
	"reaction-service/internal/application/reaction/command"
	"reaction-service/internal/application/reaction/query"
	accountDomain "reaction-service/internal/domain/account"
	domain "reaction-service/internal/domain/reaction"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedReactionServiceServer
	cmd   *command.Service
	qry   *query.Service
	erase *accountcommand.Service
}

func NewHandler(cmd *command.Service, qry *query.Service, erase *accountcommand.Service) *Handler {
	return &Handler{cmd: cmd, qry: qry, erase: erase}
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch {
	case errors.Is(err, domain.ErrInvalidEntityType), errors.Is(err, domain.ErrInvalidEntityID), errors.Is(err, domain.ErrInvalidUserID):
		code = codes.InvalidArgument
	case errors.Is(err, domain.ErrInvalidReportID), errors.Is(err, domain.ErrInvalidReportReason), errors.Is(err, domain.ErrInvalidReportStatus), errors.Is(err, domain.ErrInvalidReportNote), errors.Is(err, domain.ErrInvalidReportAction):
		code = codes.InvalidArgument
	case errors.Is(err, domain.ErrInvalidCollectionID), errors.Is(err, domain.ErrInvalidCollectionName), errors.Is(err, domain.ErrInvalidCollectionDescription), errors.Is(err, domain.ErrInvalidCollectionEntityType):
		code = codes.InvalidArgument
	case errors.Is(err, domain.ErrReportNotFound):
		code = codes.NotFound
	case errors.Is(err, domain.ErrCollectionNotFound):
		code = codes.NotFound
	case errors.Is(err, domain.ErrCollectionNameExists):
		code = codes.AlreadyExists
	case errors.Is(err, domain.ErrCollectionRepositoryUnavailable):
		code = codes.Unavailable
	case errors.Is(err, accountDomain.ErrInvalidErasure):
		code = codes.InvalidArgument
	case errors.Is(err, accountDomain.ErrUserErased):
		code = codes.FailedPrecondition
	}
	return status.Error(code, err.Error())
}

func toRef(ref *pb.EntityRef) domain.EntityRef {
	if ref == nil {
		return domain.EntityRef{}
	}
	return domain.EntityRef{Type: domain.EntityType(ref.GetEntityType()), ID: ref.GetEntityId()}
}

func toResponse(result command.Result) *pb.ReactResponse {
	return &pb.ReactResponse{Success: true, Message: "ok", Count: result.Count, Changed: result.Changed}
}

func toReportPb(report *domain.Report) *pb.ReportInfo {
	if report == nil {
		return nil
	}
	var handledAt int64
	if report.HandledAt != nil {
		handledAt = report.HandledAt.UnixMilli()
	}
	return &pb.ReportInfo{
		Id: report.ID,
		Entity: &pb.EntityRef{
			EntityType: string(report.Entity.Type),
			EntityId:   report.Entity.ID,
		},
		ReporterId:   report.ReporterID,
		Reason:       report.Reason,
		Description:  report.Description,
		Status:       int32(report.Status),
		HandledBy:    report.HandledBy,
		HandledAt:    handledAt,
		CreatedAt:    report.CreatedAt.UnixMilli(),
		UpdatedAt:    report.UpdatedAt.UnixMilli(),
		AuditNote:    report.AuditNote,
		TargetAction: report.TargetAction,
	}
}

func toLikePb(like *domain.Like) *pb.LikeInfo {
	if like == nil {
		return nil
	}
	return &pb.LikeInfo{
		Id: like.ID,
		Entity: &pb.EntityRef{
			EntityType: string(like.Entity.Type),
			EntityId:   like.Entity.ID,
		},
		UserId:    like.UserID,
		CreatedAt: like.CreatedAt.UnixMilli(),
		UpdatedAt: like.UpdatedAt.UnixMilli(),
	}
}

func toFavoritePb(favorite *domain.Favorite) *pb.FavoriteInfo {
	if favorite == nil {
		return nil
	}
	return &pb.FavoriteInfo{
		Id: favorite.ID,
		Entity: &pb.EntityRef{
			EntityType: string(favorite.Entity.Type),
			EntityId:   favorite.Entity.ID,
		},
		UserId:    favorite.UserID,
		CreatedAt: favorite.CreatedAt.UnixMilli(),
		UpdatedAt: favorite.UpdatedAt.UnixMilli(),
	}
}

func toCollectionPb(collection *domain.Collection) *pb.CollectionInfo {
	if collection == nil {
		return nil
	}
	return &pb.CollectionInfo{
		Id:          collection.ID,
		UserId:      collection.UserID,
		Name:        collection.Name,
		Description: collection.Description,
		IsPublic:    collection.IsPublic,
		ItemCount:   collection.ItemCount,
		CreatedAt:   collection.CreatedAt.UnixMilli(),
		UpdatedAt:   collection.UpdatedAt.UnixMilli(),
	}
}

func toCollectionItemPb(item *domain.CollectionItem) *pb.CollectionItemInfo {
	if item == nil {
		return nil
	}
	return &pb.CollectionItemInfo{
		Id:           item.ID,
		CollectionId: item.CollectionID,
		Entity: &pb.EntityRef{
			EntityType: string(item.Entity.Type),
			EntityId:   item.Entity.ID,
		},
		CreatedAt: item.CreatedAt.UnixMilli(),
	}
}

func (h *Handler) Like(ctx context.Context, req *pb.ReactRequest) (*pb.ReactResponse, error) {
	result, err := h.cmd.Like(ctx, toRef(req.GetEntity()), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toResponse(result), nil
}

func (h *Handler) Unlike(ctx context.Context, req *pb.ReactRequest) (*pb.ReactResponse, error) {
	result, err := h.cmd.Unlike(ctx, toRef(req.GetEntity()), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toResponse(result), nil
}

func (h *Handler) ListLikes(ctx context.Context, req *pb.ListLikesRequest) (*pb.LikeListResponse, error) {
	rows, total, err := h.qry.ListLikes(ctx, req.GetUserId(), domain.EntityType(req.GetEntityType()), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*pb.LikeInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toLikePb(row))
	}
	return &pb.LikeListResponse{Items: out, Total: total}, nil
}

func (h *Handler) Favorite(ctx context.Context, req *pb.ReactRequest) (*pb.ReactResponse, error) {
	result, err := h.cmd.Favorite(ctx, toRef(req.GetEntity()), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toResponse(result), nil
}

func (h *Handler) Unfavorite(ctx context.Context, req *pb.ReactRequest) (*pb.ReactResponse, error) {
	result, err := h.cmd.Unfavorite(ctx, toRef(req.GetEntity()), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return toResponse(result), nil
}

func (h *Handler) GetCounts(ctx context.Context, req *pb.EntityRequest) (*pb.CountsResponse, error) {
	view, err := h.qry.Count(ctx, toRef(req.GetEntity()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CountsResponse{LikeCount: view.LikeCount, FavoriteCount: view.FavoriteCount}, nil
}

func (h *Handler) ListFavorites(ctx context.Context, req *pb.ListFavoritesRequest) (*pb.FavoriteListResponse, error) {
	rows, total, err := h.qry.ListFavorites(ctx, req.GetUserId(), domain.EntityType(req.GetEntityType()), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*pb.FavoriteInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toFavoritePb(row))
	}
	return &pb.FavoriteListResponse{Items: out, Total: total}, nil
}

func (h *Handler) HotIDs(ctx context.Context, req *pb.HotIDsRequest) (*pb.HotIDsResponse, error) {
	ids, err := h.qry.HotIDs(ctx, domain.EntityType(req.GetEntityType()), int(req.GetLimit()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.HotIDsResponse{Ids: ids}, nil
}

func (h *Handler) SubmitReport(ctx context.Context, req *pb.SubmitReportRequest) (*pb.ReportResponse, error) {
	result, err := h.cmd.SubmitReport(ctx, domain.SubmitReportCmd{
		Entity:      toRef(req.GetEntity()),
		ReporterID:  req.GetReporterId(),
		Reason:      req.GetReason(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ReportResponse{Success: true, Message: "ok", Report: toReportPb(result.Report), Created: result.Created}, nil
}

func (h *Handler) ListReports(ctx context.Context, req *pb.ListReportsRequest) (*pb.ReportListResponse, error) {
	rows, total, err := h.qry.ListReports(ctx, domain.ReportStatus(req.GetStatus()), domain.EntityType(req.GetEntityType()), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	out := make([]*pb.ReportInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toReportPb(row))
	}
	return &pb.ReportListResponse{Items: out, Total: total}, nil
}

func (h *Handler) GetReport(ctx context.Context, req *pb.GetReportRequest) (*pb.ReportResponse, error) {
	report, err := h.qry.GetReport(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ReportResponse{Success: true, Message: "ok", Report: toReportPb(report)}, nil
}

func (h *Handler) AuditReport(ctx context.Context, req *pb.AuditReportRequest) (*pb.ReportResponse, error) {
	report, err := h.cmd.AuditReport(ctx, req.GetId(), domain.ReportStatus(req.GetStatus()), req.GetHandlerId(), req.GetAuditNote(), req.GetTargetAction())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ReportResponse{Success: true, Message: "ok", Report: toReportPb(report)}, nil
}

func (h *Handler) CreateCollection(ctx context.Context, req *pb.CreateCollectionRequest) (*pb.CollectionResponse, error) {
	collection, err := h.cmd.CreateCollection(ctx, req.GetUserId(), req.GetName(), req.GetDescription(), req.GetIsPublic())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CollectionResponse{Success: true, Message: "ok", Collection: toCollectionPb(collection)}, nil
}

func (h *Handler) UpdateCollection(ctx context.Context, req *pb.UpdateCollectionRequest) (*pb.CollectionResponse, error) {
	collection, err := h.cmd.UpdateCollection(ctx, req.GetUserId(), req.GetId(), req.GetName(), req.GetDescription(), req.GetIsPublic())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CollectionResponse{Success: true, Message: "ok", Collection: toCollectionPb(collection)}, nil
}

func (h *Handler) DeleteCollection(ctx context.Context, req *pb.DeleteCollectionRequest) (*pb.CollectionActionResponse, error) {
	if err := h.cmd.DeleteCollection(ctx, req.GetUserId(), req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.CollectionActionResponse{Success: true, Message: "ok", Changed: true}, nil
}

func (h *Handler) ListCollections(ctx context.Context, req *pb.ListCollectionsRequest) (*pb.ListCollectionsResponse, error) {
	rows, total, err := h.qry.ListCollections(ctx, req.GetUserId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.CollectionInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, toCollectionPb(row))
	}
	return &pb.ListCollectionsResponse{Items: items, Total: total}, nil
}

func (h *Handler) AddCollectionItem(ctx context.Context, req *pb.CollectionItemRequest) (*pb.CollectionActionResponse, error) {
	changed, err := h.cmd.AddCollectionItem(ctx, req.GetUserId(), req.GetCollectionId(), toRef(req.GetEntity()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CollectionActionResponse{Success: true, Message: "ok", Changed: changed}, nil
}

func (h *Handler) RemoveCollectionItem(ctx context.Context, req *pb.CollectionItemRequest) (*pb.CollectionActionResponse, error) {
	changed, err := h.cmd.RemoveCollectionItem(ctx, req.GetUserId(), req.GetCollectionId(), toRef(req.GetEntity()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CollectionActionResponse{Success: true, Message: "ok", Changed: changed}, nil
}

func (h *Handler) ListCollectionItems(ctx context.Context, req *pb.ListCollectionItemsRequest) (*pb.CollectionItemsResponse, error) {
	rows, total, err := h.qry.ListCollectionItems(ctx, req.GetUserId(), req.GetCollectionId(), domain.EntityType(req.GetEntityType()), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.CollectionItemInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, toCollectionItemPb(row))
	}
	return &pb.CollectionItemsResponse{Items: items, Total: total}, nil
}

func (h *Handler) EraseAccountReactions(ctx context.Context, req *pb.EraseAccountReactionsRequest) (*pb.EraseAccountReactionsResponse, error) {
	if h.erase == nil {
		return nil, status.Error(codes.Unavailable, "account erasure service unavailable")
	}
	result, err := h.erase.EraseAccountReactions(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.EraseAccountReactionsResponse{
		Completed:                true,
		DeletedLikes:             result.DeletedLikes,
		DeletedFavorites:         result.DeletedFavorites,
		DeletedCollections:       result.DeletedCollections,
		AnonymizedReports:        result.AnonymizedReports,
		AnonymizedHandledReports: result.AnonymizedHandledReports,
	}, nil
}
