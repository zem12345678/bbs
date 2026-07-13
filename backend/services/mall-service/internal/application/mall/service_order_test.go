package mall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestCreateOrderAllowsDigitalProductWithoutShippingAddress(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			101: {
				ID:           101,
				Title:        "数字专栏",
				Category:     "digital",
				GrantType:    "badge",
				GrantKey:     "badge-founder",
				PriceCredits: 120,
				Stock:        50,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "digital-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 101, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if repo.createOrderCalls != 1 {
		t.Fatalf("CreateOrder() calls = %d, want 1", repo.createOrderCalls)
	}
	if got := strings.TrimSpace(result.Order.Receiver); got != "" {
		t.Fatalf("CreateOrder() receiver = %q, want empty", got)
	}
	if got := strings.TrimSpace(result.Order.Phone); got != "" {
		t.Fatalf("CreateOrder() phone = %q, want empty", got)
	}
	if got := strings.TrimSpace(result.Order.Address); got != "" {
		t.Fatalf("CreateOrder() address = %q, want empty", got)
	}
	if got := result.Order.Items[0].Category; got != "digital" {
		t.Fatalf("CreateOrder() item category = %q, want digital", got)
	}
	if got := result.Order.Items[0].GrantType; got != "badge" {
		t.Fatalf("CreateOrder() item grant type = %q, want badge", got)
	}
	if got := result.Order.Items[0].GrantKey; got != "badge-founder" {
		t.Fatalf("CreateOrder() item grant key = %q, want badge-founder", got)
	}
}

func TestCreateOrderRequiresShippingAddressForPhysicalProduct(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			201: {
				ID:           201,
				Title:        "实体周边",
				Category:     "books",
				PriceCredits: 80,
				Stock:        12,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "physical-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 201, Quantity: 1},
		},
	})
	if err == nil {
		t.Fatal("CreateOrder() error = nil, want shipping validation error")
	}
	if !strings.Contains(err.Error(), "receiver is required") {
		t.Fatalf("CreateOrder() error = %q, want receiver validation", err.Error())
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
}

func TestCreateOrderRejectsQuantityAboveStockAfterMergingItems(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			202: {
				ID:           202,
				Title:        "数字专栏",
				Category:     "digital",
				PriceCredits: 80,
				Stock:        3,
				Status:       domain.ProductStatusActive,
			},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "oversold-direct-order",
		UserID:         7,
		Items: []domain.CreateOrderItem{
			{ProductID: 202, Quantity: 2},
			{ProductID: 202, Quantity: 2},
		},
	})
	if !errors.Is(err, domain.ErrInsufficientStock) {
		t.Fatalf("CreateOrder() error = %v, want insufficient stock", err)
	}
	if repo.createOrderCalls != 0 {
		t.Fatalf("CreateOrder() calls = %d, want 0", repo.createOrderCalls)
	}
}

func TestCreateOrderDoesNotReuseAnotherUsersIdempotencyKey(t *testing.T) {
	repo := &orderRepoStub{
		products: map[int64]domain.Product{
			203: {ID: 203, Title: "数字专栏", Category: "digital", PriceCredits: 80, Stock: 3, Status: domain.ProductStatusActive},
		},
		idempotencyOrders: map[string]domain.Order{
			"7:shared-key": {ID: 7001, UserID: 7, IdempotencyKey: "shared-key", Receiver: "用户一"},
		},
	}
	svc := NewService(repo, nil, time.Minute)

	result, err := svc.CreateOrder(context.Background(), CreateOrderCommand{
		IdempotencyKey: "shared-key",
		UserID:         8,
		Items:          []domain.CreateOrderItem{{ProductID: 203, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if result.Duplicate {
		t.Fatal("CreateOrder() reused another user's order")
	}
	if result.Order.UserID != 8 {
		t.Fatalf("CreateOrder() user = %d, want 8", result.Order.UserID)
	}
	if repo.createOrderCalls != 1 {
		t.Fatalf("CreateOrder() calls = %d, want 1", repo.createOrderCalls)
	}
}

func TestCheckoutCartRequiresShippingWhenAnyItemNeedsDelivery(t *testing.T) {
	t.Run("digital only", func(t *testing.T) {
		repo := &orderRepoStub{
			cartItems: []domain.CartItem{
				{
					Product: domain.Product{
						ID:           301,
						Title:        "数字专栏",
						Category:     "digital",
						PriceCredits: 60,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 2,
				},
			},
		}
		svc := NewService(repo, nil, time.Minute)

		result, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
			IdempotencyKey: "digital-cart",
			UserID:         7,
		})
		if err != nil {
			t.Fatalf("CheckoutCart() error = %v", err)
		}
		if repo.createOrderFromCartCalls != 1 {
			t.Fatalf("CheckoutCart() calls = %d, want 1", repo.createOrderFromCartCalls)
		}
		if got := strings.TrimSpace(result.Order.Receiver); got != "" {
			t.Fatalf("CheckoutCart() receiver = %q, want empty", got)
		}
	})

	t.Run("mixed cart", func(t *testing.T) {
		repo := &orderRepoStub{
			cartItems: []domain.CartItem{
				{
					Product: domain.Product{
						ID:           401,
						Title:        "数字专栏",
						Category:     "digital",
						PriceCredits: 60,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 1,
				},
				{
					Product: domain.Product{
						ID:           402,
						Title:        "实体周边",
						Category:     "merch",
						PriceCredits: 90,
						Stock:        100,
						Status:       domain.ProductStatusActive,
					},
					Quantity: 1,
				},
			},
		}
		svc := NewService(repo, nil, time.Minute)

		_, err := svc.CheckoutCart(context.Background(), CheckoutCartCommand{
			IdempotencyKey: "mixed-cart",
			UserID:         7,
		})
		if err == nil {
			t.Fatal("CheckoutCart() error = nil, want shipping validation error")
		}
		if !strings.Contains(err.Error(), "receiver is required") {
			t.Fatalf("CheckoutCart() error = %q, want receiver validation", err.Error())
		}
		if repo.createOrderFromCartCalls != 0 {
			t.Fatalf("CheckoutCart() calls = %d, want 0", repo.createOrderFromCartCalls)
		}
	})
}

func TestCreateRefundRequestRequiresUserNote(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           501,
			OrderNo:      "M501",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaid,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, _, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 501,
		UserID:  7,
		Reason:  "quality",
		Note:    " 短 ",
	})
	if err == nil {
		t.Fatal("CreateRefundRequest() error = nil, want note validation error")
	}
	if !strings.Contains(err.Error(), "refund note must be at least 4 characters") {
		t.Fatalf("CreateRefundRequest() error = %q, want note validation", err.Error())
	}
	if repo.createRefundRequestCalls != 0 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 0", repo.createRefundRequestCalls)
	}
}

func TestCreateRefundRequestTrimsAndPersistsUserNote(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           601,
			OrderNo:      "M601",
			UserID:       7,
			TotalCredits: 200,
			Status:       domain.OrderStatusCompleted,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	refund, duplicate, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 601,
		UserID:  7,
		Reason:  "after_sale",
		Note:    "  包装破损影响使用  ",
	})
	if err != nil {
		t.Fatalf("CreateRefundRequest() error = %v", err)
	}
	if duplicate {
		t.Fatal("CreateRefundRequest() duplicate = true, want false")
	}
	if repo.createRefundRequestCalls != 1 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 1", repo.createRefundRequestCalls)
	}
	if refund.UserNote != "包装破损影响使用" {
		t.Fatalf("CreateRefundRequest() note = %q, want trimmed note", refund.UserNote)
	}
}

func TestCreateRefundRequestReturnsExistingDuplicateRequest(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           602,
			OrderNo:      "M602",
			UserID:       7,
			TotalCredits: 180,
			Status:       domain.OrderStatusPaid,
		},
		refund: domain.RefundRequest{
			ID:            7602,
			OrderID:       602,
			OrderNo:       "M602",
			UserID:        7,
			AmountCredits: 180,
			Status:        domain.RefundStatusRequested,
			Reason:        "quality_issue",
			UserNote:      "原申请说明",
		},
	}
	svc := NewService(repo, nil, time.Minute)

	refund, duplicate, err := svc.CreateRefundRequest(context.Background(), CreateRefundRequestCommand{
		OrderID: 602,
		UserID:  7,
		Reason:  "after_sale",
		Note:    "  新的重复申请说明  ",
	})
	if err != nil {
		t.Fatalf("CreateRefundRequest() error = %v", err)
	}
	if !duplicate {
		t.Fatal("CreateRefundRequest() duplicate = false, want true")
	}
	if repo.createRefundRequestCalls != 1 {
		t.Fatalf("CreateRefundRequest() calls = %d, want 1", repo.createRefundRequestCalls)
	}
	if refund.ID != 7602 || refund.UserNote != "原申请说明" {
		t.Fatalf("CreateRefundRequest() refund = %+v, want existing refund", refund)
	}
}

func TestAdminReviewRefundRequestRetriesProcessingRefund(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     601,
			UserID: 7,
		},
		refund: domain.RefundRequest{
			ID:            701,
			OrderID:       601,
			OrderNo:       "M701",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusProcessing,
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	refund, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID:     701,
		Approved:     true,
		OperatorID:   "ops",
		AdminNote:    "重试退款",
		RestoreStock: true,
	})
	if err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}
	if refund.Status != domain.RefundStatusApproved {
		t.Fatalf("AdminReviewRefundRequest() status = %q, want approved", refund.Status)
	}
	if repo.startRefundApprovalCalls != 1 {
		t.Fatalf("StartRefundApproval() calls = %d, want 1", repo.startRefundApprovalCalls)
	}
	if repo.completeRefundApprovalCalls != 1 {
		t.Fatalf("CompleteRefundApproval() calls = %d, want 1", repo.completeRefundApprovalCalls)
	}
	if charger.adjustCalls != 1 {
		t.Fatalf("AdjustCredits() calls = %d, want 1", charger.adjustCalls)
	}
	if charger.adjustCommand.UserID != 7 || charger.adjustCommand.Delta != 200 {
		t.Fatalf("AdjustCredits() command = %+v, want user 7 delta 200", charger.adjustCommand)
	}
	if charger.adjustCommand.SourceEventID != "mall.refund:701" {
		t.Fatalf("AdjustCredits() source event = %q, want mall.refund:701", charger.adjustCommand.SourceEventID)
	}
}

func TestAdminReviewRefundRequestRetriesFailedCompletionWithStableCreditSourceEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     602,
			UserID: 7,
		},
		refund: domain.RefundRequest{
			ID:            702,
			OrderID:       602,
			OrderNo:       "M702",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusRequested,
		},
		completeRefundApprovalFailures: 1,
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)
	command := AdminReviewRefundRequestCommand{
		RefundID:     702,
		Approved:     true,
		OperatorID:   "ops",
		AdminNote:    "退款审批",
		RestoreStock: true,
	}

	if _, err := svc.AdminReviewRefundRequest(context.Background(), command); err == nil {
		t.Fatal("first AdminReviewRefundRequest() error = nil, want completion error")
	}
	if repo.refund.Status != domain.RefundStatusProcessing {
		t.Fatalf("refund status after failed completion = %q, want processing", repo.refund.Status)
	}

	refund, err := svc.AdminReviewRefundRequest(context.Background(), command)
	if err != nil {
		t.Fatalf("retry AdminReviewRefundRequest() error = %v", err)
	}
	if refund.Status != domain.RefundStatusApproved {
		t.Fatalf("retry refund status = %q, want approved", refund.Status)
	}
	if repo.completeRefundApprovalCalls != 2 {
		t.Fatalf("CompleteRefundApproval() calls = %d, want 2", repo.completeRefundApprovalCalls)
	}
	if charger.adjustCalls != 2 {
		t.Fatalf("AdjustCredits() calls = %d, want 2", charger.adjustCalls)
	}
	for i, adjust := range charger.adjustCommands {
		if adjust.SourceEventID != "mall.refund:702" {
			t.Fatalf("AdjustCredits() call %d source event = %q, want mall.refund:702", i+1, adjust.SourceEventID)
		}
	}

	if _, err := svc.AdminReviewRefundRequest(context.Background(), command); err != nil {
		t.Fatalf("settled AdminReviewRefundRequest() error = %v", err)
	}
	if repo.completeRefundApprovalCalls != 2 || charger.adjustCalls != 2 {
		t.Fatalf("settled refund retried completion=%d adjustments=%d, want 2 each", repo.completeRefundApprovalCalls, charger.adjustCalls)
	}
}

func TestAdminReviewRefundRequestIncludesRevokedDigitalEntitlementsInEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      603,
			OrderNo: "M703",
			UserID:  7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ProductID: 101,
					SKU:       "BADGE-FOUNDER",
					Title:     "创始会员徽章",
					Quantity:  1,
					Code:      "BBS-ENTITLEMENT",
					GrantType: "badge",
					GrantKey:  "badge-founder",
					Status:    domain.DigitalEntitlementStatusActive,
				},
			},
		},
		refund: domain.RefundRequest{
			ID:            703,
			OrderID:       603,
			OrderNo:       "M703",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusRequested,
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	if _, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID: 703,
		Approved: true,
	}); err != nil {
		t.Fatalf("AdminReviewRefundRequest() error = %v", err)
	}

	var payload refundReviewedEventPayload
	if err := json.Unmarshal(repo.completeRefundEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal refund event: %v", err)
	}
	if payload.EventType != RefundApprovedEventType || payload.RefundID != 703 || payload.OrderID != 603 {
		t.Fatalf("payload = %+v, want refund approval fields", payload)
	}
	if len(payload.DigitalEntitlements) != 1 {
		t.Fatalf("digital entitlements = %+v, want one entitlement", payload.DigitalEntitlements)
	}
	entitlement := payload.DigitalEntitlements[0]
	if entitlement.Status != domain.DigitalEntitlementStatusRevoked || entitlement.RefundID != 703 {
		t.Fatalf("entitlement state = %+v, want revoked refund 703", entitlement)
	}
	if entitlement.GrantKey != "badge-founder" || entitlement.FulfillmentCode != "BBS-ENTITLEMENT" {
		t.Fatalf("entitlement grant/code = %+v, want grant and fulfillment code", entitlement)
	}
}

func TestAdminReviewRefundRequestRejectsProcessingRefundRejection(t *testing.T) {
	repo := &orderRepoStub{
		refund: domain.RefundRequest{
			ID:            801,
			OrderID:       601,
			OrderNo:       "M801",
			UserID:        7,
			AmountCredits: 200,
			Status:        domain.RefundStatusProcessing,
		},
	}
	svc := NewService(repo, &creditChargerStub{}, time.Minute)

	_, err := svc.AdminReviewRefundRequest(context.Background(), AdminReviewRefundRequestCommand{
		RefundID: 801,
		Approved: false,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminReviewRefundRequest() error = %v, want invalid order state", err)
	}
	if repo.rejectRefundRequestCalls != 0 {
		t.Fatalf("RejectRefundRequest() calls = %d, want 0", repo.rejectRefundRequestCalls)
	}
}

func TestPayOrderRetriesFailedCompletionWithStableCreditSourceEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           811,
			OrderNo:      "M811",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPendingPayment,
		},
		completeOrderPaymentFailures: 1,
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)
	command := PayOrderCommand{
		OrderID:        811,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-811",
	}

	if _, err := svc.PayOrder(context.Background(), command); err == nil {
		t.Fatal("first PayOrder() error = nil, want completion error")
	}
	if repo.order.Status != domain.OrderStatusPaying {
		t.Fatalf("order status after failed completion = %q, want paying", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusPending {
		t.Fatalf("payment status after failed completion = %q, want pending", repo.payment.Status)
	}

	order, err := svc.PayOrder(context.Background(), command)
	if err != nil {
		t.Fatalf("retry PayOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusPaid {
		t.Fatalf("retry order status = %q, want paid", order.Status)
	}
	if repo.beginOrderPaymentCalls != 2 {
		t.Fatalf("BeginOrderPayment() calls = %d, want 2", repo.beginOrderPaymentCalls)
	}
	if repo.completeOrderPaymentCalls != 2 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 2", repo.completeOrderPaymentCalls)
	}
	if charger.debitCalls != 2 {
		t.Fatalf("DebitCredits() calls = %d, want 2", charger.debitCalls)
	}
	for i, debit := range charger.debitCommands {
		if debit.SourceEventID != "mall.order.pay:811:pay-811" {
			t.Fatalf("DebitCredits() call %d source event = %q, want mall.order.pay:811:pay-811", i+1, debit.SourceEventID)
		}
	}

	if _, err := svc.PayOrder(context.Background(), command); err != nil {
		t.Fatalf("settled PayOrder() error = %v", err)
	}
	if repo.completeOrderPaymentCalls != 2 || charger.debitCalls != 2 {
		t.Fatalf("settled order retried completion=%d debits=%d, want 2 each", repo.completeOrderPaymentCalls, charger.debitCalls)
	}
}

func TestPayOrderReturnsCompletedOrderWithoutDuplicateDebit(t *testing.T) {
	completedAt := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           814,
			OrderNo:      "M814",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusCompleted,
			PaidAt:       &completedAt,
			CompletedAt:  &completedAt,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{
					ProductID: 101,
					SKU:       "VIP-MONTH",
					Title:     "会员月卡",
					Quantity:  1,
					Code:      "BBS-EXISTING",
					IssuedAt:  completedAt,
				},
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	order, err := svc.PayOrder(context.Background(), PayOrderCommand{
		OrderID:        814,
		UserID:         7,
		PaymentMethod:  domain.PaymentProviderCredits,
		IdempotencyKey: "pay-814",
	})
	if err != nil {
		t.Fatalf("PayOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("PayOrder() status = %q, want completed", order.Status)
	}
	if len(order.DigitalEntitlements) != 1 || order.DigitalEntitlements[0].Code != "BBS-EXISTING" {
		t.Fatalf("PayOrder() entitlements = %+v, want existing entitlement", order.DigitalEntitlements)
	}
	if repo.beginOrderPaymentCalls != 1 {
		t.Fatalf("BeginOrderPayment() calls = %d, want 1", repo.beginOrderPaymentCalls)
	}
	if repo.completeOrderPaymentCalls != 0 {
		t.Fatalf("CompleteOrderPayment() calls = %d, want 0", repo.completeOrderPaymentCalls)
	}
	if charger.debitCalls != 0 {
		t.Fatalf("DebitCredits() calls = %d, want 0", charger.debitCalls)
	}
}

func TestListDigitalEntitlementsReturnsUserGrants(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			UserID: 7,
			DigitalEntitlements: []domain.DigitalEntitlement{
				{ID: 501, OrderID: 9001, ProductID: 101, GrantType: "membership", GrantKey: "vip-month", Status: domain.DigitalEntitlementStatusActive},
			},
		},
	}
	service := NewService(repo, nil, time.Minute)

	items, total, err := service.ListDigitalEntitlements(context.Background(), ListDigitalEntitlementsCommand{
		UserID: 7,
		Status: domain.DigitalEntitlementStatusActive,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListDigitalEntitlements() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("ListDigitalEntitlements() = total %d items %d, want 1 item", total, len(items))
	}
	if items[0].GrantType != "membership" || items[0].GrantKey != "vip-month" {
		t.Fatalf("grant = (%q, %q), want (membership, vip-month)", items[0].GrantType, items[0].GrantKey)
	}
}

func TestNewOrderPaidEventUsesUserMessageKeyAndPayload(t *testing.T) {
	paidAt := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
	event, err := newOrderPaidEvent(domain.Order{
		ID:           811,
		OrderNo:      "M811",
		UserID:       7,
		TotalCredits: 120,
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", Quantity: 1, UnitPriceCredits: 120, SubtotalCredits: 120},
		},
	}, domain.Payment{
		ID:       9101,
		Provider: domain.PaymentProviderCredits,
	}, paidAt)
	if err != nil {
		t.Fatalf("newOrderPaidEvent() error = %v", err)
	}
	if event.EventType != OrderPaidEventType {
		t.Fatalf("event type = %q, want %q", event.EventType, OrderPaidEventType)
	}
	if event.MessageKey != "7" {
		t.Fatalf("message key = %q, want user id", event.MessageKey)
	}
	var payload orderPaidEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.OrderID != 811 || payload.UserID != 7 || payload.TotalCredits != 120 || payload.PaymentID != 9101 {
		t.Fatalf("payload = %+v, want paid order fields", payload)
	}
	if payload.PaymentMethod != domain.PaymentProviderCredits {
		t.Fatalf("payment method = %q, want credits", payload.PaymentMethod)
	}
	if len(payload.Items) != 1 || payload.Items[0].ProductID != 101 || payload.Items[0].Quantity != 1 {
		t.Fatalf("payload items = %+v, want one paid item", payload.Items)
	}
	if payload.Items[0].Category != "digital" {
		t.Fatalf("payload item category = %q, want digital", payload.Items[0].Category)
	}
}

func TestRecoverStalePayingOrdersRetriesWithStableCreditSourceEvent(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           812,
			OrderNo:      "M812",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaying,
		},
		payment: domain.Payment{
			ID:             9102,
			OrderID:        812,
			UserID:         7,
			AmountCredits:  120,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-812",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{
			{
				OrderID:        812,
				UserID:         7,
				PaymentID:      9102,
				Provider:       domain.PaymentProviderCredits,
				IdempotencyKey: "pay-812",
			},
		},
	}
	charger := &creditChargerStub{}
	svc := NewService(repo, charger, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{
		StaleAfter: time.Minute,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 1 || result.Failed != 0 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=1 failed=0", result)
	}
	if repo.order.Status != domain.OrderStatusPaid {
		t.Fatalf("order status = %q, want paid", repo.order.Status)
	}
	if repo.beginOrderPaymentCalls != 1 || repo.completeOrderPaymentCalls != 1 {
		t.Fatalf("payment calls begin=%d complete=%d, want 1 each", repo.beginOrderPaymentCalls, repo.completeOrderPaymentCalls)
	}
	if repo.failOrderPaymentCalls != 0 {
		t.Fatalf("FailOrderPayment() calls = %d, want 0", repo.failOrderPaymentCalls)
	}
	if charger.debitCalls != 1 {
		t.Fatalf("DebitCredits() calls = %d, want 1", charger.debitCalls)
	}
	if charger.debitCommand.SourceEventID != "mall.order.pay:812:pay-812" {
		t.Fatalf("DebitCredits() source event = %q, want mall.order.pay:812:pay-812", charger.debitCommand.SourceEventID)
	}
}

func TestRecoverStalePayingOrdersKeepsPayingWhenDebitFails(t *testing.T) {
	debitErr := errors.New("credit service unavailable")
	repo := &orderRepoStub{
		order: domain.Order{
			ID:           813,
			OrderNo:      "M813",
			UserID:       7,
			TotalCredits: 120,
			Status:       domain.OrderStatusPaying,
		},
		payment: domain.Payment{
			ID:             9103,
			OrderID:        813,
			UserID:         7,
			AmountCredits:  120,
			Provider:       domain.PaymentProviderCredits,
			IdempotencyKey: "pay-813",
			Status:         domain.PaymentStatusPending,
		},
		stalePayingOrders: []domain.PayingOrderPayment{
			{
				OrderID:        813,
				UserID:         7,
				PaymentID:      9103,
				Provider:       domain.PaymentProviderCredits,
				IdempotencyKey: "pay-813",
			},
		},
	}
	charger := &creditChargerStub{debitErr: debitErr}
	svc := NewService(repo, charger, time.Minute)

	result, err := svc.RecoverStalePayingOrders(context.Background(), RecoverStalePayingOrdersCommand{
		StaleAfter: time.Minute,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("RecoverStalePayingOrders() error = %v", err)
	}
	if result.Recovered != 0 || result.Failed != 1 {
		t.Fatalf("RecoverStalePayingOrders() result = %+v, want recovered=0 failed=1", result)
	}
	if repo.order.Status != domain.OrderStatusPaying {
		t.Fatalf("order status = %q, want paying", repo.order.Status)
	}
	if repo.payment.Status != domain.PaymentStatusPending {
		t.Fatalf("payment status = %q, want pending", repo.payment.Status)
	}
	if repo.failOrderPaymentCalls != 0 {
		t.Fatalf("FailOrderPayment() calls = %d, want 0", repo.failOrderPaymentCalls)
	}
}

func TestAdminUpdateOrderStatusRequiresTrackingForShippedPhysicalOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: physicalPaidOrder(901),
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 901,
		Status:  domain.OrderStatusShipped,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminUpdateOrderStatus() error = %v, want invalid order state", err)
	}
	if repo.adminUpdateOrderStatusCalls != 0 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 0", repo.adminUpdateOrderStatusCalls)
	}
}

func TestAdminUpdateOrderStatusAllowsShippedPhysicalOrderWithTracking(t *testing.T) {
	repo := &orderRepoStub{
		order: physicalPaidOrder(902),
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID:         902,
		Status:          domain.OrderStatusShipped,
		OperatorID:      "ops",
		ShippingCarrier: " 顺丰 ",
		TrackingNo:      " SF123 ",
		Note:            " 已发货 ",
	})
	if err != nil {
		t.Fatalf("AdminUpdateOrderStatus() error = %v", err)
	}
	if order.Status != domain.OrderStatusShipped {
		t.Fatalf("AdminUpdateOrderStatus() status = %q, want shipped", order.Status)
	}
	if repo.adminUpdateOrderStatusCalls != 1 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 1", repo.adminUpdateOrderStatusCalls)
	}
	if repo.adminFulfillment.ShippingCarrier != "顺丰" || repo.adminFulfillment.TrackingNo != "SF123" {
		t.Fatalf("AdminUpdateOrderStatus() fulfillment = %+v, want trimmed carrier/tracking", repo.adminFulfillment)
	}
	if repo.adminNote != "已发货" {
		t.Fatalf("AdminUpdateOrderStatus() note = %q, want trimmed note", repo.adminNote)
	}
}

func TestAdminUpdateOrderStatusRequiresEvidenceForCompletingPhysicalOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: physicalPaidOrder(903),
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 903,
		Status:  domain.OrderStatusCompleted,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("AdminUpdateOrderStatus() error = %v, want invalid order state", err)
	}
	if repo.adminUpdateOrderStatusCalls != 0 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 0", repo.adminUpdateOrderStatusCalls)
	}
}

func TestAdminUpdateOrderStatusAllowsCompletingDigitalOrderWithoutFulfillment(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:      904,
			OrderNo: "M904",
			UserID:  7,
			Status:  domain.OrderStatusPaid,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.AdminUpdateOrderStatus(context.Background(), AdminUpdateOrderStatusCommand{
		OrderID: 904,
		Status:  domain.OrderStatusCompleted,
	})
	if err != nil {
		t.Fatalf("AdminUpdateOrderStatus() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("AdminUpdateOrderStatus() status = %q, want completed", order.Status)
	}
	if repo.adminUpdateOrderStatusCalls != 1 {
		t.Fatalf("AdminUpdateOrderStatus() calls = %d, want 1", repo.adminUpdateOrderStatusCalls)
	}
}

func TestConfirmOrderCompletesShippedOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:              1001,
			OrderNo:         "M1001",
			UserID:          7,
			Status:          domain.OrderStatusShipped,
			ShippingCarrier: "顺丰",
			TrackingNo:      "SF1001",
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.ConfirmOrder(context.Background(), ConfirmOrderCommand{
		OrderID: 1001,
		UserID:  7,
	})
	if err != nil {
		t.Fatalf("ConfirmOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("ConfirmOrder() status = %q, want completed", order.Status)
	}
	if repo.confirmOrderCalls != 1 {
		t.Fatalf("ConfirmOrder() calls = %d, want 1", repo.confirmOrderCalls)
	}
	if repo.confirmEvent.EventType != OrderCompletedEventType {
		t.Fatalf("ConfirmOrder() event type = %q, want %q", repo.confirmEvent.EventType, OrderCompletedEventType)
	}
}

func TestConfirmOrderRejectsUnshippedOrder(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     1002,
			UserID: 7,
			Status: domain.OrderStatusPaid,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	_, err := svc.ConfirmOrder(context.Background(), ConfirmOrderCommand{
		OrderID: 1002,
		UserID:  7,
	})
	if !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("ConfirmOrder() error = %v, want invalid order state", err)
	}
	if repo.confirmOrderCalls != 0 {
		t.Fatalf("ConfirmOrder() calls = %d, want 0", repo.confirmOrderCalls)
	}
}

func TestConfirmOrderReturnsCompletedOrderWithoutDuplicateWrite(t *testing.T) {
	repo := &orderRepoStub{
		order: domain.Order{
			ID:     1003,
			UserID: 7,
			Status: domain.OrderStatusCompleted,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	order, err := svc.ConfirmOrder(context.Background(), ConfirmOrderCommand{
		OrderID: 1003,
		UserID:  7,
	})
	if err != nil {
		t.Fatalf("ConfirmOrder() error = %v", err)
	}
	if order.Status != domain.OrderStatusCompleted {
		t.Fatalf("ConfirmOrder() status = %q, want completed", order.Status)
	}
	if repo.confirmOrderCalls != 0 {
		t.Fatalf("ConfirmOrder() calls = %d, want 0", repo.confirmOrderCalls)
	}
}

func TestCreateProductReviewStartsPendingReview(t *testing.T) {
	repo := &orderRepoStub{}
	svc := NewService(repo, nil, time.Minute)

	review, err := svc.CreateProductReview(context.Background(), CreateProductReviewCommand{
		UserID:    7,
		ProductID: 101,
		OrderID:   9001,
		Rating:    5,
		Content:   "  很好用，晒单图片见正文  ",
	})
	if err != nil {
		t.Fatalf("CreateProductReview() error = %v", err)
	}
	if repo.createProductReviewCalls != 1 {
		t.Fatalf("CreateProductReview() calls = %d, want 1", repo.createProductReviewCalls)
	}
	if review.Status != domain.ProductReviewStatusPending {
		t.Fatalf("CreateProductReview() status = %q, want pending", review.Status)
	}
	if review.Content != "很好用，晒单图片见正文" {
		t.Fatalf("CreateProductReview() content = %q, want trimmed content", review.Content)
	}
}

func TestAdminUpdateProductReviewStatusEmitsOutboxEvent(t *testing.T) {
	repo := &orderRepoStub{
		productReview: domain.ProductReview{
			ID:           9004,
			ProductID:    101,
			ProductTitle: "主题皮肤",
			OrderID:      9001,
			UserID:       7,
			Status:       domain.ProductReviewStatusPending,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	review, err := svc.AdminUpdateProductReviewStatus(context.Background(), AdminUpdateProductReviewStatusCommand{
		ID:     9004,
		Status: domain.ProductReviewStatusPublished,
	})
	if err != nil {
		t.Fatalf("AdminUpdateProductReviewStatus() error = %v", err)
	}
	if review.Status != domain.ProductReviewStatusPublished {
		t.Fatalf("review status = %q, want published", review.Status)
	}
	if repo.adminUpdateProductReviewStatusCalls != 1 {
		t.Fatalf("AdminUpdateProductReviewStatus() calls = %d, want 1", repo.adminUpdateProductReviewStatusCalls)
	}
	if repo.productReviewEvent.EventType != ReviewPublishedEventType {
		t.Fatalf("event type = %q, want %q", repo.productReviewEvent.EventType, ReviewPublishedEventType)
	}
	var payload productReviewStatusEventPayload
	if err := json.Unmarshal(repo.productReviewEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if payload.ReviewID != 9004 || payload.ProductID != 101 || payload.UserID != 7 || payload.Status != string(domain.ProductReviewStatusPublished) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestAdminUpdateProductReviewStatusSkipsEventForUnchangedStatus(t *testing.T) {
	repo := &orderRepoStub{
		productReview: domain.ProductReview{
			ID:        9005,
			ProductID: 102,
			OrderID:   9002,
			UserID:    8,
			Status:    domain.ProductReviewStatusPublished,
		},
	}
	svc := NewService(repo, nil, time.Minute)

	if _, err := svc.AdminUpdateProductReviewStatus(context.Background(), AdminUpdateProductReviewStatusCommand{
		ID:     9005,
		Status: domain.ProductReviewStatusPublished,
	}); err != nil {
		t.Fatalf("AdminUpdateProductReviewStatus() error = %v", err)
	}
	if repo.productReviewEvent.EventID != "" {
		t.Fatalf("event = %+v, want empty", repo.productReviewEvent)
	}
}

func physicalPaidOrder(id int64) domain.Order {
	return domain.Order{
		ID:       id,
		OrderNo:  "M-PHYSICAL",
		UserID:   7,
		Status:   domain.OrderStatusPaid,
		Receiver: "张三",
		Phone:    "13800000000",
		Address:  "上海市浦东新区测试路 1 号",
	}
}

type orderRepoStub struct {
	domain.Repository

	products                            map[int64]domain.Product
	idempotencyOrders                   map[string]domain.Order
	cartItems                           []domain.CartItem
	order                               domain.Order
	refund                              domain.RefundRequest
	productReview                       domain.ProductReview
	createOrderCalls                    int
	createOrderFromCartCalls            int
	createRefundRequestCalls            int
	adminUpdateOrderStatusCalls         int
	adminUpdateProductReviewStatusCalls int
	adminFulfillment                    domain.OrderFulfillment
	adminNote                           string
	confirmOrderCalls                   int
	confirmEvent                        domain.OutboxEvent
	createProductReviewCalls            int
	productReviewEvent                  domain.OutboxEvent
	startRefundApprovalCalls            int
	completeRefundApprovalCalls         int
	completeRefundApprovalFailures      int
	completeRefundEvent                 domain.OutboxEvent
	rejectRefundRequestCalls            int
	payment                             domain.Payment
	stalePayingOrders                   []domain.PayingOrderPayment
	listStalePayingOrdersCalls          int
	stalePayingStartedBefore            time.Time
	stalePayingLimit                    int
	beginOrderPaymentCalls              int
	completeOrderPaymentCalls           int
	completeOrderPaymentFailures        int
	failOrderPaymentCalls               int
	failPaymentReason                   string
}

func (r *orderRepoStub) GetOrderByIdempotencyKey(_ context.Context, userID int64, idempotencyKey string) (domain.Order, error) {
	if order, ok := r.idempotencyOrders[fmt.Sprintf("%d:%s", userID, idempotencyKey)]; ok {
		return order, nil
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (r *orderRepoStub) GetProduct(_ context.Context, productID int64) (domain.Product, error) {
	if product, ok := r.products[productID]; ok {
		return product, nil
	}
	return domain.Product{}, domain.ErrProductNotFound
}

func (r *orderRepoStub) CreateOrder(_ context.Context, order domain.Order) (domain.Order, bool, error) {
	r.createOrderCalls++
	order.ID = 9001
	return order, false, nil
}

func (r *orderRepoStub) ListCartItems(_ context.Context, _ int64) ([]domain.CartItem, int64, error) {
	return r.cartItems, int64(len(r.cartItems)), nil
}

func (r *orderRepoStub) CreateOrderFromCart(_ context.Context, order domain.Order) (domain.Order, bool, error) {
	r.createOrderFromCartCalls++
	order.ID = 9002
	return order, false, nil
}

func (r *orderRepoStub) GetOrder(_ context.Context, orderID int64) (domain.Order, error) {
	if r.order.ID == orderID {
		return r.order, nil
	}
	return domain.Order{}, domain.ErrOrderNotFound
}

func (r *orderRepoStub) ListDigitalEntitlementsByUser(_ context.Context, query domain.DigitalEntitlementListQuery) ([]domain.DigitalEntitlement, int64, error) {
	if query.UserID != r.order.UserID {
		return nil, 0, nil
	}
	return r.order.DigitalEntitlements, int64(len(r.order.DigitalEntitlements)), nil
}

func (r *orderRepoStub) CreateRefundRequest(_ context.Context, refund domain.RefundRequest) (domain.RefundRequest, bool, error) {
	r.createRefundRequestCalls++
	if r.refund.OrderID == refund.OrderID {
		return r.refund, true, nil
	}
	refund.ID = 9003
	return refund, false, nil
}

func (r *orderRepoStub) CreateProductReview(_ context.Context, review domain.ProductReview) (domain.ProductReview, error) {
	r.createProductReviewCalls++
	review.ID = 9004
	return review, nil
}

func (r *orderRepoStub) GetProductReview(_ context.Context, reviewID int64) (domain.ProductReview, error) {
	if r.productReview.ID == reviewID {
		return r.productReview, nil
	}
	return domain.ProductReview{}, domain.ErrProductReviewNotFound
}

func (r *orderRepoStub) AdminUpdateProductReviewStatus(_ context.Context, reviewID int64, status domain.ProductReviewStatus, updatedAt time.Time, event domain.OutboxEvent) (domain.ProductReview, error) {
	r.adminUpdateProductReviewStatusCalls++
	if r.productReview.ID != reviewID {
		return domain.ProductReview{}, domain.ErrProductReviewNotFound
	}
	r.productReview.Status = status
	r.productReview.UpdatedAt = updatedAt
	r.productReviewEvent = event
	return r.productReview, nil
}

func (r *orderRepoStub) AdminUpdateOrderStatus(_ context.Context, orderID int64, nextStatus domain.OrderStatus, _ string, fulfillment domain.OrderFulfillment, note string, changedAt time.Time, _ domain.OutboxEvent) (domain.Order, error) {
	r.adminUpdateOrderStatusCalls++
	if r.order.ID != orderID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	r.adminFulfillment = fulfillment
	r.adminNote = note
	r.order.Status = nextStatus
	if fulfillment.ShippingCarrier != "" {
		r.order.ShippingCarrier = fulfillment.ShippingCarrier
	}
	if fulfillment.TrackingNo != "" {
		r.order.TrackingNo = fulfillment.TrackingNo
	}
	if nextStatus == domain.OrderStatusShipped {
		r.order.ShippedAt = &changedAt
	}
	if nextStatus == domain.OrderStatusCompleted {
		r.order.CompletedAt = &changedAt
	}
	return r.order, nil
}

func (r *orderRepoStub) ConfirmOrder(_ context.Context, orderID, userID int64, completedAt time.Time, event domain.OutboxEvent) (domain.Order, error) {
	r.confirmOrderCalls++
	r.confirmEvent = event
	if r.order.ID != orderID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if r.order.Status != domain.OrderStatusShipped {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	r.order.Status = domain.OrderStatusCompleted
	r.order.CompletedAt = &completedAt
	return r.order, nil
}

func (r *orderRepoStub) GetRefundRequest(_ context.Context, refundID int64) (domain.RefundRequest, error) {
	if r.refund.ID == refundID {
		return r.refund, nil
	}
	return domain.RefundRequest{}, domain.ErrRefundNotFound
}

func (r *orderRepoStub) StartRefundApproval(_ context.Context, refundID int64, operatorID, adminNote string, restoreStock bool, reviewedAt time.Time) (domain.RefundRequest, error) {
	r.startRefundApprovalCalls++
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
	}
	if r.refund.Status == domain.RefundStatusRequested {
		r.refund.Status = domain.RefundStatusProcessing
		r.refund.OperatorID = operatorID
		r.refund.AdminNote = adminNote
		r.refund.RestoreStock = restoreStock
		r.refund.ReviewedAt = &reviewedAt
	}
	return r.refund, nil
}

func (r *orderRepoStub) CompleteRefundApproval(_ context.Context, refundID int64, reviewedAt time.Time, event domain.OutboxEvent) (domain.RefundRequest, error) {
	r.completeRefundApprovalCalls++
	r.completeRefundEvent = event
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
	}
	if r.completeRefundApprovalFailures > 0 {
		r.completeRefundApprovalFailures--
		return domain.RefundRequest{}, errors.New("complete refund approval failed")
	}
	r.refund.Status = domain.RefundStatusApproved
	r.refund.RefundedAt = &reviewedAt
	return r.refund, nil
}

func (r *orderRepoStub) RejectRefundRequest(_ context.Context, refundID int64, operatorID, adminNote string, reviewedAt time.Time, _ domain.OutboxEvent) (domain.RefundRequest, error) {
	r.rejectRefundRequestCalls++
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
	}
	r.refund.Status = domain.RefundStatusRejected
	r.refund.OperatorID = operatorID
	r.refund.AdminNote = adminNote
	r.refund.ReviewedAt = &reviewedAt
	return r.refund, nil
}

func (r *orderRepoStub) CloseExpiredOrder(_ context.Context, orderID, userID int64, _ time.Time, _ time.Time) (domain.Order, bool, error) {
	if r.order.ID != orderID {
		return domain.Order{}, false, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, false, domain.ErrOrderOwnerMismatch
	}
	return r.order, false, nil
}

func (r *orderRepoStub) ListStalePayingOrders(_ context.Context, startedBefore time.Time, limit int) ([]domain.PayingOrderPayment, error) {
	r.listStalePayingOrdersCalls++
	r.stalePayingStartedBefore = startedBefore
	r.stalePayingLimit = limit
	return r.stalePayingOrders, nil
}

func (r *orderRepoStub) BeginOrderPayment(_ context.Context, orderID, userID int64, paymentMethod, idempotencyKey string, now time.Time) (domain.Order, domain.Payment, error) {
	r.beginOrderPaymentCalls++
	if r.order.ID != orderID {
		return domain.Order{}, domain.Payment{}, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, domain.Payment{}, domain.ErrOrderOwnerMismatch
	}
	if r.order.Status == domain.OrderStatusPaid || r.order.Status == domain.OrderStatusCompleted {
		return r.order, domain.Payment{}, nil
	}
	if r.order.Status != domain.OrderStatusPendingPayment && r.order.Status != domain.OrderStatusPaying {
		return domain.Order{}, domain.Payment{}, domain.ErrInvalidOrderState
	}
	if r.payment.ID == 0 {
		r.payment = domain.Payment{
			ID:             9101,
			OrderID:        orderID,
			UserID:         userID,
			AmountCredits:  r.order.TotalCredits,
			Provider:       paymentMethod,
			IdempotencyKey: idempotencyKey,
			Status:         domain.PaymentStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}
	r.order.Status = domain.OrderStatusPaying
	r.order.PaymentMethod = paymentMethod
	r.order.UpdatedAt = now
	return r.order, r.payment, nil
}

func (r *orderRepoStub) CompleteOrderPayment(_ context.Context, orderID, userID, paymentID int64, paidAt time.Time, _ domain.OutboxEvent) (domain.Order, error) {
	r.completeOrderPaymentCalls++
	if r.order.ID != orderID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.Order{}, domain.ErrOrderOwnerMismatch
	}
	if r.payment.ID != paymentID {
		return domain.Order{}, domain.ErrInvalidOrderState
	}
	if r.completeOrderPaymentFailures > 0 {
		r.completeOrderPaymentFailures--
		return domain.Order{}, errors.New("complete order payment failed")
	}
	r.order.Status = domain.OrderStatusPaid
	r.order.PaidAt = &paidAt
	r.order.UpdatedAt = paidAt
	r.payment.Status = domain.PaymentStatusSucceeded
	r.payment.PaidAt = &paidAt
	r.payment.UpdatedAt = paidAt
	return r.order, nil
}

func (r *orderRepoStub) FailOrderPayment(_ context.Context, orderID, userID, paymentID int64, reason string, failedAt time.Time) error {
	r.failOrderPaymentCalls++
	if r.order.ID != orderID {
		return domain.ErrOrderNotFound
	}
	if r.order.UserID != userID {
		return domain.ErrOrderOwnerMismatch
	}
	if r.payment.ID != paymentID {
		return domain.ErrInvalidOrderState
	}
	r.failPaymentReason = reason
	r.order.Status = domain.OrderStatusPendingPayment
	r.order.UpdatedAt = failedAt
	r.payment.Status = domain.PaymentStatusFailed
	r.payment.FailureReason = reason
	r.payment.UpdatedAt = failedAt
	return nil
}

type creditChargerStub struct {
	adjustCalls    int
	adjustCommand  CreditAdjustCommand
	adjustCommands []CreditAdjustCommand
	debitCalls     int
	debitCommand   CreditDebitCommand
	debitCommands  []CreditDebitCommand
	debitErr       error
}

func (c *creditChargerStub) DebitCredits(_ context.Context, command CreditDebitCommand) error {
	c.debitCalls++
	c.debitCommand = command
	c.debitCommands = append(c.debitCommands, command)
	return c.debitErr
}

func (c *creditChargerStub) AdjustCredits(_ context.Context, command CreditAdjustCommand) error {
	c.adjustCalls++
	c.adjustCommand = command
	c.adjustCommands = append(c.adjustCommands, command)
	return nil
}
