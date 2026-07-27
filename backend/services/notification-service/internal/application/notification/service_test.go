package notification

import (
	"context"
	"errors"
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

func TestNotifyQAAcceptedCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyQAAccepted(context.Background(), "content.qa.accepted:101:9001", 101, "如何排查回调？", 10, 9001, 22, 10, time.Now()); err != nil {
		t.Fatalf("notify qa accepted: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 22 || item.Type != "qa_answer_accepted" || item.EntityType != "topic" || item.EntityID != 101 || item.SourceID != 9001 || item.ActorID != 10 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "回答被采纳" {
		t.Fatalf("title = %q", item.Title)
	}
	if !strings.Contains(item.Content, "如何排查回调") || !strings.Contains(item.Content, "10 积分") {
		t.Fatalf("content = %q", item.Content)
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

	if err := svc.NotifyMallOrderPaid(context.Background(), "evt-mall-paid", 8802, 42, 360, "MO202607080002", "credits", []MallDigitalEntitlement{{Title: "创始会员徽章", GrantType: "badge", GrantKey: "badge-founder", FulfillmentCode: "BBS-ENTITLEMENT"}}, time.Now()); err != nil {
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
	if !strings.Contains(item.Content, "360 积分") || !strings.Contains(item.Content, "credits") || !strings.Contains(item.Content, "BBS-ENTITLEMENT") {
		t.Fatalf("content = %q", item.Content)
	}
}

func TestNotifyMallRefundApprovedIncludesDigitalEntitlementRevocation(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyMallRefund(context.Background(), "evt-mall-refund", true, 9902, 8803, 42, 120, "MO202607080003", "digital_refund", "退款通过", []MallDigitalEntitlement{
		{
			ProductID:       101,
			SKU:             "BADGE-FOUNDER",
			Title:           "创始会员徽章",
			Quantity:        1,
			FulfillmentCode: "BBS-ENTITLEMENT",
			GrantType:       "badge",
			GrantKey:        "badge-founder",
			Status:          "REVOKED",
			RefundID:        9902,
		},
	}, time.Now()); err != nil {
		t.Fatalf("notify mall refund: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_refund_approved" || item.EntityType != "mall_order" || item.EntityID != 8803 || item.SourceID != 9902 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "售后退款已通过" {
		t.Fatalf("title = %q", item.Title)
	}
	for _, expected := range []string{"数字权益已撤销", "创始会员徽章", "badge-founder", "BBS-ENTITLEMENT", "退款通过"} {
		if !strings.Contains(item.Content, expected) {
			t.Fatalf("content = %q, want %q", item.Content, expected)
		}
	}
}

func TestNotifyMallDigitalEntitlementRevokedCreatesNotification(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	if err := svc.NotifyMallDigitalEntitlementRevoked(context.Background(), "evt-entitlement-revoked", 503, 8804, 42, "MO202607140001", "admin-7", "risk review", MallDigitalEntitlement{
		ProductID:       101,
		SKU:             "VIP-MONTH",
		Title:           "会员月卡",
		FulfillmentCode: "BBS-VIP-503",
		GrantType:       "membership",
		GrantKey:        "vip-month",
		Status:          "REVOKED",
	}, time.Now()); err != nil {
		t.Fatalf("notify mall entitlement revoked: %v", err)
	}

	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	item := repo.created[0]
	if item.UserID != 42 || item.Type != "mall_digital_entitlement_revoked" || item.EntityType != "mall_order" || item.EntityID != 8804 || item.SourceID != 503 {
		t.Fatalf("notification = %+v", item)
	}
	if item.Title != "数字权益已撤销" {
		t.Fatalf("title = %q", item.Title)
	}
	for _, expected := range []string{"MO202607140001", "会员月卡", "vip-month", "BBS-VIP-503", "risk review", "admin-7"} {
		if !strings.Contains(item.Content, expected) {
			t.Fatalf("content = %q, want %q", item.Content, expected)
		}
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

func TestDispatchSystemNotificationsNormalizesRecipientsAndIsIdempotent(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)
	command := domain.SystemNotificationCommand{
		RecipientIDs:   []int64{11, 22, 11},
		Title:          "  系统维护  ",
		Content:        "  今晚 23:00 维护  ",
		ActorID:        7,
		IdempotencyKey: "  maintenance-20260725  ",
	}

	delivered, err := service.DispatchSystemNotifications(t.Context(), command)
	if err != nil {
		t.Fatalf("DispatchSystemNotifications() error = %v", err)
	}
	if delivered != 2 {
		t.Fatalf("delivered = %d, want 2", delivered)
	}
	if len(repo.systemCommands) != 1 {
		t.Fatalf("system command count = %d, want 1", len(repo.systemCommands))
	}
	got := repo.systemCommands[0]
	if got.Title != "系统维护" || got.Content != "今晚 23:00 维护" || got.IdempotencyKey != "maintenance-20260725" || got.ActorID != 7 {
		t.Fatalf("normalized command = %#v", got)
	}
	if len(got.RecipientIDs) != 2 || got.RecipientIDs[0] != 11 || got.RecipientIDs[1] != 22 {
		t.Fatalf("recipient ids = %v, want [11 22]", got.RecipientIDs)
	}

	delivered, err = service.DispatchSystemNotifications(t.Context(), command)
	if err != nil {
		t.Fatalf("retry DispatchSystemNotifications() error = %v", err)
	}
	if delivered != 0 || len(repo.created) != 2 {
		t.Fatalf("retry delivered = %d, created = %d; want 0 and 2", delivered, len(repo.created))
	}
}

func TestDispatchSystemNotificationsRejectsInvalidInput(t *testing.T) {
	valid := domain.SystemNotificationCommand{
		RecipientIDs:   []int64{11},
		Title:          "系统通知",
		Content:        "请查看详情",
		ActorID:        7,
		IdempotencyKey: "notice-1",
	}
	tooManyRecipients := make([]int64, domain.SystemNotificationMaxRecipients+1)
	for i := range tooManyRecipients {
		tooManyRecipients[i] = int64(i + 1)
	}
	tests := []struct {
		name   string
		mutate func(*domain.SystemNotificationCommand)
	}{
		{name: "empty recipients", mutate: func(command *domain.SystemNotificationCommand) { command.RecipientIDs = nil }},
		{name: "non-positive recipient", mutate: func(command *domain.SystemNotificationCommand) { command.RecipientIDs = []int64{11, 0} }},
		{name: "too many recipients", mutate: func(command *domain.SystemNotificationCommand) { command.RecipientIDs = tooManyRecipients }},
		{name: "empty title", mutate: func(command *domain.SystemNotificationCommand) { command.Title = "  " }},
		{name: "long title", mutate: func(command *domain.SystemNotificationCommand) {
			command.Title = strings.Repeat("标题", domain.SystemNotificationMaxTitleRunes)
		}},
		{name: "empty content", mutate: func(command *domain.SystemNotificationCommand) { command.Content = "  " }},
		{name: "long content", mutate: func(command *domain.SystemNotificationCommand) {
			command.Content = strings.Repeat("内容", domain.SystemNotificationMaxContentRunes)
		}},
		{name: "missing idempotency key", mutate: func(command *domain.SystemNotificationCommand) { command.IdempotencyKey = " " }},
		{name: "long idempotency key", mutate: func(command *domain.SystemNotificationCommand) {
			command.IdempotencyKey = strings.Repeat("k", domain.SystemNotificationMaxIdempotencyKey+1)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemoryRepo()
			service := NewService(repo)
			command := valid
			command.RecipientIDs = append([]int64(nil), valid.RecipientIDs...)
			tt.mutate(&command)
			_, err := service.DispatchSystemNotifications(t.Context(), command)
			if !errors.Is(err, domain.ErrInvalidSystemNotification) {
				t.Fatalf("DispatchSystemNotifications() error = %v, want ErrInvalidSystemNotification", err)
			}
			if len(repo.systemCommands) != 0 {
				t.Fatalf("repository should not be called, got %#v", repo.systemCommands)
			}
		})
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
	systemCommands []domain.SystemNotificationCommand
	systemKeys     map[string]struct{}
	systemErr      error
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		contents:   map[string]domain.ContentRef{},
		articles:   map[int64]domain.ArticleRef{},
		comments:   map[int64]domain.CommentRef{},
		systemKeys: map[string]struct{}{},
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

func (r *memoryRepo) CreateSystemNotifications(_ context.Context, command domain.SystemNotificationCommand, _ time.Time) (int32, error) {
	if r.systemErr != nil {
		return 0, r.systemErr
	}
	r.systemCommands = append(r.systemCommands, command)
	var delivered int32
	for _, recipientID := range command.RecipientIDs {
		key := stringID(recipientID) + ":" + stringID(command.ActorID) + ":" + command.IdempotencyKey
		if _, ok := r.systemKeys[key]; ok {
			continue
		}
		r.systemKeys[key] = struct{}{}
		r.created = append(r.created, domain.Notification{
			UserID:  recipientID,
			Type:    domain.SystemNotificationType,
			Title:   command.Title,
			Content: command.Content,
			ActorID: command.ActorID,
		})
		delivered++
	}
	return delivered, nil
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
