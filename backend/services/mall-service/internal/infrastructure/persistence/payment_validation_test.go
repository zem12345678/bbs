package persistence

import (
	"errors"
	"testing"

	domain "mall-service/internal/domain/mall"
)

func TestValidatePaymentForOrder(t *testing.T) {
	order := domain.Order{ID: 11, UserID: 7, TotalCredits: 80}
	valid := domain.Payment{OrderID: 11, UserID: 7, AmountCredits: 80}
	if err := validatePaymentForOrder(valid, order, 7); err != nil {
		t.Fatalf("validatePaymentForOrder() error = %v", err)
	}

	for _, payment := range []domain.Payment{
		{OrderID: 12, UserID: 7, AmountCredits: 80},
		{OrderID: 11, UserID: 8, AmountCredits: 80},
		{OrderID: 11, UserID: 7, AmountCredits: 81},
	} {
		if err := validatePaymentForOrder(payment, order, 7); !errors.Is(err, domain.ErrInvalidOrderState) {
			t.Fatalf("validatePaymentForOrder(%+v) error = %v, want invalid order state", payment, err)
		}
	}
}
