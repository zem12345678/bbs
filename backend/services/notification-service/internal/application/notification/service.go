package notification

import (
	"context"
	"fmt"
	"time"

	domain "notification-service/internal/domain/notification"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, userID int64, limit, offset int32, unreadOnly bool) ([]domain.Notification, int64, int64, error) {
	return s.repo.List(ctx, userID, limit, offset, unreadOnly)
}

func (s *Service) CountUnread(ctx context.Context, userID int64) (int64, error) {
	return s.repo.UnreadCount(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, id int64) error {
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *Service) MarkAllRead(ctx context.Context, userID int64) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *Service) UpsertArticle(ctx context.Context, articleID, authorID int64, title string, publishedAt time.Time) error {
	article := domain.ArticleRef{ID: articleID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveArticle(ctx, article, publishedAt); err != nil {
		return err
	}
	return s.repo.FlushPendingArticleNotifications(ctx, article)
}

func (s *Service) NotifyFollow(ctx context.Context, eventID string, followerID, followeeID int64, occurredAt time.Time) error {
	if followerID <= 0 || followeeID <= 0 || followerID == followeeID {
		return nil
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     followeeID,
		Type:       "follow",
		Title:      "有新粉丝",
		Content:    fmt.Sprintf("用户 #%d 关注了你", followerID),
		ActorID:    followerID,
		EntityType: "user",
		EntityID:   followerID,
		SourceID:   followerID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyComment(ctx context.Context, eventID string, commentID, articleID, actorID int64, occurredAt time.Time) error {
	if commentID <= 0 || articleID <= 0 || actorID <= 0 {
		return nil
	}
	article, err := s.repo.GetArticle(ctx, articleID)
	if err != nil {
		return err
	}
	if article.AuthorID <= 0 {
		return s.repo.SavePendingArticleNotification(ctx, eventID, "comment", articleID, actorID, commentID, occurredAt)
	}
	if article.AuthorID == actorID {
		return nil
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     article.AuthorID,
		Type:       "comment",
		Title:      "文章收到新评论",
		Content:    fmt.Sprintf("用户 #%d 评论了《%s》", actorID, article.Title),
		ActorID:    actorID,
		EntityType: "article",
		EntityID:   articleID,
		SourceID:   commentID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyReaction(ctx context.Context, eventID string, kind string, articleID, actorID int64, occurredAt time.Time) error {
	if articleID <= 0 || actorID <= 0 {
		return nil
	}
	article, err := s.repo.GetArticle(ctx, articleID)
	if err != nil {
		return err
	}
	if article.AuthorID <= 0 {
		return s.repo.SavePendingArticleNotification(ctx, eventID, kind, articleID, actorID, articleID, occurredAt)
	}
	if article.AuthorID == actorID {
		return nil
	}
	title := "文章收到互动"
	content := fmt.Sprintf("用户 #%d 与《%s》发生了互动", actorID, article.Title)
	if kind == "like" {
		title = "文章被点赞"
		content = fmt.Sprintf("用户 #%d 点赞了《%s》", actorID, article.Title)
	}
	if kind == "favorite" {
		title = "文章被收藏"
		content = fmt.Sprintf("用户 #%d 收藏了《%s》", actorID, article.Title)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     article.AuthorID,
		Type:       kind,
		Title:      title,
		Content:    content,
		ActorID:    actorID,
		EntityType: "article",
		EntityID:   articleID,
		SourceID:   articleID,
	}, eventID, occurredAt)
}
