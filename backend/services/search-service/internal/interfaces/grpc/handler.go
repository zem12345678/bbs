package grpc

import (
	"context"
	"errors"

	pb "search-service/api/proto/searchpb"
	"search-service/internal/application/search/command"
	"search-service/internal/application/search/query"
	domain "search-service/internal/domain/search"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedSearchServiceServer
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
	case errors.Is(err, domain.ErrInvalidArticleID), errors.Is(err, domain.ErrInvalidUserID), errors.Is(err, domain.ErrInvalidDeletionJobID), errors.Is(err, domain.ErrInvalidPolicyVersion), errors.Is(err, domain.ErrKeywordRequired):
		code = codes.InvalidArgument
	}
	return status.Error(code, err.Error())
}

func toDomainArticle(in *pb.ArticleDocument) domain.ArticleDocument {
	if in == nil {
		return domain.ArticleDocument{}
	}
	return domain.ArticleDocument{
		ID:             in.GetId(),
		Title:          in.GetTitle(),
		Summary:        in.GetSummary(),
		ContentExcerpt: in.GetContentExcerpt(),
		TagIDs:         in.GetTagIds(),
		TagNames:       in.GetTagNames(),
		AuthorID:       in.GetAuthorId(),
		AuthorNickname: in.GetAuthorNickname(),
		Status:         in.GetStatus(),
		ViewCount:      in.GetViewCount(),
		CommentCount:   in.GetCommentCount(),
		LikeCount:      in.GetLikeCount(),
		FavoriteCount:  in.GetFavoriteCount(),
		CreatedAt:      in.GetCreatedAt(),
		UpdatedAt:      in.GetUpdatedAt(),
	}
}

func toDomainTopic(in *pb.TopicDocument) domain.TopicDocument {
	if in == nil {
		return domain.TopicDocument{}
	}
	return domain.TopicDocument{
		ID:             in.GetId(),
		Slug:           in.GetSlug(),
		Type:           in.GetType(),
		Title:          in.GetTitle(),
		ContentExcerpt: in.GetContentExcerpt(),
		TagNames:       in.GetTagNames(),
		AuthorID:       in.GetAuthorId(),
		Status:         in.GetStatus(),
		ViewCount:      in.GetViewCount(),
		CategoryID:     in.GetCategoryId(),
		CommentCount:   in.GetCommentCount(),
		LikeCount:      in.GetLikeCount(),
		FavoriteCount:  in.GetFavoriteCount(),
		CreatedAt:      in.GetCreatedAt(),
		UpdatedAt:      in.GetUpdatedAt(),
	}
}

func toDomainUser(in *pb.UserDocument) domain.UserDocument {
	if in == nil {
		return domain.UserDocument{}
	}
	return domain.UserDocument{
		ID:        in.GetId(),
		Username:  in.GetUsername(),
		Nickname:  in.GetNickname(),
		Status:    in.GetStatus(),
		CreatedAt: in.GetCreatedAt(),
		UpdatedAt: in.GetUpdatedAt(),
	}
}

func toPbArticle(in domain.ArticleDocument) *pb.ArticleDocument {
	return &pb.ArticleDocument{
		Id:             in.ID,
		Title:          in.Title,
		Summary:        in.Summary,
		ContentExcerpt: in.ContentExcerpt,
		TagIds:         in.TagIDs,
		TagNames:       in.TagNames,
		AuthorId:       in.AuthorID,
		AuthorNickname: in.AuthorNickname,
		Status:         in.Status,
		ViewCount:      in.ViewCount,
		CommentCount:   in.CommentCount,
		LikeCount:      in.LikeCount,
		FavoriteCount:  in.FavoriteCount,
		CreatedAt:      in.CreatedAt,
		UpdatedAt:      in.UpdatedAt,
	}
}

func toPbTopic(in domain.TopicDocument) *pb.TopicDocument {
	return &pb.TopicDocument{
		Id:             in.ID,
		Slug:           in.Slug,
		Type:           in.Type,
		Title:          in.Title,
		ContentExcerpt: in.ContentExcerpt,
		TagNames:       in.TagNames,
		AuthorId:       in.AuthorID,
		Status:         in.Status,
		ViewCount:      in.ViewCount,
		CategoryId:     in.CategoryID,
		CommentCount:   in.CommentCount,
		LikeCount:      in.LikeCount,
		FavoriteCount:  in.FavoriteCount,
		CreatedAt:      in.CreatedAt,
		UpdatedAt:      in.UpdatedAt,
	}
}

func toPbUser(in domain.UserDocument) *pb.UserDocument {
	return &pb.UserDocument{
		Id:        in.ID,
		Username:  in.Username,
		Nickname:  in.Nickname,
		Status:    in.Status,
		CreatedAt: in.CreatedAt,
		UpdatedAt: in.UpdatedAt,
	}
}

func toPbHighlight(in domain.SearchHighlight) *pb.SearchHighlight {
	if len(in.Title) == 0 && len(in.Summary) == 0 && len(in.ContentExcerpt) == 0 && len(in.TagNames) == 0 {
		return nil
	}
	return &pb.SearchHighlight{
		Title:          in.Title,
		Summary:        in.Summary,
		ContentExcerpt: in.ContentExcerpt,
		TagNames:       in.TagNames,
	}
}

func (h *Handler) EnsureArticleIndex(ctx context.Context, _ *pb.EnsureArticleIndexRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.EnsureArticleIndex(ctx); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) EnsureTopicIndex(ctx context.Context, _ *pb.EnsureTopicIndexRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.EnsureTopicIndex(ctx); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) EnsureUserIndex(ctx context.Context, _ *pb.EnsureUserIndexRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.EnsureUserIndex(ctx); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) IndexArticle(ctx context.Context, req *pb.IndexArticleRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.IndexArticle(ctx, toDomainArticle(req.GetArticle())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) IndexTopic(ctx context.Context, req *pb.IndexTopicRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.IndexTopic(ctx, toDomainTopic(req.GetTopic())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) IndexUser(ctx context.Context, req *pb.IndexUserRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.IndexUser(ctx, toDomainUser(req.GetUser())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ReindexArticle(ctx context.Context, req *pb.IndexArticleRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.ReindexArticle(ctx, toDomainArticle(req.GetArticle())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ReindexTopic(ctx context.Context, req *pb.IndexTopicRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.ReindexTopic(ctx, toDomainTopic(req.GetTopic())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) ReindexUser(ctx context.Context, req *pb.IndexUserRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.ReindexUser(ctx, toDomainUser(req.GetUser())); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) DeleteArticle(ctx context.Context, req *pb.DeleteArticleRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DeleteArticle(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) DeleteTopic(ctx context.Context, req *pb.DeleteTopicRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DeleteTopic(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.SimpleResponse, error) {
	if err := h.cmd.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.SimpleResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) EraseUserData(ctx context.Context, req *pb.EraseUserDataRequest) (*pb.EraseUserDataResponse, error) {
	if err := h.cmd.EraseUserData(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.EraseUserDataResponse{Completed: true}, nil
}

func (h *Handler) SearchArticles(ctx context.Context, req *pb.SearchArticlesRequest) (*pb.SearchArticlesResponse, error) {
	result, err := h.qry.SearchArticles(ctx, req.GetKeyword(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.ArticleHit, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &pb.ArticleHit{Article: toPbArticle(item.Document), Score: item.Score, Highlight: toPbHighlight(item.Highlight)})
	}
	return &pb.SearchArticlesResponse{Items: items, Total: result.Total}, nil
}

func (h *Handler) SearchTopics(ctx context.Context, req *pb.SearchTopicsRequest) (*pb.SearchTopicsResponse, error) {
	result, err := h.qry.SearchTopics(ctx, req.GetKeyword(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.TopicHit, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &pb.TopicHit{Topic: toPbTopic(item.Document), Score: item.Score, Highlight: toPbHighlight(item.Highlight)})
	}
	return &pb.SearchTopicsResponse{Items: items, Total: result.Total}, nil
}

func (h *Handler) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	result, err := h.qry.SearchUsers(ctx, req.GetKeyword(), req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, toStatus(err)
	}
	items := make([]*pb.UserHit, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, &pb.UserHit{User: toPbUser(item.Document), Score: item.Score})
	}
	return &pb.SearchUsersResponse{Items: items, Total: result.Total}, nil
}
