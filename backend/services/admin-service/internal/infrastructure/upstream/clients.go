package upstream

import (
	"context"
	"fmt"

	"admin/internal/clients/pb/commentpb"
	"admin/internal/clients/pb/contentpb"
	"admin/internal/clients/pb/reactionpb"
	"admin/internal/clients/pb/userpb"
	domain "admin/internal/domain/admin"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Options struct {
	User     string
	Reaction string
	Content  string
	Comment  string
}

type Clients struct {
	user     userpb.UserServiceClient
	reaction reactionpb.ReactionServiceClient
	content  contentpb.ContentServiceClient
	comment  commentpb.CommentServiceClient
	conns    []*grpc.ClientConn
}

func New(o Options) (*Clients, error) {
	userConn, err := dial(o.User, "user")
	if err != nil {
		return nil, err
	}
	reactionConn, err := dial(o.Reaction, "reaction")
	if err != nil {
		_ = userConn.Close()
		return nil, err
	}
	contentConn, err := dial(o.Content, "content")
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		return nil, err
	}
	commentConn, err := dial(o.Comment, "comment")
	if err != nil {
		_ = userConn.Close()
		_ = reactionConn.Close()
		_ = contentConn.Close()
		return nil, err
	}
	return &Clients{
		user:     userpb.NewUserServiceClient(userConn),
		reaction: reactionpb.NewReactionServiceClient(reactionConn),
		content:  contentpb.NewContentServiceClient(contentConn),
		comment:  commentpb.NewCommentServiceClient(commentConn),
		conns:    []*grpc.ClientConn{userConn, reactionConn, contentConn, commentConn},
	}, nil
}

func (c *Clients) Close() error {
	var first error
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (c *Clients) ListUsers(ctx context.Context, query string, status int32, page int32, pageSize int32) (domain.UserList, error) {
	resp, err := c.user.ListUsers(ctx, &userpb.ListUsersRequest{Query: query, Status: status, Page: page, PageSize: pageSize})
	if err != nil {
		return domain.UserList{}, err
	}
	items := make([]domain.User, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainUser(item))
	}
	return domain.UserList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) UpdateStatus(ctx context.Context, userID int64, status int32) (domain.User, error) {
	resp, err := c.user.UpdateStatus(ctx, &userpb.UpdateStatusRequest{Id: userID, Status: status})
	if err != nil {
		return domain.User{}, err
	}
	return toDomainUser(resp.GetUser()), nil
}

func (c *Clients) ListReports(ctx context.Context, status int32, limit int32, offset int32) (domain.ReportList, error) {
	resp, err := c.reaction.ListReports(ctx, &reactionpb.ListReportsRequest{Status: status, Limit: limit, Offset: offset})
	if err != nil {
		return domain.ReportList{}, err
	}
	items := make([]domain.Report, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainReport(item))
	}
	return domain.ReportList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) AuditReport(ctx context.Context, id int64, status int32, handlerID int64) (domain.Report, error) {
	resp, err := c.reaction.AuditReport(ctx, &reactionpb.AuditReportRequest{Id: id, Status: status, HandlerId: handlerID})
	if err != nil {
		return domain.Report{}, err
	}
	return toDomainReport(resp.GetReport()), nil
}

func (c *Clients) ListArticles(ctx context.Context, status int32, tag string, authorID int64, limit int32, offset int32) (domain.ArticleList, error) {
	resp, err := c.content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status:   status,
		Tag:      tag,
		AuthorId: authorID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return domain.ArticleList{}, err
	}
	items := make([]domain.Article, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainArticle(item))
	}
	return domain.ArticleList{Items: items, Total: int64(len(items))}, nil
}

func (c *Clients) HideArticle(ctx context.Context, id int64) (domain.Article, error) {
	resp, err := c.content.HideArticle(ctx, &contentpb.ArticleIDRequest{Id: id})
	if err != nil {
		return domain.Article{}, err
	}
	return toDomainArticle(resp.GetArticle()), nil
}

func (c *Clients) ArchiveArticle(ctx context.Context, id int64) (domain.Article, error) {
	resp, err := c.content.ArchiveArticle(ctx, &contentpb.ArticleIDRequest{Id: id})
	if err != nil {
		return domain.Article{}, err
	}
	return toDomainArticle(resp.GetArticle()), nil
}

func (c *Clients) ListTopics(ctx context.Context, status int32, typ string, tag string, authorID int64, categoryID int64, limit int32, offset int32) (domain.TopicList, error) {
	resp, err := c.content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status:     status,
		Type:       typ,
		Tag:        tag,
		AuthorId:   authorID,
		CategoryId: categoryID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return domain.TopicList{}, err
	}
	items := make([]domain.Topic, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainTopic(item))
	}
	return domain.TopicList{Items: items, Total: int64(len(items))}, nil
}

func (c *Clients) HideTopic(ctx context.Context, id int64) (domain.Topic, error) {
	resp, err := c.content.HideTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		return domain.Topic{}, err
	}
	return toDomainTopic(resp.GetTopic()), nil
}

func (c *Clients) ArchiveTopic(ctx context.Context, id int64) (domain.Topic, error) {
	resp, err := c.content.ArchiveTopic(ctx, &contentpb.TopicIDRequest{Id: id})
	if err != nil {
		return domain.Topic{}, err
	}
	return toDomainTopic(resp.GetTopic()), nil
}

func (c *Clients) ListCategories(ctx context.Context, status int32, limit int32, offset int32) (domain.CategoryList, error) {
	resp, err := c.content.ListCategories(ctx, &contentpb.ListCategoriesRequest{Status: status, Limit: limit, Offset: offset})
	if err != nil {
		return domain.CategoryList{}, err
	}
	items := make([]domain.Category, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainCategory(item))
	}
	return domain.CategoryList{Items: items, Total: int64(len(items))}, nil
}

func (c *Clients) CreateCategory(ctx context.Context, command domain.UpsertCategoryCommand) (domain.Category, error) {
	resp, err := c.content.CreateCategory(ctx, toContentCategoryRequest(command))
	if err != nil {
		return domain.Category{}, err
	}
	return toDomainCategory(resp.GetCategory()), nil
}

func (c *Clients) UpdateCategory(ctx context.Context, command domain.UpsertCategoryCommand) (domain.Category, error) {
	resp, err := c.content.UpdateCategory(ctx, toContentCategoryRequest(command))
	if err != nil {
		return domain.Category{}, err
	}
	return toDomainCategory(resp.GetCategory()), nil
}

func (c *Clients) DeleteCategory(ctx context.Context, id int64) error {
	_, err := c.content.DeleteCategory(ctx, &contentpb.CategoryIDRequest{Id: id})
	return err
}

func (c *Clients) ListComments(ctx context.Context, entityType string, entityID int64, authorID int64, status int32, page int32, pageSize int32) (domain.CommentList, error) {
	resp, err := c.comment.ListRecentComments(ctx, &commentpb.ListRecentCommentsRequest{
		EntityType: entityType,
		EntityId:   entityID,
		AuthorId:   authorID,
		Status:     status,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return domain.CommentList{}, err
	}
	items := make([]domain.Comment, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, toDomainComment(item))
	}
	return domain.CommentList{Items: items, Total: resp.GetTotal()}, nil
}

func (c *Clients) HideComment(ctx context.Context, id int64, actorID int64) error {
	_, err := c.comment.DeleteComment(ctx, &commentpb.DeleteCommentRequest{Id: id, ActorId: actorID, Moderator: true})
	return err
}

func toDomainUser(u *userpb.UserInfo) domain.User {
	if u == nil {
		return domain.User{}
	}
	return domain.User{
		ID:             u.GetId(),
		Username:       u.GetUsername(),
		Email:          u.GetEmail(),
		Nickname:       u.GetNickname(),
		AvatarURL:      u.GetAvatarUrl(),
		Bio:            u.GetBio(),
		Status:         u.GetStatus(),
		FollowerCount:  u.GetFollowerCount(),
		FollowingCount: u.GetFollowingCount(),
		CreatedAt:      u.GetCreatedAt(),
		UpdatedAt:      u.GetUpdatedAt(),
		LastLoginAt:    u.GetLastLoginAt(),
	}
}

func toDomainReport(r *reactionpb.ReportInfo) domain.Report {
	if r == nil {
		return domain.Report{}
	}
	var entity domain.EntityRef
	if r.GetEntity() != nil {
		entity = domain.EntityRef{EntityType: r.GetEntity().GetEntityType(), EntityID: r.GetEntity().GetEntityId()}
	}
	return domain.Report{
		ID:          r.GetId(),
		Entity:      entity,
		ReporterID:  r.GetReporterId(),
		Reason:      r.GetReason(),
		Description: r.GetDescription(),
		Status:      r.GetStatus(),
		HandledBy:   r.GetHandledBy(),
		HandledAt:   r.GetHandledAt(),
		CreatedAt:   r.GetCreatedAt(),
		UpdatedAt:   r.GetUpdatedAt(),
	}
}

func toDomainArticle(a *contentpb.ArticleInfo) domain.Article {
	if a == nil {
		return domain.Article{}
	}
	return domain.Article{
		ID:          a.GetId(),
		Slug:        a.GetSlug(),
		Title:       a.GetTitle(),
		Summary:     a.GetSummary(),
		Body:        a.GetBody(),
		CoverURL:    a.GetCoverUrl(),
		Tags:        a.GetTags(),
		AuthorID:    a.GetAuthorId(),
		Status:      a.GetStatus(),
		CreatedAt:   a.GetCreatedAt(),
		UpdatedAt:   a.GetUpdatedAt(),
		PublishedAt: a.GetPublishedAt(),
	}
}

func toDomainTopic(t *contentpb.TopicInfo) domain.Topic {
	if t == nil {
		return domain.Topic{}
	}
	return domain.Topic{
		ID:          t.GetId(),
		Slug:        t.GetSlug(),
		Type:        t.GetType(),
		Title:       t.GetTitle(),
		Body:        t.GetBody(),
		Tags:        t.GetTags(),
		AuthorID:    t.GetAuthorId(),
		CategoryID:  t.GetCategoryId(),
		Status:      t.GetStatus(),
		CreatedAt:   t.GetCreatedAt(),
		UpdatedAt:   t.GetUpdatedAt(),
		PublishedAt: t.GetPublishedAt(),
	}
}

func toDomainCategory(c *contentpb.CategoryInfo) domain.Category {
	if c == nil {
		return domain.Category{}
	}
	return domain.Category{
		ID:          c.GetId(),
		Slug:        c.GetSlug(),
		Name:        c.GetName(),
		Description: c.GetDescription(),
		Sort:        c.GetSort(),
		Status:      c.GetStatus(),
		TopicCount:  c.GetTopicCount(),
		CreatedAt:   c.GetCreatedAt(),
		UpdatedAt:   c.GetUpdatedAt(),
	}
}

func toContentCategoryRequest(command domain.UpsertCategoryCommand) *contentpb.UpsertCategoryRequest {
	return &contentpb.UpsertCategoryRequest{
		Id:          command.ID,
		Slug:        command.Slug,
		Name:        command.Name,
		Description: command.Description,
		Sort:        command.Sort,
		Status:      command.Status,
	}
}

func toDomainComment(c *commentpb.CommentInfo) domain.Comment {
	if c == nil {
		return domain.Comment{}
	}
	return domain.Comment{
		ID:         c.GetId(),
		EntityType: c.GetEntityType(),
		EntityID:   c.GetEntityId(),
		RootID:     c.GetRootId(),
		ParentID:   c.GetParentId(),
		AuthorID:   c.GetAuthorId(),
		Content:    c.GetContent(),
		Status:     c.GetStatus(),
		ReplyCount: c.GetReplyCount(),
		LikeCount:  c.GetLikeCount(),
		CreatedAt:  c.GetCreatedAt(),
		UpdatedAt:  c.GetUpdatedAt(),
	}
}

func dial(addr string, name string) (*grpc.ClientConn, error) {
	if addr == "" {
		return nil, fmt.Errorf("%s upstream required", name)
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
