package upstream

import (
	"context"

	"admin/api/proto/commentpb"
	domain "admin/internal/domain/admin"
)

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

func (c *Clients) RestoreComment(ctx context.Context, id int64, actorID int64) error {
	_, err := c.comment.RestoreComment(ctx, &commentpb.RestoreCommentRequest{Id: id, ActorId: actorID, Moderator: true})
	return err
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
