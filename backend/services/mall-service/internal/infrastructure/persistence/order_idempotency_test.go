package persistence

import (
	"errors"
	"testing"

	domain "mall-service/internal/domain/mall"
)

func TestIdempotentExistingOrderRequiresSameRequest(t *testing.T) {
	existing := domain.Order{
		ID:             9001,
		UserID:         7,
		IdempotencyKey: "same-key",
		CouponCode:     "SAVE10",
		Receiver:       "Alice",
		Phone:          "13800138000",
		Address:        "No.1 Road",
		Items:          []domain.OrderItem{{ProductID: 101, Quantity: 1}},
	}

	order, duplicate, err := idempotentExistingOrder(existing, domain.Order{
		UserID:         7,
		IdempotencyKey: "same-key",
		CouponCode:     " save10 ",
		Receiver:       " Alice ",
		Phone:          " 13800138000 ",
		Address:        " No.1 Road ",
		Items:          []domain.OrderItem{{ProductID: 101, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("idempotentExistingOrder() error = %v", err)
	}
	if !duplicate || order.ID != existing.ID {
		t.Fatalf("idempotentExistingOrder() = %+v duplicate=%v, want existing duplicate", order, duplicate)
	}

	_, duplicate, err = idempotentExistingOrder(existing, domain.Order{
		UserID:         7,
		IdempotencyKey: "same-key",
		CouponCode:     "SAVE10",
		Receiver:       "Alice",
		Phone:          "13800138000",
		Address:        "No.1 Road",
		Items:          []domain.OrderItem{{ProductID: 102, Quantity: 1}},
	})
	if !errors.Is(err, domain.ErrDuplicateReference) {
		t.Fatalf("idempotentExistingOrder() error = %v, want duplicate reference", err)
	}
	if duplicate {
		t.Fatal("idempotentExistingOrder() duplicate = true, want false for mismatched request")
	}
}
