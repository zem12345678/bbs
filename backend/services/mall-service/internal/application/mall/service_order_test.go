package mall

import (
	"context"
	"errors"
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

func TestAdminReviewRefundRequestRetriesProcessingRefund(t *testing.T) {
	repo := &orderRepoStub{
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

	products                    map[int64]domain.Product
	cartItems                   []domain.CartItem
	order                       domain.Order
	refund                      domain.RefundRequest
	createOrderCalls            int
	createOrderFromCartCalls    int
	createRefundRequestCalls    int
	adminUpdateOrderStatusCalls int
	adminFulfillment            domain.OrderFulfillment
	adminNote                   string
	confirmOrderCalls           int
	confirmEvent                domain.OutboxEvent
	startRefundApprovalCalls    int
	completeRefundApprovalCalls int
	rejectRefundRequestCalls    int
}

func (r *orderRepoStub) GetOrderByIdempotencyKey(context.Context, string) (domain.Order, error) {
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

func (r *orderRepoStub) CreateRefundRequest(_ context.Context, refund domain.RefundRequest) (domain.RefundRequest, bool, error) {
	r.createRefundRequestCalls++
	refund.ID = 9003
	return refund, false, nil
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

func (r *orderRepoStub) CompleteRefundApproval(_ context.Context, refundID int64, reviewedAt time.Time, _ domain.OutboxEvent) (domain.RefundRequest, error) {
	r.completeRefundApprovalCalls++
	if r.refund.ID != refundID {
		return domain.RefundRequest{}, domain.ErrRefundNotFound
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

type creditChargerStub struct {
	adjustCalls   int
	adjustCommand CreditAdjustCommand
}

func (c *creditChargerStub) DebitCredits(context.Context, CreditDebitCommand) error {
	return nil
}

func (c *creditChargerStub) AdjustCredits(_ context.Context, command CreditAdjustCommand) error {
	c.adjustCalls++
	c.adjustCommand = command
	return nil
}
