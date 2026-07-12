package persistence

import (
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestIsOrderExpiredForCloseSkipsPayingOrders(t *testing.T) {
	expireBefore := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)

	if !isOrderExpiredForClose(domain.Order{
		Status:    domain.OrderStatusPendingPayment,
		CreatedAt: expireBefore.Add(-time.Second),
	}, expireBefore) {
		t.Fatal("pending payment order before expiration cutoff should expire")
	}

	if isOrderExpiredForClose(domain.Order{
		Status:    domain.OrderStatusPaying,
		UpdatedAt: expireBefore.Add(-time.Hour),
	}, expireBefore) {
		t.Fatal("paying order should not be closed by unpaid-order expiration")
	}
}
