package notification

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	domain "notification-service/internal/domain/notification"
)

type Service struct {
	repo domain.Repository
}

type MallDigitalEntitlement struct {
	ProductID       int64
	SKU             string
	Title           string
	Quantity        int32
	FulfillmentCode string
	GrantType       string
	GrantKey        string
	Status          string
	RefundID        int64
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

func (s *Service) EraseUserData(ctx context.Context, userID, deletionJobID int64, policyVersion int32) error {
	if userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return domain.ErrInvalidUserErasure
	}
	return s.repo.EraseUserData(ctx, userID, deletionJobID, policyVersion)
}

func (s *Service) DispatchSystemNotifications(ctx context.Context, command domain.SystemNotificationCommand) (int32, error) {
	command, err := normalizeSystemNotificationCommand(command)
	if err != nil {
		return 0, err
	}
	return s.repo.CreateSystemNotifications(ctx, command, time.Now())
}

func normalizeSystemNotificationCommand(command domain.SystemNotificationCommand) (domain.SystemNotificationCommand, error) {
	if command.ActorID <= 0 || len(command.RecipientIDs) == 0 || len(command.RecipientIDs) > domain.SystemNotificationMaxRecipients {
		return domain.SystemNotificationCommand{}, domain.ErrInvalidSystemNotification
	}

	recipients := make([]int64, 0, len(command.RecipientIDs))
	seen := make(map[int64]struct{}, len(command.RecipientIDs))
	for _, recipientID := range command.RecipientIDs {
		if recipientID <= 0 {
			return domain.SystemNotificationCommand{}, domain.ErrInvalidSystemNotification
		}
		if _, ok := seen[recipientID]; ok {
			continue
		}
		seen[recipientID] = struct{}{}
		recipients = append(recipients, recipientID)
	}

	command.Title = strings.TrimSpace(command.Title)
	command.Content = strings.TrimSpace(command.Content)
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.Title == "" || command.Content == "" || command.IdempotencyKey == "" ||
		utf8.RuneCountInString(command.Title) > domain.SystemNotificationMaxTitleRunes ||
		utf8.RuneCountInString(command.Content) > domain.SystemNotificationMaxContentRunes ||
		utf8.RuneCountInString(command.IdempotencyKey) > domain.SystemNotificationMaxIdempotencyKey ||
		strings.ContainsRune(command.Title, '\x00') || strings.ContainsRune(command.Content, '\x00') || strings.ContainsRune(command.IdempotencyKey, '\x00') {
		return domain.SystemNotificationCommand{}, domain.ErrInvalidSystemNotification
	}
	command.RecipientIDs = recipients
	return command, nil
}

func (s *Service) UpsertArticle(ctx context.Context, articleID, authorID int64, title string, publishedAt time.Time) error {
	article := domain.ArticleRef{ID: articleID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveArticle(ctx, article, publishedAt); err != nil {
		return err
	}
	if err := s.upsertContent(ctx, "article", articleID, authorID, title, publishedAt); err != nil {
		return err
	}
	return s.repo.FlushPendingArticleNotifications(ctx, article)
}

func (s *Service) UpsertTopic(ctx context.Context, topicID, authorID int64, title string, publishedAt time.Time) error {
	return s.upsertContent(ctx, "topic", topicID, authorID, title, publishedAt)
}

func (s *Service) upsertContent(ctx context.Context, entityType string, entityID, authorID int64, title string, publishedAt time.Time) error {
	content := domain.ContentRef{EntityType: entityType, ID: entityID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveContent(ctx, content, publishedAt); err != nil {
		return err
	}
	return s.repo.FlushPendingContentNotifications(ctx, content)
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

func (s *Service) NotifyComment(ctx context.Context, eventID string, commentID, parentID int64, entityType string, entityID, actorID int64, occurredAt time.Time) error {
	if commentID <= 0 || entityID <= 0 || actorID <= 0 || !supportedContentType(entityType) {
		return nil
	}
	comment := domain.CommentRef{ID: commentID, EntityType: entityType, EntityID: entityID, AuthorID: actorID, ParentID: parentID}
	if err := s.repo.SaveComment(ctx, comment, occurredAt); err != nil {
		return err
	}
	if err := s.notifyContentComment(ctx, eventID, commentID, entityType, entityID, actorID, occurredAt); err != nil {
		return err
	}
	if err := s.notifyReply(ctx, eventID, commentID, parentID, entityType, entityID, actorID, occurredAt); err != nil {
		return err
	}
	return s.repo.FlushPendingReplyNotifications(ctx, comment)
}

func (s *Service) notifyContentComment(ctx context.Context, eventID string, commentID int64, entityType string, entityID, actorID int64, occurredAt time.Time) error {
	content, err := s.repo.GetContent(ctx, entityType, entityID)
	if err != nil {
		return err
	}
	if content.AuthorID <= 0 {
		return s.repo.SavePendingContentNotification(ctx, eventID, "comment", entityType, entityID, actorID, commentID, occurredAt)
	}
	if content.AuthorID == actorID {
		return nil
	}
	label := contentLabel(entityType)
	return s.repo.Create(ctx, domain.Notification{
		UserID:     content.AuthorID,
		Type:       "comment",
		Title:      label + "收到新评论",
		Content:    fmt.Sprintf("用户 #%d 评论了《%s》", actorID, content.Title),
		ActorID:    actorID,
		EntityType: entityType,
		EntityID:   entityID,
		SourceID:   commentID,
	}, eventID, occurredAt)
}

func (s *Service) notifyReply(ctx context.Context, eventID string, commentID, parentID int64, entityType string, entityID, actorID int64, occurredAt time.Time) error {
	if parentID <= 0 {
		return nil
	}
	parent, err := s.repo.GetComment(ctx, parentID)
	if err != nil {
		return err
	}
	if parent.AuthorID <= 0 {
		return s.repo.SavePendingReplyNotification(ctx, eventID, parentID, commentID, entityType, entityID, actorID, occurredAt)
	}
	if parent.AuthorID == actorID {
		return nil
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     parent.AuthorID,
		Type:       "reply",
		Title:      "评论收到回复",
		Content:    fmt.Sprintf("用户 #%d 回复了你的评论", actorID),
		ActorID:    actorID,
		EntityType: entityType,
		EntityID:   entityID,
		SourceID:   commentID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyReaction(ctx context.Context, eventID string, kind string, entityType string, entityID, actorID int64, occurredAt time.Time) error {
	if entityID <= 0 || actorID <= 0 || !supportedContentType(entityType) {
		return nil
	}
	content, err := s.repo.GetContent(ctx, entityType, entityID)
	if err != nil {
		return err
	}
	if content.AuthorID <= 0 {
		return s.repo.SavePendingContentNotification(ctx, eventID, kind, entityType, entityID, actorID, entityID, occurredAt)
	}
	if content.AuthorID == actorID {
		return nil
	}
	label := contentLabel(entityType)
	title := label + "收到互动"
	message := fmt.Sprintf("用户 #%d 与《%s》发生了互动", actorID, content.Title)
	if kind == "like" {
		title = label + "被点赞"
		message = fmt.Sprintf("用户 #%d 点赞了《%s》", actorID, content.Title)
	}
	if kind == "favorite" {
		title = label + "被收藏"
		message = fmt.Sprintf("用户 #%d 收藏了《%s》", actorID, content.Title)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     content.AuthorID,
		Type:       kind,
		Title:      title,
		Content:    message,
		ActorID:    actorID,
		EntityType: entityType,
		EntityID:   entityID,
		SourceID:   entityID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyQAAccepted(ctx context.Context, eventID string, topicID int64, title string, questionAuthorID, acceptedCommentID, acceptedCommentAuthorID, rewardCredits int64, occurredAt time.Time) error {
	if eventID == "" || topicID <= 0 || acceptedCommentID <= 0 || acceptedCommentAuthorID <= 0 {
		return nil
	}
	topicTitle := strings.TrimSpace(title)
	if topicTitle == "" {
		topicTitle = fmt.Sprintf("话题 #%d", topicID)
	}
	content := fmt.Sprintf("你的回答被采纳：话题《%s》", topicTitle)
	if rewardCredits > 0 {
		content = fmt.Sprintf("%s，获得 %d 积分", content, rewardCredits)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     acceptedCommentAuthorID,
		Type:       "qa_answer_accepted",
		Title:      "回答被采纳",
		Content:    content,
		ActorID:    questionAuthorID,
		EntityType: "topic",
		EntityID:   topicID,
		SourceID:   acceptedCommentID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyMallRefund(ctx context.Context, eventID string, approved bool, refundID, orderID, userID, amountCredits int64, orderNo, reason, adminNote string, digitalEntitlements []MallDigitalEntitlement, occurredAt time.Time) error {
	if eventID == "" || refundID <= 0 || orderID <= 0 || userID <= 0 {
		return nil
	}
	title := "售后申请已拒绝"
	content := fmt.Sprintf("订单 %s 的售后申请未通过", orderNo)
	notificationType := "mall_refund_rejected"
	if approved {
		title = "售后退款已通过"
		content = fmt.Sprintf("订单 %s 已退款 %d 积分", orderNo, amountCredits)
		notificationType = "mall_refund_approved"
	}
	if approved && len(digitalEntitlements) > 0 {
		content = fmt.Sprintf("%s。数字权益已撤销：%s", content, mallDigitalEntitlementSummary(digitalEntitlements))
	}
	if adminNote != "" {
		content = fmt.Sprintf("%s。审核备注：%s", content, adminNote)
	} else if reason != "" {
		content = fmt.Sprintf("%s。原因：%s", content, reason)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     userID,
		Type:       notificationType,
		Title:      title,
		Content:    content,
		EntityType: "mall_order",
		EntityID:   orderID,
		SourceID:   refundID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyMallDigitalEntitlementRevoked(ctx context.Context, eventID string, entitlementID, orderID, userID int64, orderNo, operatorID, reason string, entitlement MallDigitalEntitlement, occurredAt time.Time) error {
	if eventID == "" || entitlementID <= 0 || userID <= 0 {
		return nil
	}
	content := fmt.Sprintf("订单 %s 的数字权益已撤销：%s", orderNo, mallDigitalEntitlementSummary([]MallDigitalEntitlement{entitlement}))
	if reason != "" {
		content = fmt.Sprintf("%s。原因：%s", content, reason)
	}
	if operatorID != "" {
		content = fmt.Sprintf("%s。操作人：%s", content, operatorID)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     userID,
		Type:       "mall_digital_entitlement_revoked",
		Title:      "数字权益已撤销",
		Content:    content,
		EntityType: "mall_order",
		EntityID:   orderID,
		SourceID:   entitlementID,
	}, eventID, occurredAt)
}

func mallDigitalEntitlementSummary(entitlements []MallDigitalEntitlement) string {
	items := make([]string, 0, len(entitlements))
	for _, entitlement := range entitlements {
		title := strings.TrimSpace(entitlement.Title)
		if title == "" {
			title = strings.TrimSpace(entitlement.SKU)
		}
		if title == "" && entitlement.ProductID > 0 {
			title = fmt.Sprintf("商品 #%d", entitlement.ProductID)
		}
		parts := nonEmptyStrings(title)
		grantKey := strings.TrimSpace(entitlement.GrantKey)
		if grantKey != "" {
			parts = append(parts, fmt.Sprintf("%s %s", mallGrantTypeLabel(entitlement.GrantType), grantKey))
		}
		if code := strings.TrimSpace(entitlement.FulfillmentCode); code != "" {
			parts = append(parts, code)
		}
		if len(parts) > 0 {
			items = append(items, strings.Join(parts, " / "))
		}
	}
	if len(items) == 0 {
		return "权益已失效"
	}
	if len(items) > 3 {
		return strings.Join(items[:3], "；") + fmt.Sprintf("；等 %d 项", len(items))
	}
	return strings.Join(items, "；")
}

func mallGrantTypeLabel(grantType string) string {
	switch strings.ToLower(strings.TrimSpace(grantType)) {
	case "badge":
		return "徽章"
	case "theme":
		return "主题"
	case "membership":
		return "会员"
	default:
		return "数字权益"
	}
}

func (s *Service) NotifyMallOrderPaid(ctx context.Context, eventID string, orderID, userID, totalCredits int64, orderNo, paymentMethod string, digitalEntitlements []MallDigitalEntitlement, occurredAt time.Time) error {
	if eventID == "" || orderID <= 0 || userID <= 0 {
		return nil
	}
	content := fmt.Sprintf("订单 %s 已支付 %d 积分", orderNo, totalCredits)
	if paymentMethod != "" {
		content = fmt.Sprintf("%s。支付方式：%s", content, paymentMethod)
	}
	if len(digitalEntitlements) > 0 {
		content = fmt.Sprintf("%s。数字权益已发放：%s", content, mallDigitalEntitlementSummary(digitalEntitlements))
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     userID,
		Type:       "mall_order_paid",
		Title:      "订单已支付",
		Content:    content,
		EntityType: "mall_order",
		EntityID:   orderID,
		SourceID:   orderID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyMallOrderStatus(ctx context.Context, eventID string, shipped bool, orderID, userID int64, orderNo, shippingCarrier, trackingNo, note string, occurredAt time.Time) error {
	if eventID == "" || orderID <= 0 || userID <= 0 {
		return nil
	}
	title := "订单已完成"
	content := fmt.Sprintf("订单 %s 已完成", orderNo)
	notificationType := "mall_order_completed"
	if shipped {
		title = "订单已发货"
		content = fmt.Sprintf("订单 %s 已发货", orderNo)
		notificationType = "mall_order_shipped"
		if shippingCarrier != "" || trackingNo != "" {
			content = fmt.Sprintf("%s。物流：%s", content, strings.Join(nonEmptyStrings(shippingCarrier, trackingNo), " / "))
		}
	}
	if note != "" {
		content = fmt.Sprintf("%s。备注：%s", content, note)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     userID,
		Type:       notificationType,
		Title:      title,
		Content:    content,
		EntityType: "mall_order",
		EntityID:   orderID,
		SourceID:   orderID,
	}, eventID, occurredAt)
}

func (s *Service) NotifyMallProductReviewStatus(ctx context.Context, eventID string, published bool, reviewID, productID, orderID, userID int64, productTitle string, occurredAt time.Time) error {
	if eventID == "" || reviewID <= 0 || productID <= 0 || userID <= 0 {
		return nil
	}
	title := "商品评价未展示"
	notificationType := "mall_review_hidden"
	productName := strings.TrimSpace(productTitle)
	if productName == "" {
		productName = fmt.Sprintf("商品 #%d", productID)
	}
	content := fmt.Sprintf("你对《%s》的评价暂未展示", productName)
	if published {
		title = "商品评价已展示"
		notificationType = "mall_review_published"
		content = fmt.Sprintf("你对《%s》的评价已审核通过并展示", productName)
	}
	if orderID > 0 {
		content = fmt.Sprintf("%s。订单 #%d", content, orderID)
	}
	return s.repo.Create(ctx, domain.Notification{
		UserID:     userID,
		Type:       notificationType,
		Title:      title,
		Content:    content,
		EntityType: "mall_product",
		EntityID:   productID,
		SourceID:   reviewID,
	}, eventID, occurredAt)
}

func supportedContentType(entityType string) bool {
	return entityType == "article" || entityType == "topic"
}

func nonEmptyStrings(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}

func contentLabel(entityType string) string {
	if entityType == "topic" {
		return "话题"
	}
	return "文章"
}
