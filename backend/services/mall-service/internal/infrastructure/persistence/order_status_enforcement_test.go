package persistence

import (
	"errors"
	"testing"

	domain "mall-service/internal/domain/mall"
)

func TestValidatePersistedAdminOrderFulfillmentRequiresEvidenceForPhysicalItems(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 202, SKU: "HOODIE", Title: "社区卫衣", Category: "merch", Quantity: 1},
		},
	}

	if err := validatePersistedAdminOrderFulfillment(order, domain.OrderStatusShipped, domain.OrderFulfillment{}, ""); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("ship physical order error = %v, want invalid order state", err)
	}
	if err := validatePersistedAdminOrderFulfillment(order, domain.OrderStatusCompleted, domain.OrderFulfillment{}, ""); !errors.Is(err, domain.ErrInvalidOrderState) {
		t.Fatalf("complete physical order error = %v, want invalid order state", err)
	}
	if err := validatePersistedAdminOrderFulfillment(order, domain.OrderStatusShipped, domain.OrderFulfillment{TrackingNo: "SF123"}, ""); err != nil {
		t.Fatalf("ship physical order with tracking error = %v, want nil", err)
	}
	if err := validatePersistedAdminOrderFulfillment(order, domain.OrderStatusCompleted, domain.OrderFulfillment{}, "manual delivery evidence"); err != nil {
		t.Fatalf("complete physical order with note error = %v, want nil", err)
	}
}

func TestValidatePersistedAdminOrderFulfillmentAllowsDigitalOrdersWithoutEvidence(t *testing.T) {
	order := domain.Order{
		Items: []domain.OrderItem{
			{ProductID: 101, SKU: "VIP-MONTH", Title: "会员月卡", Category: "digital", GrantType: "membership", GrantKey: "vip-month", Quantity: 1},
		},
	}

	if err := validatePersistedAdminOrderFulfillment(order, domain.OrderStatusCompleted, domain.OrderFulfillment{}, ""); err != nil {
		t.Fatalf("complete digital order error = %v, want nil", err)
	}
}
