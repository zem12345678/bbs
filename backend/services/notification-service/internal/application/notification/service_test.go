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

func TestNotifyFollowRequestNotifications(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		notify     func(*Service, context.Context, string, int64, int64, time.Time) error
		wantUserID int64
		wantType   string
		wantActor  int64
		wantTitle  string
	}{
		{
			name:       "received",
			notify:     (*Service).NotifyFollowRequestReceived,
			wantUserID: 22,
			wantType:   domain.NotificationTypeFollowRequestReceived,
			wantActor:  11,
			wantTitle:  "收到关注申请",
		},
		{
			name:       "accepted",
			notify:     (*Service).NotifyFollowRequestAccepted,
			wantUserID: 11,
			wantType:   domain.NotificationTypeFollowRequestAccepted,
			wantActor:  22,
			wantTitle:  "关注申请已通过",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMemoryRepo()
			svc := NewService(repo)
			if err := tt.notify(svc, context.Background(), "evt-follow-request-"+tt.name, 11, 22, time.Now()); err != nil {
				t.Fatalf("notify follow request: %v", err)
			}
			if len(repo.created) != 1 {
				t.Fatalf("created notifications = %d, want 1", len(repo.created))
			}
			item := repo.created[0]
			if item.UserID != tt.wantUserID || item.Type != tt.wantType || item.ActorID != tt.wantActor || item.EntityType != "user" || item.EntityID != tt.wantActor || item.SourceID != tt.wantActor {
				t.Fatalf("notification = %+v", item)
			}
			if item.Title != tt.wantTitle || !strings.Contains(item.Content, strconv.FormatInt(tt.wantActor, 10)) {
				t.Fatalf("notification copy = title %q content %q", item.Title, item.Content)
			}
		})
	}

	repo := newMemoryRepo()
	svc := NewService(repo)
	for _, pair := range [][2]int64{{0, 22}, {11, 0}, {11, 11}} {
		if err := svc.NotifyFollowRequestReceived(context.Background(), "evt-invalid", pair[0], pair[1], time.Now()); err != nil {
			t.Fatalf("invalid received notification: %v", err)
		}
		if err := svc.NotifyFollowRequestAccepted(context.Background(), "evt-invalid", pair[0], pair[1], time.Now()); err != nil {
			t.Fatalf("invalid accepted notification: %v", err)
		}
	}
	if len(repo.created) != 0 {
		t.Fatalf("invalid participants created %d notifications", len(repo.created))
	}
}

func TestNotificationPreferencesDefaultAndUpdate(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	svc := NewService(repo)

	items, err := svc.GetNotificationPreferences(context.Background(), 42)
	if err != nil {
		t.Fatalf("get default preferences: %v", err)
	}
	if len(items) == 0 || !items[0].Enabled {
		t.Fatalf("default preferences = %+v, want enabled defaults", items)
	}

	updated, err := svc.UpdateNotificationPreferences(context.Background(), 42, []domain.NotificationPreference{{Type: domain.NotificationTypeComment, Enabled: false}})
	if err != nil {
		t.Fatalf("update preferences: %v", err)
	}
	var commentEnabled bool
	for _, item := range updated {
		if item.Type == domain.NotificationTypeComment {
			commentEnabled = item.Enabled
		}
	}
	if commentEnabled {
		t.Fatalf("updated comment preference = enabled, want disabled")
	}
	if _, err := svc.UpdateNotificationPreferences(context.Background(), 42, []domain.NotificationPreference{{Type: "unknown", Enabled: false}}); !errors.Is(err, domain.ErrInvalidNotificationPreferences) {
		t.Fatalf("invalid preference error = %v", err)
	}
}

func TestListCompatibilityNormalizesFiltersAndPreservesCursor(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	repo.compatibilityItems = []domain.Notification{{ID: 12, UserID: 42, Type: domain.NotificationTypeFollow}}
	service := NewService(repo)
	items, err := service.ListCompatibility(t.Context(), domain.NotificationCompatibilityQuery{
		UserID:          42,
		Limit:           10,
		SinceID:         11,
		IncludeTypesSet: true,
		IncludeTypes:    []string{" follow ", "follow", ""},
		ExcludeTypesSet: true,
		ExcludeTypes:    []string{"comment"},
	})
	if err != nil {
		t.Fatalf("ListCompatibility() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 12 {
		t.Fatalf("items = %+v", items)
	}
	if repo.compatibilityQuery == nil {
		t.Fatal("repository query was not recorded")
	}
	query := repo.compatibilityQuery
	if query.SinceID != 11 || !query.IncludeTypesSet || len(query.IncludeTypes) != 1 || query.IncludeTypes[0] != domain.NotificationTypeFollow {
		t.Fatalf("query = %+v", query)
	}
	if len(query.ExcludeTypes) != 1 || query.ExcludeTypes[0] != domain.NotificationTypeComment {
		t.Fatalf("query exclusions = %+v", query.ExcludeTypes)
	}
}

func TestListCompatibilityEmptyIncludeAndInvalidRequest(t *testing.T) {
	t.Parallel()

	repo := newMemoryRepo()
	service := NewService(repo)
	items, err := service.ListCompatibility(t.Context(), domain.NotificationCompatibilityQuery{UserID: 42, Limit: 10, IncludeTypesSet: true})
	if err != nil {
		t.Fatalf("empty include: %v", err)
	}
	if len(items) != 0 || repo.compatibilityQuery != nil {
		t.Fatalf("empty include = items:%+v query:%+v", items, repo.compatibilityQuery)
	}
	if _, err := service.ListCompatibility(t.Context(), domain.NotificationCompatibilityQuery{UserID: 42, Limit: 101}); !errors.Is(err, domain.ErrInvalidNotificationQuery) {
		t.Fatalf("invalid limit error = %v", err)
	}
	if _, err := service.ListCompatibility(t.Context(), domain.NotificationCompatibilityQuery{UserID: 0, Limit: 10}); !errors.Is(err, domain.ErrInvalidNotificationQuery) {
		t.Fatalf("invalid user error = %v", err)
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

func TestCreateExportCompletedNotificationCreatesIdempotentFileNotification(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)
	command := domain.ExportCompletedNotificationCommand{
		RecipientID:    42,
		FileID:         9001,
		ExportedEntity: " clip ",
		IdempotencyKey: " export-42-9001 ",
	}

	if err := service.CreateExportCompletedNotification(t.Context(), command); err != nil {
		t.Fatalf("CreateExportCompletedNotification() error = %v", err)
	}
	if err := service.CreateExportCompletedNotification(t.Context(), command); err != nil {
		t.Fatalf("retry CreateExportCompletedNotification() error = %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	got := repo.created[0]
	if got.UserID != 42 || got.Type != domain.NotificationTypeExportCompleted || got.EntityType != "file" || got.EntityID != 9001 || got.SourceID != 9001 || got.ActorID != 0 {
		t.Fatalf("notification = %#v", got)
	}
	if got.Title == "" || got.Content == "" {
		t.Fatalf("notification copy = title %q content %q", got.Title, got.Content)
	}
	if len(repo.createdEventIDs) != 1 {
		t.Fatalf("created event IDs = %#v", repo.createdEventIDs)
	}
	if _, ok := repo.createdEventIDs["42:export_completed:export-42-9001"]; !ok {
		t.Fatalf("created event IDs = %#v", repo.createdEventIDs)
	}
}

func TestCreateExportCompletedNotificationUsesAntennaCopy(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)

	err := service.CreateExportCompletedNotification(t.Context(), domain.ExportCompletedNotificationCommand{
		RecipientID: 42, FileID: 9002, ExportedEntity: domain.ExportCompletedEntityAntenna, IdempotencyKey: "antenna-42-9002",
	})

	if err != nil {
		t.Fatalf("CreateExportCompletedNotification() error = %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created notifications = %d, want 1", len(repo.created))
	}
	got := repo.created[0]
	if got.Title != "天线导出完成" || !strings.Contains(got.Content, "天线导出") || got.EntityID != 9002 {
		t.Fatalf("notification = %#v", got)
	}
}

func TestCreateExportCompletedNotificationUsesSafetyExportCopy(t *testing.T) {
	tests := []struct {
		entity string
		title  string
	}{
		{entity: domain.ExportCompletedEntityBlocking, title: "屏蔽列表导出完成"},
		{entity: domain.ExportCompletedEntityMuting, title: "静音列表导出完成"},
	}
	for _, test := range tests {
		t.Run(test.entity, func(t *testing.T) {
			repo := newMemoryRepo()
			service := NewService(repo)

			err := service.CreateExportCompletedNotification(t.Context(), domain.ExportCompletedNotificationCommand{
				RecipientID: 42, FileID: 9003, ExportedEntity: test.entity, IdempotencyKey: test.entity + "-42-9003",
			})

			if err != nil {
				t.Fatalf("CreateExportCompletedNotification() error = %v", err)
			}
			if len(repo.created) != 1 || repo.created[0].Title != test.title || repo.created[0].EntityID != 9003 {
				t.Fatalf("created notifications = %#v", repo.created)
			}
		})
	}
}

func TestCreateExportCompletedNotificationUsesRelationshipExportCopy(t *testing.T) {
	tests := []struct {
		entity string
		title  string
	}{
		{entity: domain.ExportCompletedEntityFollowing, title: "关注列表导出完成"},
		{entity: domain.ExportCompletedEntityUserList, title: "用户列表导出完成"},
	}
	for _, test := range tests {
		t.Run(test.entity, func(t *testing.T) {
			repo := newMemoryRepo()
			service := NewService(repo)

			err := service.CreateExportCompletedNotification(t.Context(), domain.ExportCompletedNotificationCommand{
				RecipientID: 42, FileID: 9004, ExportedEntity: test.entity, IdempotencyKey: test.entity + "-42-9004",
			})

			if err != nil {
				t.Fatalf("CreateExportCompletedNotification() error = %v", err)
			}
			if len(repo.created) != 1 || repo.created[0].Title != test.title || repo.created[0].EntityID != 9004 {
				t.Fatalf("created notifications = %#v", repo.created)
			}
		})
	}
}

func TestCreateExportCompletedNotificationUsesFavoriteExportCopy(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)
	err := service.CreateExportCompletedNotification(t.Context(), domain.ExportCompletedNotificationCommand{
		RecipientID: 42, FileID: 9005, ExportedEntity: domain.ExportCompletedEntityFavorite, IdempotencyKey: "favorite-42-9005",
	})
	if err != nil {
		t.Fatalf("CreateExportCompletedNotification() error = %v", err)
	}
	if len(repo.created) != 1 || repo.created[0].Title != "收藏导出完成" || repo.created[0].EntityID != 9005 {
		t.Fatalf("created notifications = %#v", repo.created)
	}
}

func TestCreateExportCompletedNotificationUsesNoteExportCopy(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)
	err := service.CreateExportCompletedNotification(t.Context(), domain.ExportCompletedNotificationCommand{
		RecipientID: 42, FileID: 9006, ExportedEntity: domain.ExportCompletedEntityNote, IdempotencyKey: "note-42-9006",
	})
	if err != nil {
		t.Fatalf("CreateExportCompletedNotification() error = %v", err)
	}
	if len(repo.created) != 1 || repo.created[0].Title != "内容导出完成" || !strings.Contains(repo.created[0].Content, "内容导出") || repo.created[0].EntityID != 9006 {
		t.Fatalf("created notifications = %#v", repo.created)
	}
}

func TestCreateExportCompletedNotificationUsesAccountDataExportCopy(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)
	err := service.CreateExportCompletedNotification(t.Context(), domain.ExportCompletedNotificationCommand{
		RecipientID: 42, FileID: 9007, ExportedEntity: domain.ExportCompletedEntityData, IdempotencyKey: "data-42-9007",
	})
	if err != nil {
		t.Fatalf("CreateExportCompletedNotification() error = %v", err)
	}
	if len(repo.created) != 1 || repo.created[0].Title != "账户数据导出完成" || !strings.Contains(repo.created[0].Content, "账户数据归档") || repo.created[0].EntityID != 9007 {
		t.Fatalf("created notifications = %#v", repo.created)
	}
}

func TestCreateExportCompletedNotificationRejectsInvalidInput(t *testing.T) {
	valid := domain.ExportCompletedNotificationCommand{
		RecipientID:    42,
		FileID:         9001,
		ExportedEntity: domain.ExportCompletedEntityClip,
		IdempotencyKey: "export-42-9001",
	}
	tests := []struct {
		name   string
		mutate func(*domain.ExportCompletedNotificationCommand)
	}{
		{name: "invalid recipient", mutate: func(command *domain.ExportCompletedNotificationCommand) { command.RecipientID = 0 }},
		{name: "invalid file", mutate: func(command *domain.ExportCompletedNotificationCommand) { command.FileID = 0 }},
		{name: "unsupported entity", mutate: func(command *domain.ExportCompletedNotificationCommand) { command.ExportedEntity = "unsupported" }},
		{name: "empty idempotency key", mutate: func(command *domain.ExportCompletedNotificationCommand) { command.IdempotencyKey = " " }},
		{name: "long idempotency key", mutate: func(command *domain.ExportCompletedNotificationCommand) {
			command.IdempotencyKey = strings.Repeat("k", domain.ExportCompletedNotificationMaxIdempotencyKey+1)
		}},
		{name: "nul idempotency key", mutate: func(command *domain.ExportCompletedNotificationCommand) { command.IdempotencyKey = "bad\x00key" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemoryRepo()
			service := NewService(repo)
			command := valid
			tt.mutate(&command)
			err := service.CreateExportCompletedNotification(t.Context(), command)
			if !errors.Is(err, domain.ErrInvalidExportCompletedNotification) {
				t.Fatalf("CreateExportCompletedNotification() error = %v, want ErrInvalidExportCompletedNotification", err)
			}
			if len(repo.created) != 0 {
				t.Fatalf("repository should not be called, got %#v", repo.created)
			}
		})
	}
}

func TestEraseUserDataValidatesAndDelegates(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo)

	if err := service.EraseUserData(t.Context(), 42, 9001, 3); err != nil {
		t.Fatalf("EraseUserData() error = %v", err)
	}
	if len(repo.erasures) != 1 || repo.erasures[0].userID != 42 || repo.erasures[0].jobID != 9001 || repo.erasures[0].policyVersion != 3 {
		t.Fatalf("erasures = %#v", repo.erasures)
	}

	for _, request := range []userErasure{
		{userID: 0, jobID: 9001, policyVersion: 3},
		{userID: 42, jobID: 0, policyVersion: 3},
		{userID: 42, jobID: 9001, policyVersion: 0},
	} {
		if err := service.EraseUserData(t.Context(), request.userID, request.jobID, request.policyVersion); !errors.Is(err, domain.ErrInvalidUserErasure) {
			t.Fatalf("EraseUserData(%d, %d, %d) error = %v, want ErrInvalidUserErasure", request.userID, request.jobID, request.policyVersion, err)
		}
	}
	if len(repo.erasures) != 1 {
		t.Fatalf("invalid requests reached repository: %#v", repo.erasures)
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

type userErasure struct {
	userID        int64
	jobID         int64
	policyVersion int32
}

type memoryRepo struct {
	contents           map[string]domain.ContentRef
	articles           map[int64]domain.ArticleRef
	comments           map[int64]domain.CommentRef
	pending            []pendingNotification
	pendingReplies     []pendingReplyNotification
	created            []domain.Notification
	createdEventIDs    map[string]struct{}
	systemCommands     []domain.SystemNotificationCommand
	systemKeys         map[string]struct{}
	systemErr          error
	preferences        []domain.NotificationPreference
	erasures           []userErasure
	compatibilityItems []domain.Notification
	compatibilityQuery *domain.NotificationCompatibilityQuery
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		contents:        map[string]domain.ContentRef{},
		articles:        map[int64]domain.ArticleRef{},
		comments:        map[int64]domain.CommentRef{},
		createdEventIDs: map[string]struct{}{},
		systemKeys:      map[string]struct{}{},
	}
}

func (r *memoryRepo) EnsureSchema(context.Context) error { return nil }

func (r *memoryRepo) EraseUserData(_ context.Context, userID, jobID int64, policyVersion int32) error {
	r.erasures = append(r.erasures, userErasure{userID: userID, jobID: jobID, policyVersion: policyVersion})
	return nil
}

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

func (r *memoryRepo) Create(_ context.Context, item domain.Notification, sourceEventID string, _ time.Time) error {
	eventKey := stringID(item.UserID) + ":" + sourceEventID
	if _, exists := r.createdEventIDs[eventKey]; exists {
		return nil
	}
	r.createdEventIDs[eventKey] = struct{}{}
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

func (r *memoryRepo) ListPreferences(context.Context, int64) ([]domain.NotificationPreference, error) {
	return append([]domain.NotificationPreference(nil), r.preferences...), nil
}

func (r *memoryRepo) ReplacePreferences(_ context.Context, _ int64, preferences []domain.NotificationPreference) error {
	r.preferences = append([]domain.NotificationPreference(nil), preferences...)
	return nil
}

func (r *memoryRepo) List(context.Context, int64, int32, int32, bool) ([]domain.Notification, int64, int64, error) {
	return nil, 0, 0, nil
}

func (r *memoryRepo) ListCompatibility(_ context.Context, query domain.NotificationCompatibilityQuery) ([]domain.Notification, error) {
	copyQuery := query
	copyQuery.IncludeTypes = append([]string(nil), query.IncludeTypes...)
	copyQuery.ExcludeTypes = append([]string(nil), query.ExcludeTypes...)
	r.compatibilityQuery = &copyQuery
	return append([]domain.Notification(nil), r.compatibilityItems...), nil
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
