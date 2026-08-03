package grpc

import (
	"context"
	"errors"
	"time"

	pb "content-service/api/proto/contentpb"
	accountcommand "content-service/internal/application/account"
	articlecommand "content-service/internal/application/article/command"
	articlequery "content-service/internal/application/article/query"
	categorycommand "content-service/internal/application/category/command"
	categoryquery "content-service/internal/application/category/query"
	topiccommand "content-service/internal/application/topic/command"
	topicquery "content-service/internal/application/topic/query"
	accountDomain "content-service/internal/domain/account"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	topicDomain "content-service/internal/domain/topic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedContentServiceServer
	articleCmd   *articlecommand.Service
	articleQry   *articlequery.Service
	topicCmd     *topiccommand.Service
	topicQry     *topicquery.Service
	categoryCmd  *categorycommand.Service
	categoryQry  *categoryquery.Service
	accountErase *accountcommand.Service
}

func NewHandler(articleCmd *articlecommand.Service, articleQry *articlequery.Service, topicCmd *topiccommand.Service, topicQry *topicquery.Service, categoryCmd *categorycommand.Service, categoryQry *categoryquery.Service, accountErasers ...*accountcommand.Service) *Handler {
	var accountEraser *accountcommand.Service
	if len(accountErasers) > 0 {
		accountEraser = accountErasers[0]
	}
	return &Handler{articleCmd: articleCmd, articleQry: articleQry, topicCmd: topicCmd, topicQry: topicQry, categoryCmd: categoryCmd, categoryQry: categoryQry, accountErase: accountEraser}
}

func toStatus(err error) error {
	if err == nil {
		return nil
	}
	code := codes.Internal
	switch {
	case errors.Is(err, articleDomain.ErrNotFound),
		errors.Is(err, topicDomain.ErrNotFound),
		errors.Is(err, topicDomain.ErrPollNotFound),
		errors.Is(err, topicDomain.ErrCommentNotFound),
		errors.Is(err, categoryDomain.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, articleDomain.ErrSlugExists), errors.Is(err, topicDomain.ErrSlugExists), errors.Is(err, categoryDomain.ErrSlugExists):
		code = codes.AlreadyExists
	case errors.Is(err, topicDomain.ErrMembershipEntitlementRequired),
		errors.Is(err, topicDomain.ErrTopicOwnerMismatch):
		code = codes.PermissionDenied
	case errors.Is(err, articleDomain.ErrSlugRequired),
		errors.Is(err, articleDomain.ErrTitleRequired),
		errors.Is(err, articleDomain.ErrBodyRequired),
		errors.Is(err, articleDomain.ErrAuthorRequired),
		errors.Is(err, topicDomain.ErrSlugRequired),
		errors.Is(err, topicDomain.ErrTitleRequired),
		errors.Is(err, topicDomain.ErrBodyRequired),
		errors.Is(err, topicDomain.ErrAuthorRequired),
		errors.Is(err, topicDomain.ErrBountyInvalid),
		errors.Is(err, topicDomain.ErrPollChoicesInvalid),
		errors.Is(err, topicDomain.ErrPollChoiceInvalid),
		errors.Is(err, topicDomain.ErrPollChoiceDuplicate),
		errors.Is(err, topicDomain.ErrPollExpiryInvalid),
		errors.Is(err, topicDomain.ErrPollSelectionInvalid),
		errors.Is(err, topicDomain.ErrInvalidComment),
		errors.Is(err, topicDomain.ErrCommentNotInTopic),
		errors.Is(err, categoryDomain.ErrSlugRequired),
		errors.Is(err, categoryDomain.ErrNameRequired),
		errors.Is(err, accountDomain.ErrInvalidErasure):
		code = codes.InvalidArgument
	case errors.Is(err, articleDomain.ErrAlreadyPublished),
		errors.Is(err, articleDomain.ErrNotPublished),
		errors.Is(err, articleDomain.ErrArchived),
		errors.Is(err, topicDomain.ErrAlreadyPublished),
		errors.Is(err, topicDomain.ErrNotPublished),
		errors.Is(err, topicDomain.ErrArchived),
		errors.Is(err, topicDomain.ErrNotQuestion),
		errors.Is(err, topicDomain.ErrAlreadyAccepted),
		errors.Is(err, topicDomain.ErrNotAccepted),
		errors.Is(err, topicDomain.ErrCannotAcceptOwnComment),
		errors.Is(err, topicDomain.ErrBountyCreditInsufficient),
		errors.Is(err, topicDomain.ErrQAAcceptanceReversalInsufficientCredit),
		errors.Is(err, topicDomain.ErrBountyCreditReleaseFailed),
		errors.Is(err, topicDomain.ErrPollExpired),
		errors.Is(err, topicDomain.ErrPollLocked),
		errors.Is(err, categoryDomain.ErrInUse),
		errors.Is(err, accountDomain.ErrUserErased):
		code = codes.FailedPrecondition
	case errors.Is(err, topicDomain.ErrPollAlreadyVoted):
		code = codes.AlreadyExists
	case errors.Is(err, topicDomain.ErrQAAcceptanceSettlementPending):
		code = codes.Aborted
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
		Poll:              toPbTopicPoll(t.Poll),
	}
}

func toPbTopicPoll(poll *topicDomain.Poll) *pb.TopicPollInfo {
	if poll == nil {
		return nil
	}
	selected := make(map[int32]struct{}, len(poll.MyChoices))
	for _, index := range poll.MyChoices {
		selected[index] = struct{}{}
	}
	choices := make([]*pb.TopicPollChoiceInfo, 0, len(poll.Choices))
	for _, choice := range poll.Choices {
		_, isSelected := selected[choice.Index]
		choices = append(choices, &pb.TopicPollChoiceInfo{Index: choice.Index, Text: choice.Text, Votes: choice.Votes, Selected: isSelected})
	}
	var expiresAt int64
	var expired bool
	if poll.ExpiresAt != nil {
		expiresAt = poll.ExpiresAt.UnixMilli()
		expired = !poll.ExpiresAt.After(time.Now())
	}
	return &pb.TopicPollInfo{Multiple: poll.Multiple, Choices: choices, ExpiresAt: expiresAt, TotalVoters: poll.TotalVoters, HasVoted: len(poll.MyChoices) > 0, Expired: expired}
}

func pollInputFromPb(input *pb.TopicPollInput) *topicDomain.PollInput {
	if input == nil {
		return nil
	}
	var expiresAt *time.Time
	if input.GetExpiresAt() > 0 {
		value := time.UnixMilli(input.GetExpiresAt())
		expiresAt = &value
	}
	return &topicDomain.PollInput{Enabled: input.GetEnabled(), Multiple: input.GetMultiple(), Choices: input.GetChoices(), ExpiresAt: expiresAt}
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
		Poll:        pollInputFromPb(req.GetPoll()),
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
		Poll:        pollInputFromPb(req.GetPoll()),
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
	t, err := h.topicCmd.AcceptComment(ctx, req.GetTopicId(), req.GetCommentId(), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(t)}, nil
}

func (h *Handler) UnacceptTopicComment(ctx context.Context, req *pb.UnacceptTopicCommentRequest) (*pb.TopicResponse, error) {
	t, err := h.topicCmd.UnacceptComment(ctx, req.GetTopicId(), req.GetCommentId(), req.GetUserId())
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
		view, err = h.topicQry.GetBySlugForViewer(ctx, req.GetSlug(), req.GetTrackView(), req.GetViewerUserId())
	} else {
		view, err = h.topicQry.GetByIDForViewer(ctx, req.GetId(), req.GetTrackView(), req.GetViewerUserId())
	}
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicResponse{Success: true, Message: "ok", Topic: toPbTopic(view.Topic)}, nil
}

func (h *Handler) VoteTopicPoll(ctx context.Context, req *pb.VoteTopicPollRequest) (*pb.TopicPollResponse, error) {
	poll, err := h.topicCmd.VotePoll(ctx, req.GetTopicId(), req.GetUserId(), req.GetChoices())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicPollResponse{Success: true, Message: "ok", Poll: toPbTopicPoll(poll)}, nil
}

func (h *Handler) ListTopics(ctx context.Context, req *pb.ListTopicsRequest) (*pb.TopicListResponse, error) {
	var typ topicDomain.Type
	if req.GetType() != "" {
		typ = topicDomain.NormalizeType(req.GetType())
	}
	rows, total, err := h.topicQry.List(ctx, topicDomain.Status(req.GetStatus()), typ, req.GetTag(), req.GetAuthorId(), req.GetCategoryId(), req.GetSort(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.TopicListResponse{Items: toPbTopicList(rows), Total: total}, nil
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
		view, err = h.articleQry.GetBySlug(ctx, req.GetSlug(), req.GetTrackView())
	} else {
		view, err = h.articleQry.GetByID(ctx, req.GetId(), req.GetTrackView())
	}
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleResponse{Success: true, Message: "ok", Article: toPb(view.Article)}, nil
}

func (h *Handler) ListArticles(ctx context.Context, req *pb.ListArticlesRequest) (*pb.ArticleListResponse, error) {
	rows, total, err := h.articleQry.List(ctx, articleDomain.Status(req.GetStatus()), req.GetTag(), req.GetAuthorId(), req.GetSort(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArticleListResponse{Items: toPbList(rows), Total: total}, nil
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

func (h *Handler) ArchiveAccountContent(ctx context.Context, req *pb.ArchiveAccountContentRequest) (*pb.ArchiveAccountContentResponse, error) {
	if h.accountErase == nil {
		return nil, status.Error(codes.Unavailable, "account erasure service unavailable")
	}
	result, err := h.accountErase.ArchiveAccountContent(ctx, req.GetUserId(), req.GetDeletionJobId(), req.GetPolicyVersion())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.ArchiveAccountContentResponse{
		Completed:          true,
		ArchivedArticles:   result.ArchivedArticles,
		ArchivedTopics:     result.ArchivedTopics,
		DeletedPollBallots: result.DeletedPollBallots,
	}, nil
}
