package notification

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	domain "notification-service/internal/domain/notification"
)

func TestNotifyTopicCommentCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:101"] = domain.ContentRef{EntityType: "topic", ID: 101, AuthorID: 10, Title: "社区路线图"}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-comment-topic", 9001, 0, "topic", 101, 22, time.Now()); err != nil {
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

func TestNotifyMallOrderStatusCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyMallOrderStatus(context.Background(), "evt-mall-shipped", true, 8801, 42, "MO202607080001", "顺丰", "SF1001", "已安排发货", time.Now()); err != nil {
		t.Fatalf("notify mall order status: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_order_shipped" || item.EntityType != "mall_order" || item.EntityID != 8801 || item.SourceID != 8801 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "订单已发货" {
		t.Fatalf("title = %q", item.Title)
	}
	if item.Content == "" || !strings.Contains(item.Content, "顺丰 / SF1001") {
		t.Fatalf("content = %q", item.Content)
	}
}

func TestNotifyMallOrderPaidCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyMallOrderPaid(context.Background(), "evt-mall-paid", 8802, 42, 360, "MO202607080002", "credits", time.Now()); err != nil {
		t.Fatalf("notify mall order paid: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_order_paid" || item.EntityType != "mall_order" || item.EntityID != 8802 || item.SourceID != 8802 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "订单已支付" {
		t.Fatalf("title = %q", item.Title)
	}
	if !strings.Contains(item.Content, "360 积分") || !strings.Contains(item.Content, "credits") {
		t.Fatalf("content = %q", item.Content)
	}
}

func TestNotifyMallProductReviewStatusCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyMallProductReviewStatus(context.Background(), "evt-review-published", true, 9901, 8801, 7701, 42, "主题皮肤", time.Now()); err != nil {
		t.Fatalf("notify mall product review status: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_review_published" || item.EntityType != "mall_product" || item.EntityID != 8801 || item.SourceID != 9901 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "商品评价已展示" {
		t.Fatalf("title = %q", item.Title)
	}
	if !strings.Contains(item.Content, "主题皮肤") || !strings.Contains(item.Content, "订单 #7701") {
		t.Fatalf("content = %q", item.Content)
	}
}

func TestNotifyOwnContentDoesNotCreateNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:103"] = domain.ContentRef{EntityType: "topic", ID: 103, AuthorID: 10, Title: "自评测试"}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-own-comment", 9002, 0, "topic", 103, 10, time.Now()); err != nil {
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

	if err := svc.NotifyComment(context.Background(), "evt-pending-comment", 9003, 0, "topic", 104, 22, time.Now()); err != nil {
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

func TestNotifyReplyCreatesNotificationForParentCommentAuthor(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:105"] = domain.ContentRef{EntityType: "topic", ID: 105, AuthorID: 10, Title: "回复提醒"}
	repo.comments[7001] = domain.CommentRef{ID: 7001, EntityType: "topic", EntityID: 105, AuthorID: 20}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-reply-topic", 7002, 7001, "topic", 105, 30, time.Now()); err != nil {
		t.Fatalf("notify reply: %v", err)
	}

	if len(repo.created) != 2 {
		t.Fatalf("created notifications = %d, want 2", len(repo.created))
	}
	reply := findNotification(repo.created, 20, "reply")
	if reply == nil {
		t.Fatalf("reply notification not found: %+v", repo.created)
	}
	if reply.Title != "评论收到回复" || reply.EntityType != "topic" || reply.EntityID != 105 || reply.SourceID != 7002 {
		t.Fatalf("reply notification = %+v", *reply)
	}
}

func TestNotifyReplyToOwnCommentSkipsReplyNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:106"] = domain.ContentRef{EntityType: "topic", ID: 106, AuthorID: 10, Title: "自回复"}
	repo.comments[7101] = domain.CommentRef{ID: 7101, EntityType: "topic", EntityID: 106, AuthorID: 30}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-reply-own", 7102, 7101, "topic", 106, 30, time.Now()); err != nil {
		t.Fatalf("notify own reply: %v", err)
	}

	if findNotification(repo.created, 30, "reply") != nil {
		t.Fatalf("own reply notification should be skipped: %+v", repo.created)
	}
	if findNotification(repo.created, 10, "comment") == nil {
		t.Fatalf("content author notification should still be created: %+v", repo.created)
	}
}

func TestPendingReplyNotificationFlushesAfterParentCommentArrives(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.contents["topic:107"] = domain.ContentRef{EntityType: "topic", ID: 107, AuthorID: 10, Title: "乱序回复"}
	svc := NewService(repo)

	if err := svc.NotifyComment(context.Background(), "evt-pending-reply", 7202, 7201, "topic", 107, 30, time.Now()); err != nil {
		t.Fatalf("notify pending reply: %v", err)
	}
	if len(repo.pendingReplies) != 1 {
		t.Fatalf("pending replies = %d, want 1", len(repo.pendingReplies))
	}

	if err := svc.NotifyComment(context.Background(), "evt-parent-comment", 7201, 0, "topic", 107, 20, time.Now()); err != nil {
		t.Fatalf("notify parent comment: %v", err)
	}

	if len(repo.pendingReplies) != 0 {
		t.Fatalf("pending replies = %d, want 0", len(repo.pendingReplies))
	}
	reply := findNotification(repo.created, 20, "reply")
	if reply == nil || reply.SourceID != 7202 {
		t.Fatalf("flushed reply notification = %+v", reply)
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

type pendingReplyNotification struct {
	eventID         string
	parentCommentID int64
	commentID       int64
	entityType      string
	entityID        int64
	actorID         int64
	createdAt       time.Time
}

type memoryRepo struct {
	contents       map[string]domain.ContentRef
	articles       map[int64]domain.ArticleRef
	comments       map[int64]domain.CommentRef
	pending        []pendingNotification
	pendingReplies []pendingReplyNotification
	created        []domain.Notification
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		contents: map[string]domain.ContentRef{},
		articles: map[int64]domain.ArticleRef{},
		comments: map[int64]domain.CommentRef{},
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

func (r *memoryRepo) SaveComment(_ context.Context, comment domain.CommentRef, _ time.Time) error {
	r.comments[comment.ID] = comment
	return nil
}

func (r *memoryRepo) GetComment(_ context.Context, id int64) (domain.CommentRef, error) {
	return r.comments[id], nil
}

func (r *memoryRepo) SavePendingReplyNotification(_ context.Context, eventID string, parentCommentID, commentID int64, entityType string, entityID, actorID int64, createdAt time.Time) error {
	r.pendingReplies = append(r.pendingReplies, pendingReplyNotification{
		eventID:         eventID,
		parentCommentID: parentCommentID,
		commentID:       commentID,
		entityType:      entityType,
		entityID:        entityID,
		actorID:         actorID,
		createdAt:       createdAt,
	})
	return nil
}

func (r *memoryRepo) FlushPendingReplyNotifications(_ context.Context, parent domain.CommentRef) error {
	remaining := r.pendingReplies[:0]
	for _, pending := range r.pendingReplies {
		if pending.parentCommentID != parent.ID {
			remaining = append(remaining, pending)
			continue
		}
		if pending.actorID != parent.AuthorID {
			r.created = append(r.created, domain.Notification{
				UserID:     parent.AuthorID,
				Type:       "reply",
				Title:      "评论收到回复",
				Content:    "pending reply",
				ActorID:    pending.actorID,
				EntityType: pending.entityType,
				EntityID:   pending.entityID,
				SourceID:   pending.commentID,
			})
		}
	}
	r.pendingReplies = remaining
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

func findNotification(items []domain.Notification, userID int64, notificationType string) *domain.Notification {
	for i := range items {
		if items[i].UserID == userID && items[i].Type == notificationType {
			return &items[i]
		}
	}
	return nil
}
