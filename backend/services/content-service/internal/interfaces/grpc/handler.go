package grpc

import (
	"context"
	"errors"

	pb "content-service/api/proto/contentpb"
	articlecommand "content-service/internal/application/article/command"
	articlequery "content-service/internal/application/article/query"
	categorycommand "content-service/internal/application/category/command"
	categoryquery "content-service/internal/application/category/query"
	topiccommand "content-service/internal/application/topic/command"
	topicquery "content-service/internal/application/topic/query"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	topicDomain "content-service/internal/domain/topic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedContentServiceServer
	articleCmd  *articlecommand.Service
	articleQry  *articlequery.Service
	topicCmd    *topiccommand.Service
	topicQry    *topicquery.Service
	categoryCmd *categorycommand.Service
	categoryQry *categoryquery.Service
}

func NewHandler(articleCmd *articlecommand.Service, articleQry *articlequery.Service, topicCmd *topiccommand.Service, topicQry *topicquery.Service, categoryCmd *categorycommand.Service, categoryQry *categoryquery.Service) *Handler {
	return &Handler{articleCmd: articleCmd, articleQry: articleQry, topicCmd: topicCmd, topicQry: topicQry, categoryCmd: categoryCmd, categoryQry: categoryQry}
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch {
	case errors.Is(err, articleDomain.ErrNotFound),
		errors.Is(err, topicDomain.ErrNotFound),
		errors.Is(err, topicDomain.ErrCommentNotFound),
		errors.Is(err, categoryDomain.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, articleDomain.ErrSlugExists), errors.Is(err, topicDomain.ErrSlugExists), errors.Is(err, categoryDomain.ErrSlugExists):
		code = codes.AlreadyExists
	case errors.Is(err, articleDomain.ErrSlugRequired),
		errors.Is(err, articleDomain.ErrTitleRequired),
		errors.Is(err, articleDomain.ErrBodyRequired),
		errors.Is(err, articleDomain.ErrAuthorRequired),
		errors.Is(err, topicDomain.ErrSlugRequired),
		errors.Is(err, topicDomain.ErrTitleRequired),
		errors.Is(err, topicDomain.ErrBodyRequired),
		errors.Is(err, topicDomain.ErrAuthorRequired),
		errors.Is(err, topicDomain.ErrBountyInvalid),
		errors.Is(err, topicDomain.ErrInvalidComment),
		errors.Is(err, topicDomain.ErrCommentNotInTopic),
		errors.Is(err, categoryDomain.ErrSlugRequired),
		errors.Is(err, categoryDomain.ErrNameRequired):
		code = codes.InvalidArgument
	case errors.Is(err, articleDomain.ErrAlreadyPublished),
		errors.Is(err, articleDomain.ErrNotPublished),
		errors.Is(err, articleDomain.ErrArchived),
		errors.Is(err, topicDomain.ErrAlreadyPublished),
		errors.Is(err, topicDomain.ErrNotPublished),
		errors.Is(err, topicDomain.ErrArchived),
		errors.Is(err, topicDomain.ErrNotQuestion),
		errors.Is(err, topicDomain.ErrAlreadyAccepted),
		errors.Is(err, categoryDomain.ErrInUse):
		code = codes.FailedPrecondition
	}
	return status.Error(code, err.Error())
}

func toPbTopic(t *topicDomain.Topic) *pb.TopicInfo {
	if t == nil {
		return nil
	}
	var publishedAt int64
	if t.PublishedAt != nil {
		publishedAt = t.PublishedAt.UnixMilli()
	}
	return &pb.TopicInfo{
		Id:                t.ID,
		Slug:              t.Slug,
		Type:              string(t.Type),
		Title:             t.Title,
		Body:              t.Body,
		Tags:              t.Tags,
		AuthorId:          t.AuthorID,
		Status:            int32(t.Status),
		CreatedAt:         t.CreatedAt.UnixMilli(),
		UpdatedAt:         t.UpdatedAt.UnixMilli(),
		PublishedAt:       publishedAt,
		CategoryId:        t.CategoryID,
		ViewCount:         t.ViewCount,
		BountyScore:       t.BountyScore,
		QaStatus:          string(t.QAStatus),
		AcceptedCommentId: t.AcceptedCommentID,
	}
}

func toPbTopicList(rows []topicquery.TopicView) []*pb.TopicInfo {
	out := make([]*pb.TopicInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPbTopic(row.Topic))
	}
	return out
}

func toPbCategory(c *categoryDomain.Category) *pb.CategoryInfo {
	if c == nil {
		return nil
	}
	return &pb.CategoryInfo{
		Id:          c.ID,
		Slug:        c.Slug,
		Name:        c.Name,
		Description: c.Description,
		Sort:        c.Sort,
		Status:      int32(c.Status),
		TopicCount:  c.TopicCount,
		CreatedAt:   c.CreatedAt.UnixMilli(),
		UpdatedAt:   c.UpdatedAt.UnixMilli(),
	}
}

func toPbCategories(rows []categoryquery.CategoryView) []*pb.CategoryInfo {
	out := make([]*pb.CategoryInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPbCategory(row.Category))
	}
	return out
}

func toPb(a *articleDomain.Article) *pb.ArticleInfo {
	if a == nil {
		return nil
	}
	var publishedAt int64
	if a.PublishedAt != nil {
		publishedAt = a.PublishedAt.UnixMilli()
	}
	return &pb.ArticleInfo{
		Id:          a.ID,
		Slug:        a.Slug,
		Title:       a.Title,
		Summary:     a.Summary,
		Body:        a.Body,
		CoverUrl:    a.CoverURL,
		Tags:        a.Tags,
		AuthorId:    a.AuthorID,
		Status:      int32(a.Status),
		CreatedAt:   a.CreatedAt.UnixMilli(),
		UpdatedAt:   a.UpdatedAt.UnixMilli(),
		PublishedAt: publishedAt,
		ViewCount:   a.ViewCount,
	}
}

func toPbList(rows []articlequery.ArticleView) []*pb.ArticleInfo {
	out := make([]*pb.ArticleInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPb(row.Article))
	}
	return out
}

func toPbTags(rows []articleDomain.TagStats) []*pb.TagInfo {
	out := make([]*pb.TagInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, &pb.TagInfo{Name: row.Name, Count: row.Count})
	}
	return out
}

func (h *Handler) CreateTopic(ctx context.Context, req *pb.CreateTopicRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.Create(ctx, topicDomain.CreateCmd{
		Slug:        req.GetSlug(),
		Type:        req.GetType(),
		Title:       req.GetTitle(),
		Body:        req.GetBody(),
		Tags:        req.GetTags(),
		AuthorID:    req.GetAuthorId(),
		CategoryID:  req.GetCategoryId(),
		BountyScore: req.GetBountyScore(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) UpdateTopic(ctx context.Context, req *pb.UpdateTopicRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.Update(ctx, req.GetId(), topicDomain.UpdateCmd{
		Title:       req.GetTitle(),
		Body:        req.GetBody(),
		Tags:        req.GetTags(),
		CategoryID:  req.GetCategoryId(),
		BountyScore: req.GetBountyScore(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) PublishTopic(ctx context.Context, req *pb.TopicIDRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.Publish(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) HideTopic(ctx context.Context, req *pb.TopicIDRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.Hide(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) ArchiveTopic(ctx context.Context, req *pb.TopicIDRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.Archive(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) AcceptTopicComment(ctx context.Context, req *pb.AcceptTopicCommentRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.AcceptComment(ctx, req.GetTopicId(), req.GetCommentId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) GetTopic(ctx context.Context, req *pb.GetTopicRequest) (*pb.TopicResponse, error) {
	var (
		view topicquery.TopicView
		err  error
	)
	if req.GetSlug() != "" {
		view, err = h.topicQry.GetBySlug(ctx, req.GetSlug())
	} else {
		view, err = h.topicQry.GetByID(ctx, req.GetId())
	}
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(view.Topic)}, nil
}

func (h *Handler) ListTopics(ctx context.Context, req *pb.ListTopicsRequest) (*pb.TopicListResponse, error) {
	var typ topicDomain.Type
	if req.GetType() != "" {
		typ = topicDomain.NormalizeType(req.GetType())
	}
	rows, err := h.topicQry.List(ctx, topicDomain.Status(req.GetStatus()), typ, req.GetTag(), req.GetAuthorId(), req.GetCategoryId(), req.GetSort(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicListResponse{Items: toPbTopicList(rows)}, nil
}

func (h *Handler) ListCategories(ctx context.Context, req *pb.ListCategoriesRequest) (*pb.CategoryListResponse, error) {
	rows, err := h.categoryQry.List(ctx, categoryDomain.Status(req.GetStatus()), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CategoryListResponse{Items: toPbCategories(rows)}, nil
}

func (h *Handler) GetCategory(ctx context.Context, req *pb.CategoryIDRequest) (*pb.CategoryResponse, error) {
	view, err := h.categoryQry.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CategoryResponse{Success: true, Message: "ok", Category: toPbCategory(view.Category)}, nil
}

func (h *Handler) CreateCategory(ctx context.Context, req *pb.UpsertCategoryRequest) (*pb.CategoryResponse, error) {
	category, err := h.categoryCmd.Create(ctx, categoryDomain.CreateCmd{
		Slug:        req.GetSlug(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Sort:        req.GetSort(),
		Status:      categoryDomain.Status(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CategoryResponse{Success: true, Message: "ok", Category: toPbCategory(category)}, nil
}

func (h *Handler) UpdateCategory(ctx context.Context, req *pb.UpsertCategoryRequest) (*pb.CategoryResponse, error) {
	category, err := h.categoryCmd.Update(ctx, req.GetId(), categoryDomain.UpdateCmd{
		Slug:        req.GetSlug(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Sort:        req.GetSort(),
		Status:      categoryDomain.Status(req.GetStatus()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CategoryResponse{Success: true, Message: "ok", Category: toPbCategory(category)}, nil
}

func (h *Handler) DeleteCategory(ctx context.Context, req *pb.CategoryIDRequest) (*pb.CategoryResponse, error) {
	if err := h.categoryCmd.Delete(ctx, req.GetId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.CategoryResponse{Success: true, Message: "ok"}, nil
}

func (h *Handler) CreateArticle(ctx context.Context, req *pb.CreateArticleRequest) (*pb.ArticleResponse, error) {
	a, err := h.articleCmd.Create(ctx, articleDomain.CreateCmd{
		Slug:     req.GetSlug(),
		Title:    req.GetTitle(),
		Summary:  req.GetSummary(),
		Body:     req.GetBody(),
		CoverURL: req.GetCoverUrl(),
		Tags:     req.GetTags(),
		AuthorID: req.GetAuthorId(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(a)}, nil
}

func (h *Handler) UpdateArticle(ctx context.Context, req *pb.UpdateArticleRequest) (*pb.ArticleResponse, error) {
	a, err := h.articleCmd.Update(ctx, req.GetId(), articleDomain.UpdateCmd{
		Title:    req.GetTitle(),
		Summary:  req.GetSummary(),
		Body:     req.GetBody(),
		CoverURL: req.GetCoverUrl(),
		Tags:     req.GetTags(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(a)}, nil
}

func (h *Handler) PublishArticle(ctx context.Context, req *pb.ArticleIDRequest) (*pb.ArticleResponse, error) {
	a, err := h.articleCmd.Publish(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(a)}, nil
}

func (h *Handler) HideArticle(ctx context.Context, req *pb.ArticleIDRequest) (*pb.ArticleResponse, error) {
	a, err := h.articleCmd.Hide(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(a)}, nil
}

func (h *Handler) ArchiveArticle(ctx context.Context, req *pb.ArticleIDRequest) (*pb.ArticleResponse, error) {
	a, err := h.articleCmd.Archive(ctx, req.GetId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(a)}, nil
}

func (h *Handler) GetArticle(ctx context.Context, req *pb.GetArticleRequest) (*pb.ArticleResponse, error) {
	var (
		view articlequery.ArticleView
		err  error
	)
	if req.GetSlug() != "" {
		view, err = h.articleQry.GetBySlug(ctx, req.GetSlug())
	} else {
		view, err = h.articleQry.GetByID(ctx, req.GetId())
	}
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(view.Article)}, nil
}

func (h *Handler) ListArticles(ctx context.Context, req *pb.ListArticlesRequest) (*pb.ArticleListResponse, error) {
	rows, err := h.articleQry.List(ctx, articleDomain.Status(req.GetStatus()), req.GetTag(), req.GetAuthorId(), req.GetSort(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleListResponse{Items: toPbList(rows)}, nil
}

func (h *Handler) FeedArticlesByTime(ctx context.Context, req *pb.FeedArticlesByTimeRequest) (*pb.ArticleListResponse, error) {
	rows, err := h.articleQry.FeedByTime(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleListResponse{Items: toPbList(rows)}, nil
}

func (h *Handler) ListTags(ctx context.Context, req *pb.ListTagsRequest) (*pb.TagListResponse, error) {
	rows, err := h.articleQry.ListTags(ctx, articleDomain.Status(req.GetStatus()), req.GetQuery(), int(req.GetLimit()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TagListResponse{Items: toPbTags(rows)}, nil
}

func (h *Handler) AutocompleteTags(ctx context.Context, req *pb.AutocompleteTagsRequest) (*pb.TagListResponse, error) {
	rows, err := h.articleQry.ListTags(ctx, articleDomain.StatusPublished, req.GetQuery(), int(req.GetLimit()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TagListResponse{Items: toPbTags(rows)}, nil
}
