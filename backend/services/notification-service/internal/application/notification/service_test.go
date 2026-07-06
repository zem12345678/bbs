package notification

import (
	"context"
	"strconv"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"
)

func TestNotifyTopicCommentCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:101"] = domain.ContentRef{EntityType: "topic", ID: 101, AuthorID: 10, Title: "社区路线图"}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-comment-topic", 9001, "topic", 101, 22, time.Now()); err != nil {
		t.Fatalf("notify comment: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 10 || item.EntityType != "topic" || item.EntityID != 101 || item.SourceID != 9001 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "话题收到新评论" {
		t.Fatalf("title = %q", item.Title)
	}
}

func TestNotifyTopicReactionCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:102"] = domain.ContentRef{EntityType: "topic", ID: 102, AuthorID: 10, Title: "发布规范"}
	svc := NewService(repo)

	if err := svc.NotifyReaction(context.Background(), "evt-like-topic", "like", "topic", 102, 33, time.Now()); err != nil {
		t.Fatalf("notify reaction: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.Title != "话题被点赞" || item.Type != "like" || item.EntityType != "topic" || item.UserID != 10 {
		t.Fatalf("notification = %+v", item)
	}
}

func TestNotifyOwnContentDoesNotCreateNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:103"] = domain.ContentRef{EntityType: "topic", ID: 103, AuthorID: 10, Title: "自评测试"}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-own-comment", 9002, "topic", 103, 10, time.Now()); err != nil {
		t.Fatalf("notify own comment: %v", err)
	}

	if len(repo.created) != 0 {
		t.Fatalf("created notifications = %d, want 0", len(repo.created))
	}
}

func TestPendingTopicNotificationFlushesAfterPublish(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-pending-comment", 9003, "topic", 104, 22, time.Now()); err != nil {
		t.Fatalf("notify pending comment: %v", err)
	}
	if len(repo.pending) != 1 {
		t.Fatalf("pending notifications = %d, want 1", len(repo.pending))
	}

	if err := svc.UpsertTopic(context.Background(), 104, 10, "发布后补投", time.Now()); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	if len(repo.pending) != 0 {
		t.Fatalf("pending notifications = %d, want 0", len(repo.pending))
	}
	if repo.created[0].UserID != 10 || repo.created[0].EntityType != "topic" {
		t.Fatalf("notification = %+v", repo.created[0])
	}
}

type pendingNotification struct {
	eventID          string
	notificationType string
	entityType       string
	entityID         int64
	actorID          int64
	sourceID         int64
	createdAt        time.Time
}

type memoryRepo struct {
	contents map[string]domain.ContentRef
	articles map[int64]domain.ArticleRef
	pending  []pendingNotification
	created  []domain.Notification
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		contents: map[string]domain.ContentRef{},
		articles: map[int64]domain.ArticleRef{},
	}
}

func (r *memoryRepo) EnsureSchema(context.Context) error { return nil }

func (r *memoryRepo) SaveArticle(_ context.Context, article domain.ArticleRef, _ time.Time) error {
	r.articles[article.ID] = article
	r.contents[contentKey("article", article.ID)] = domain.ContentRef{EntityType: "article", ID: article.ID, AuthorID: article.AuthorID, Title: article.Title}
	return nil
}

func (r *memoryRepo) GetArticle(_ context.Context, id int64) (domain.ArticleRef, error) {
	return r.articles[id], nil
}

func (r *memoryRepo) SavePendingArticleNotification(ctx context.Context, eventID, notificationType string, articleID, actorID, sourceID int64, createdAt time.Time) error {
	return r.SavePendingContentNotification(ctx, eventID, notificationType, "article", articleID, actorID, sourceID, createdAt)
}

func (r *memoryRepo) FlushPendingArticleNotifications(ctx context.Context, article domain.ArticleRef) error {
	content := domain.ContentRef{EntityType: "article", ID: article.ID, AuthorID: article.AuthorID, Title: article.Title}
	return r.FlushPendingContentNotifications(ctx, content)
}

func (r *memoryRepo) SaveContent(_ context.Context, content domain.ContentRef, _ time.Time) error {
	r.contents[contentKey(content.EntityType, content.ID)] = content
	return nil
}

func (r *memoryRepo) GetContent(_ context.Context, entityType string, id int64) (domain.ContentRef, error) {
	if content, ok := r.contents[contentKey(entityType, id)]; ok {
		return content, nil
	}
	if entityType == "article" {
		article := r.articles[id]
		return domain.ContentRef{EntityType: "article", ID: article.ID, AuthorID: article.AuthorID, Title: article.Title}, nil
	}
	return domain.ContentRef{}, nil
}

func (r *memoryRepo) SavePendingContentNotification(_ context.Context, eventID, notificationType, entityType string, entityID, actorID, sourceID int64, createdAt time.Time) error {
	r.pending = append(r.pending, pendingNotification{
		eventID:          eventID,
		notificationType: notificationType,
		entityType:       entityType,
		entityID:         entityID,
		actorID:          actorID,
		sourceID:         sourceID,
		createdAt:        createdAt,
	})
	return nil
}

func (r *memoryRepo) FlushPendingContentNotifications(_ context.Context, content domain.ContentRef) error {
	remaining := r.pending[:0]
	for _, pending := range r.pending {
		if pending.entityType != content.EntityType || pending.entityID != content.ID {
			remaining = append(remaining, pending)
			continue
		}
		if pending.actorID != content.AuthorID {
			r.created = append(r.created, domain.Notification{
				UserID:     content.AuthorID,
				Type:       pending.notificationType,
				Title:      contentTitle(content.EntityType, pending.notificationType),
				Content:    "pending",
				ActorID:    pending.actorID,
				EntityType: content.EntityType,
				EntityID:   content.ID,
				SourceID:   pending.sourceID,
			})
		}
	}
	r.pending = remaining
	return nil
}

func (r *memoryRepo) Create(_ context.Context, item domain.Notification, _ string, _ time.Time) error {
	r.created = append(r.created, item)
	return nil
}

func (r *memoryRepo) List(context.Context, int64, int32, int32, bool) ([]domain.Notification, int64, int64, error) {
	return nil, 0, 0, nil
}

func (r *memoryRepo) UnreadCount(context.Context, int64) (int64, error) { return 0, nil }
func (r *memoryRepo) MarkRead(context.Context, int64, int64) error      { return nil }
func (r *memoryRepo) MarkAllRead(context.Context, int64) error          { return nil }

func contentKey(entityType string, id int64) string {
	return entityType + ":" + stringID(id)
}

func stringID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func contentTitle(entityType, notificationType string) string {
	label := "文章"
	if entityType == "topic" {
		label = "话题"
	}
	if notificationType == "comment" {
		return label + "收到新评论"
	}
	return label + "收到互动"
}
