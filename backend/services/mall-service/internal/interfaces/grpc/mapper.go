package grpc

import (
	"time"

	pb "mall-service/api/proto/mallpb"
	domain "mall-service/internal/domain/mall"
)

func productToPB(product domain.Product) *pb.Product {
	return &pb.Product{
		Id:           product.ID,
		Sku:          product.SKU,
		Title:        product.Title,
		Description:  product.Description,
		Category:     product.Category,
		CoverUrl:     product.CoverURL,
		GrantType:    product.GrantType,
		GrantKey:     product.GrantKey,
		PriceCredits: product.PriceCredits,
		Stock:        product.Stock,
		SalesCount:   product.SalesCount,
		Status:       productStatusToPB(product.Status),
		Sort:         product.Sort,
		CreatedAt:    millis(product.CreatedAt),
		UpdatedAt:    millis(product.UpdatedAt),
	}
}

func productsToPB(items []domain.Product) []*pb.Product {
	out := make([]*pb.Product, 0, len(items))
	for _, item := range items {
		out = append(out, productToPB(item))
	}
	return out
}

func productCategoryToPB(category domain.ProductCategory) *pb.ProductCategory {
	return &pb.ProductCategory{
		Id:           category.ID,
		Slug:         category.Slug,
		Name:         category.Name,
		Description:  category.Description,
		Status:       productCategoryStatusToPB(category.Status),
		Sort:         category.Sort,
		ProductCount: category.ProductCount,
		CreatedAt:    millis(category.CreatedAt),
		UpdatedAt:    millis(category.UpdatedAt),
	}
}

func productCategoriesToPB(items []domain.ProductCategory) []*pb.ProductCategory {
	out := make([]*pb.ProductCategory, 0, len(items))
	for _, item := range items {
		out = append(out, productCategoryToPB(item))
	}
	return out
}

func productReviewToPB(review domain.ProductReview) *pb.ProductReview {
	return &pb.ProductReview{
		Id:           review.ID,
		ProductId:    review.ProductID,
		ProductSku:   review.ProductSKU,
		ProductTitle: review.ProductTitle,
		OrderId:      review.OrderID,
		UserId:       review.UserID,
		Rating:       review.Rating,
		Content:      review.Content,
		Status:       productReviewStatusToPB(review.Status),
		CreatedAt:    millis(review.CreatedAt),
		UpdatedAt:    millis(review.UpdatedAt),
	}
}

func productReviewsToPB(items []domain.ProductReview) []*pb.ProductReview {
	out := make([]*pb.ProductReview, 0, len(items))
	for _, item := range items {
		out = append(out, productReviewToPB(item))
	}
	return out
}

func productFavoriteToPB(item domain.ProductFavorite) *pb.ProductFavorite {
	return &pb.ProductFavorite{
		Product:   productToPB(item.Product),
		CreatedAt: millis(item.CreatedAt),
	}
}

func productFavoritesToPB(items []domain.ProductFavorite) []*pb.ProductFavorite {
	out := make([]*pb.ProductFavorite, 0, len(items))
	for _, item := range items {
		out = append(out, productFavoriteToPB(item))
	}
	return out
}

func couponToPB(coupon domain.Coupon) *pb.Coupon {
	var startsAt int64
	if coupon.StartsAt != nil {
		startsAt = millis(*coupon.StartsAt)
	}
	var endsAt int64
	if coupon.EndsAt != nil {
		endsAt = millis(*coupon.EndsAt)
	}
	return &pb.Coupon{
		Id:              coupon.ID,
		Code:            coupon.Code,
		Name:            coupon.Name,
		Description:     coupon.Description,
		DiscountCredits: coupon.DiscountCredits,
		MinOrderCredits: coupon.MinOrderCredits,
		TotalQuota:      coupon.TotalQuota,
		PerUserLimit:    coupon.PerUserLimit,
		ClaimedCount:    coupon.ClaimedCount,
		UsedCount:       coupon.UsedCount,
		Status:          couponStatusToPB(coupon.Status),
		StartsAt:        startsAt,
		EndsAt:          endsAt,
		CreatedAt:       millis(coupon.CreatedAt),
		UpdatedAt:       millis(coupon.UpdatedAt),
	}
}

func couponsToPB(items []domain.Coupon) []*pb.Coupon {
	out := make([]*pb.Coupon, 0, len(items))
	for _, item := range items {
		out = append(out, couponToPB(item))
	}
	return out
}

func couponUsageToPB(usage domain.CouponUsage) *pb.CouponUsage {
	var usedAt int64
	if usage.UsedAt != nil {
		usedAt = millis(*usage.UsedAt)
	}
	var releasedAt int64
	if usage.ReleasedAt != nil {
		releasedAt = millis(*usage.ReleasedAt)
	}
	return &pb.CouponUsage{
		Id:              usage.ID,
		CouponId:        usage.CouponID,
		Code:            usage.Code,
		UserId:          usage.UserID,
		OrderId:         usage.OrderID,
		Status:          couponUsageStatusToPB(usage.Status),
		DiscountCredits: usage.DiscountCredits,
		CreatedAt:       millis(usage.CreatedAt),
		UsedAt:          usedAt,
		ReleasedAt:      releasedAt,
		UpdatedAt:       millis(usage.UpdatedAt),
		Coupon:          couponToPB(usage.Coupon),
	}
}

func couponUsagesToPB(items []domain.CouponUsage) []*pb.CouponUsage {
	out := make([]*pb.CouponUsage, 0, len(items))
	for _, item := range items {
		out = append(out, couponUsageToPB(item))
	}
	return out
}

func productStockLogToPB(log domain.ProductStockLog) *pb.ProductStockLog {
	return &pb.ProductStockLog{
		Id:            log.ID,
		ProductId:     log.ProductID,
		Sku:           log.SKU,
		Title:         log.Title,
		Delta:         log.Delta,
		BeforeStock:   log.BeforeStock,
		AfterStock:    log.AfterStock,
		Reason:        log.Reason,
		ReferenceType: log.ReferenceType,
		ReferenceId:   log.ReferenceID,
		OperatorType:  log.OperatorType,
		OperatorId:    log.OperatorID,
		Note:          log.Note,
		CreatedAt:     millis(log.CreatedAt),
	}
}

func productStockLogsToPB(items []domain.ProductStockLog) []*pb.ProductStockLog {
	out := make([]*pb.ProductStockLog, 0, len(items))
	for _, item := range items {
		out = append(out, productStockLogToPB(item))
	}
	return out
}

func outboxRequeueAuditToPB(audit domain.OutboxRequeueAudit) *pb.OutboxRequeueAudit {
	return &pb.OutboxRequeueAudit{
		Id:               audit.ID,
		EventId:          audit.EventID,
		AggregateType:    audit.AggregateType,
		AggregateId:      audit.AggregateID,
		PreviousStatus:   audit.PreviousStatus,
		PreviousAttempts: int32(audit.PreviousAttempts),
		PreviousError:    audit.PreviousError,
		OperatorId:       audit.OperatorID,
		RequeuedAt:       millis(audit.RequeuedAt),
	}
}

func outboxRequeueAuditsToPB(items []domain.OutboxRequeueAudit) []*pb.OutboxRequeueAudit {
	out := make([]*pb.OutboxRequeueAudit, 0, len(items))
	for _, item := range items {
		out = append(out, outboxRequeueAuditToPB(item))
	}
	return out
}

func orderToPB(order domain.Order) *pb.Order {
	var paidAt int64
	if order.PaidAt != nil {
		paidAt = millis(*order.PaidAt)
	}
	var shippedAt int64
	if order.ShippedAt != nil {
		shippedAt = millis(*order.ShippedAt)
	}
	var completedAt int64
	if order.CompletedAt != nil {
		completedAt = millis(*order.CompletedAt)
	}
	return &pb.Order{
		Id:                  order.ID,
		OrderNo:             order.OrderNo,
		IdempotencyKey:      order.IdempotencyKey,
		UserId:              order.UserID,
		Items:               orderItemsToPB(order.Items),
		DigitalEntitlements: digitalEntitlementsToPB(order.DigitalEntitlements),
		TotalCredits:        order.TotalCredits,
		OriginalCredits:     order.OriginalCredits,
		DiscountCredits:     order.DiscountCredits,
		CouponId:            order.CouponID,
		CouponCode:          order.CouponCode,
		Status:              orderStatusToPB(order.Status),
		Receiver:            order.Receiver,
		Phone:               order.Phone,
		Address:             order.Address,
		PaymentMethod:       order.PaymentMethod,
		PaidAt:              paidAt,
		CreatedAt:           millis(order.CreatedAt),
		UpdatedAt:           millis(order.UpdatedAt),
		ShippingCarrier:     order.ShippingCarrier,
		TrackingNo:          order.TrackingNo,
		ShippedAt:           shippedAt,
		CompletedAt:         completedAt,
	}
}

func ordersToPB(items []domain.Order) []*pb.Order {
	out := make([]*pb.Order, 0, len(items))
	for _, item := range items {
		out = append(out, orderToPB(item))
	}
	return out
}

func orderItemsToPB(items []domain.OrderItem) []*pb.OrderItem {
	out := make([]*pb.OrderItem, 0, len(items))
	for _, item := range items {
		out = append(out, &pb.OrderItem{
			ProductId:        item.ProductID,
			Sku:              item.SKU,
			Title:            item.Title,
			Quantity:         item.Quantity,
			UnitPriceCredits: item.UnitPriceCredits,
			SubtotalCredits:  item.SubtotalCredits,
			GrantType:        item.GrantType,
			GrantKey:         item.GrantKey,
		})
	}
	return out
}

func digitalEntitlementsToPB(items []domain.DigitalEntitlement) []*pb.DigitalEntitlement {
	out := make([]*pb.DigitalEntitlement, 0, len(items))
	for _, item := range items {
		out = append(out, digitalEntitlementToPB(item))
	}
	return out
}

func digitalEntitlementToPB(item domain.DigitalEntitlement) *pb.DigitalEntitlement {
	var revokedAt int64
	if item.RevokedAt != nil {
		revokedAt = millis(*item.RevokedAt)
	}
	status := digitalEntitlementStatusForResponse(item, time.Now())
	return &pb.DigitalEntitlement{
		Id:              item.ID,
		OrderId:         item.OrderID,
		OrderNo:         item.OrderNo,
		UserId:          item.UserID,
		ProductId:       item.ProductID,
		Sku:             item.SKU,
		Title:           item.Title,
		Quantity:        item.Quantity,
		FulfillmentCode: item.Code,
		GrantType:       item.GrantType,
		GrantKey:        item.GrantKey,
		IssuedAt:        millis(item.IssuedAt),
		Status:          status,
		RevokedAt:       revokedAt,
		RefundId:        item.RefundID,
		ExpiresAt:       millisPtr(item.ExpiresAt),
		RevokedBy:       item.RevokedBy,
		RevokeReason:    item.RevokeReason,
	}
}

func digitalEntitlementStatusForResponse(item domain.DigitalEntitlement, now time.Time) string {
	if item.RevokedAt != nil || item.Status == domain.DigitalEntitlementStatusRevoked {
		return domain.DigitalEntitlementStatusRevoked
	}
	if item.Status == domain.DigitalEntitlementStatusActive && item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
		return domain.DigitalEntitlementStatusExpired
	}
	return item.Status
}

func paymentToPB(payment domain.Payment) *pb.Payment {
	var paidAt int64
	if payment.PaidAt != nil {
		paidAt = millis(*payment.PaidAt)
	}
	return &pb.Payment{
		Id:              payment.ID,
		OrderId:         payment.OrderID,
		OrderNo:         payment.OrderNo,
		UserId:          payment.UserID,
		AmountCredits:   payment.AmountCredits,
		Provider:        payment.Provider,
		IdempotencyKey:  payment.IdempotencyKey,
		Status:          paymentStatusToPB(payment.Status),
		ProviderTradeNo: payment.ProviderTradeNo,
		FailureReason:   payment.FailureReason,
		PaidAt:          paidAt,
		CreatedAt:       millis(payment.CreatedAt),
		UpdatedAt:       millis(payment.UpdatedAt),
	}
}

func paymentsToPB(items []domain.Payment) []*pb.Payment {
	out := make([]*pb.Payment, 0, len(items))
	for _, item := range items {
		out = append(out, paymentToPB(item))
	}
	return out
}

func cartItemToPB(item domain.CartItem) *pb.CartItem {
	return &pb.CartItem{
		Product:   productToPB(item.Product),
		Quantity:  item.Quantity,
		CreatedAt: millis(item.CreatedAt),
		UpdatedAt: millis(item.UpdatedAt),
	}
}

func cartItemsToPB(items []domain.CartItem) []*pb.CartItem {
	out := make([]*pb.CartItem, 0, len(items))
	for _, item := range items {
		out = append(out, cartItemToPB(item))
	}
	return out
}

func orderStatusLogToPB(log domain.OrderStatusLog) *pb.OrderStatusLog {
	return &pb.OrderStatusLog{
		Id:           log.ID,
		OrderId:      log.OrderID,
		FromStatus:   orderStatusToPB(log.FromStatus),
		ToStatus:     orderStatusToPB(log.ToStatus),
		Reason:       log.Reason,
		OperatorType: log.OperatorType,
		OperatorId:   log.OperatorID,
		Note:         log.Note,
		CreatedAt:    millis(log.CreatedAt),
	}
}

func refundRequestToPB(refund domain.RefundRequest) *pb.RefundRequest {
	var reviewedAt int64
	if refund.ReviewedAt != nil {
		reviewedAt = millis(*refund.ReviewedAt)
	}
	var refundedAt int64
	if refund.RefundedAt != nil {
		refundedAt = millis(*refund.RefundedAt)
	}
	var canceledAt int64
	if refund.CanceledAt != nil {
		canceledAt = millis(*refund.CanceledAt)
	}
	return &pb.RefundRequest{
		Id:            refund.ID,
		OrderId:       refund.OrderID,
		OrderNo:       refund.OrderNo,
		UserId:        refund.UserID,
		AmountCredits: refund.AmountCredits,
		Status:        refundStatusToPB(refund.Status),
		Reason:        refund.Reason,
		UserNote:      refund.UserNote,
		AdminNote:     refund.AdminNote,
		RestoreStock:  refund.RestoreStock,
		OperatorId:    refund.OperatorID,
		RequestedAt:   millis(refund.RequestedAt),
		ReviewedAt:    reviewedAt,
		RefundedAt:    refundedAt,
		CanceledAt:    canceledAt,
		CreatedAt:     millis(refund.CreatedAt),
		UpdatedAt:     millis(refund.UpdatedAt),
	}
}

func refundRequestsToPB(items []domain.RefundRequest) []*pb.RefundRequest {
	out := make([]*pb.RefundRequest, 0, len(items))
	for _, item := range items {
		out = append(out, refundRequestToPB(item))
	}
	return out
}

func addressToPB(address domain.Address) *pb.Address {
	return &pb.Address{
		Id:         address.ID,
		UserId:     address.UserID,
		Receiver:   address.Receiver,
		Phone:      address.Phone,
		Province:   address.Province,
		City:       address.City,
		District:   address.District,
		Detail:     address.Detail,
		PostalCode: address.PostalCode,
		IsDefault:  address.IsDefault,
		CreatedAt:  millis(address.CreatedAt),
		UpdatedAt:  millis(address.UpdatedAt),
	}
}

func addressesToPB(items []domain.Address) []*pb.Address {
	out := make([]*pb.Address, 0, len(items))
	for _, item := range items {
		out = append(out, addressToPB(item))
	}
	return out
}

func mallOverviewToPB(overview domain.MallOverview) *pb.MallOverview {
	return &pb.MallOverview{
		ProductTotal:                 overview.ProductTotal,
		ActiveProductTotal:           overview.ActiveProductTotal,
		LowStockTotal:                overview.LowStockTotal,
		StockTotal:                   overview.StockTotal,
		SalesCountTotal:              overview.SalesCountTotal,
		OrderTotal:                   overview.OrderTotal,
		PaidOrderTotal:               overview.PaidOrderTotal,
		RevenueCreditsTotal:          overview.RevenueCreditsTotal,
		TodayOrderTotal:              overview.TodayOrderTotal,
		TodayRevenueCredits:          overview.TodayRevenueCredits,
		PendingShipmentTotal:         overview.PendingShipmentTotal,
		PendingRefundTotal:           overview.PendingRefundTotal,
		RefundedCreditsTotal:         overview.RefundedCreditsTotal,
		SucceededPaymentCreditsTotal: overview.SucceededPaymentCreditsTotal,
		FailedPaymentTotal:           overview.FailedPaymentTotal,
		FailedPaymentCreditsTotal:    overview.FailedPaymentCreditsTotal,
		PendingRefundCreditsTotal:    overview.PendingRefundCreditsTotal,
		NetRevenueCreditsTotal:       overview.NetRevenueCreditsTotal,
		PendingOutboxTotal:           overview.PendingOutboxTotal,
		OrderStatusCounts:            statusCountsToPB(overview.OrderStatusCounts),
		RefundStatusCounts:           statusCountsToPB(overview.RefundStatusCounts),
		OutboxStatusCounts:           statusCountsToPB(overview.OutboxStatusCounts),
		OutboxLastError:              overview.OutboxLastError,
		OutboxLastErrorAt:            millisPtr(overview.OutboxLastErrorAt),
		OutboxNextAttemptAt:          millisPtr(overview.OutboxNextAttemptAt),
		FinanceAnomalyTotal:          overview.FinanceAnomalyTotal,
		FinanceAnomalies:             financeAnomaliesToPB(overview.FinanceAnomalies),
		LowStockProducts:             productsToPB(overview.LowStockProducts),
		TopSellingProducts:           productsToPB(overview.TopSellingProducts),
	}
}

func financeAnomalyToPB(item domain.FinanceAnomaly) *pb.FinanceAnomaly {
	return &pb.FinanceAnomaly{
		IssueType:               item.IssueType,
		OrderId:                 item.OrderID,
		OrderNo:                 item.OrderNo,
		UserId:                  item.UserID,
		OrderStatus:             orderStatusToPB(item.OrderStatus),
		OrderTotalCredits:       item.OrderTotalCredits,
		SucceededPaymentCredits: item.SucceededPaymentCredits,
		RefundedCredits:         item.RefundedCredits,
		DifferenceCredits:       item.DifferenceCredits,
		UpdatedAt:               millis(item.UpdatedAt),
	}
}

func financeAnomaliesToPB(items []domain.FinanceAnomaly) []*pb.FinanceAnomaly {
	out := make([]*pb.FinanceAnomaly, 0, len(items))
	for _, item := range items {
		out = append(out, financeAnomalyToPB(item))
	}
	return out
}

func statusCountsToPB(items []domain.StatusCount) []*pb.MallStatusCount {
	out := make([]*pb.MallStatusCount, 0, len(items))
	for _, item := range items {
		out = append(out, &pb.MallStatusCount{Status: item.Status, Count: item.Count})
	}
	return out
}

func orderStatusLogsToPB(items []domain.OrderStatusLog) []*pb.OrderStatusLog {
	out := make([]*pb.OrderStatusLog, 0, len(items))
	for _, item := range items {
		out = append(out, orderStatusLogToPB(item))
	}
	return out
}

func productStatusToPB(status domain.ProductStatus) pb.ProductStatus {
	switch status {
	case domain.ProductStatusDraft:
		return pb.ProductStatus_PRODUCT_STATUS_DRAFT
	case domain.ProductStatusActive:
		return pb.ProductStatus_PRODUCT_STATUS_ACTIVE
	case domain.ProductStatusArchived:
		return pb.ProductStatus_PRODUCT_STATUS_ARCHIVED
	default:
		return pb.ProductStatus_PRODUCT_STATUS_UNSPECIFIED
	}
}

func productStatusFromPB(status pb.ProductStatus) domain.ProductStatus {
	switch status {
	case pb.ProductStatus_PRODUCT_STATUS_DRAFT:
		return domain.ProductStatusDraft
	case pb.ProductStatus_PRODUCT_STATUS_ACTIVE:
		return domain.ProductStatusActive
	case pb.ProductStatus_PRODUCT_STATUS_ARCHIVED:
		return domain.ProductStatusArchived
	default:
		return ""
	}
}

func productCategoryStatusToPB(status domain.ProductCategoryStatus) pb.ProductCategoryStatus {
	switch status {
	case domain.ProductCategoryStatusDraft:
		return pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_DRAFT
	case domain.ProductCategoryStatusActive:
		return pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_ACTIVE
	case domain.ProductCategoryStatusArchived:
		return pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_ARCHIVED
	default:
		return pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_UNSPECIFIED
	}
}

func productCategoryStatusFromPB(status pb.ProductCategoryStatus) domain.ProductCategoryStatus {
	switch status {
	case pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_DRAFT:
		return domain.ProductCategoryStatusDraft
	case pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_ACTIVE:
		return domain.ProductCategoryStatusActive
	case pb.ProductCategoryStatus_PRODUCT_CATEGORY_STATUS_ARCHIVED:
		return domain.ProductCategoryStatusArchived
	default:
		return ""
	}
}

func productReviewStatusToPB(status domain.ProductReviewStatus) pb.ProductReviewStatus {
	switch status {
	case domain.ProductReviewStatusPending:
		return pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PENDING
	case domain.ProductReviewStatusPublished:
		return pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PUBLISHED
	case domain.ProductReviewStatusHidden:
		return pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_HIDDEN
	default:
		return pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_UNSPECIFIED
	}
}

func productReviewStatusFromPB(status pb.ProductReviewStatus) domain.ProductReviewStatus {
	switch status {
	case pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PENDING:
		return domain.ProductReviewStatusPending
	case pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_PUBLISHED:
		return domain.ProductReviewStatusPublished
	case pb.ProductReviewStatus_PRODUCT_REVIEW_STATUS_HIDDEN:
		return domain.ProductReviewStatusHidden
	default:
		return ""
	}
}

func orderStatusToPB(status domain.OrderStatus) pb.OrderStatus {
	switch status {
	case domain.OrderStatusPendingPayment:
		return pb.OrderStatus_ORDER_STATUS_PENDING_PAYMENT
	case domain.OrderStatusPaying:
		return pb.OrderStatus_ORDER_STATUS_PAYING
	case domain.OrderStatusPaid:
		return pb.OrderStatus_ORDER_STATUS_PAID
	case domain.OrderStatusCanceled:
		return pb.OrderStatus_ORDER_STATUS_CANCELED
	case domain.OrderStatusShipped:
		return pb.OrderStatus_ORDER_STATUS_SHIPPED
	case domain.OrderStatusCompleted:
		return pb.OrderStatus_ORDER_STATUS_COMPLETED
	case domain.OrderStatusClosed:
		return pb.OrderStatus_ORDER_STATUS_CLOSED
	case domain.OrderStatusRefunded:
		return pb.OrderStatus_ORDER_STATUS_REFUNDED
	default:
		return pb.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func orderStatusFromPB(status pb.OrderStatus) domain.OrderStatus {
	switch status {
	case pb.OrderStatus_ORDER_STATUS_PENDING_PAYMENT:
		return domain.OrderStatusPendingPayment
	case pb.OrderStatus_ORDER_STATUS_PAYING:
		return domain.OrderStatusPaying
	case pb.OrderStatus_ORDER_STATUS_PAID:
		return domain.OrderStatusPaid
	case pb.OrderStatus_ORDER_STATUS_CANCELED:
		return domain.OrderStatusCanceled
	case pb.OrderStatus_ORDER_STATUS_SHIPPED:
		return domain.OrderStatusShipped
	case pb.OrderStatus_ORDER_STATUS_COMPLETED:
		return domain.OrderStatusCompleted
	case pb.OrderStatus_ORDER_STATUS_CLOSED:
		return domain.OrderStatusClosed
	case pb.OrderStatus_ORDER_STATUS_REFUNDED:
		return domain.OrderStatusRefunded
	default:
		return ""
	}
}

func refundStatusToPB(status domain.RefundStatus) pb.RefundStatus {
	switch status {
	case domain.RefundStatusRequested:
		return pb.RefundStatus_REFUND_STATUS_REQUESTED
	case domain.RefundStatusProcessing:
		return pb.RefundStatus_REFUND_STATUS_PROCESSING
	case domain.RefundStatusApproved:
		return pb.RefundStatus_REFUND_STATUS_APPROVED
	case domain.RefundStatusRejected:
		return pb.RefundStatus_REFUND_STATUS_REJECTED
	case domain.RefundStatusCanceled:
		return pb.RefundStatus_REFUND_STATUS_CANCELED
	default:
		return pb.RefundStatus_REFUND_STATUS_UNSPECIFIED
	}
}

func refundStatusFromPB(status pb.RefundStatus) domain.RefundStatus {
	switch status {
	case pb.RefundStatus_REFUND_STATUS_REQUESTED:
		return domain.RefundStatusRequested
	case pb.RefundStatus_REFUND_STATUS_PROCESSING:
		return domain.RefundStatusProcessing
	case pb.RefundStatus_REFUND_STATUS_APPROVED:
		return domain.RefundStatusApproved
	case pb.RefundStatus_REFUND_STATUS_REJECTED:
		return domain.RefundStatusRejected
	case pb.RefundStatus_REFUND_STATUS_CANCELED:
		return domain.RefundStatusCanceled
	default:
		return ""
	}
}

func couponStatusToPB(status domain.CouponStatus) pb.CouponStatus {
	switch status {
	case domain.CouponStatusDraft:
		return pb.CouponStatus_COUPON_STATUS_DRAFT
	case domain.CouponStatusActive:
		return pb.CouponStatus_COUPON_STATUS_ACTIVE
	case domain.CouponStatusArchived:
		return pb.CouponStatus_COUPON_STATUS_ARCHIVED
	default:
		return pb.CouponStatus_COUPON_STATUS_UNSPECIFIED
	}
}

func couponStatusFromPB(status pb.CouponStatus) domain.CouponStatus {
	switch status {
	case pb.CouponStatus_COUPON_STATUS_DRAFT:
		return domain.CouponStatusDraft
	case pb.CouponStatus_COUPON_STATUS_ACTIVE:
		return domain.CouponStatusActive
	case pb.CouponStatus_COUPON_STATUS_ARCHIVED:
		return domain.CouponStatusArchived
	default:
		return ""
	}
}

func couponUsageStatusToPB(status domain.CouponUsageStatus) pb.CouponUsageStatus {
	switch status {
	case domain.CouponUsageStatusClaimed:
		return pb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED
	case domain.CouponUsageStatusReserved:
		return pb.CouponUsageStatus_COUPON_USAGE_STATUS_RESERVED
	case domain.CouponUsageStatusUsed:
		return pb.CouponUsageStatus_COUPON_USAGE_STATUS_USED
	case domain.CouponUsageStatusReleased:
		return pb.CouponUsageStatus_COUPON_USAGE_STATUS_RELEASED
	default:
		return pb.CouponUsageStatus_COUPON_USAGE_STATUS_UNSPECIFIED
	}
}

func couponUsageStatusFromPB(status pb.CouponUsageStatus) domain.CouponUsageStatus {
	switch status {
	case pb.CouponUsageStatus_COUPON_USAGE_STATUS_CLAIMED:
		return domain.CouponUsageStatusClaimed
	case pb.CouponUsageStatus_COUPON_USAGE_STATUS_RESERVED:
		return domain.CouponUsageStatusReserved
	case pb.CouponUsageStatus_COUPON_USAGE_STATUS_USED:
		return domain.CouponUsageStatusUsed
	case pb.CouponUsageStatus_COUPON_USAGE_STATUS_RELEASED:
		return domain.CouponUsageStatusReleased
	default:
		return ""
	}
}

func paymentStatusToPB(status domain.PaymentStatus) pb.PaymentStatus {
	switch status {
	case domain.PaymentStatusPending:
		return pb.PaymentStatus_PAYMENT_STATUS_PENDING
	case domain.PaymentStatusSucceeded:
		return pb.PaymentStatus_PAYMENT_STATUS_SUCCEEDED
	case domain.PaymentStatusFailed:
		return pb.PaymentStatus_PAYMENT_STATUS_FAILED
	default:
		return pb.PaymentStatus_PAYMENT_STATUS_UNSPECIFIED
	}
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func millisPtr(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return millis(*t)
}
