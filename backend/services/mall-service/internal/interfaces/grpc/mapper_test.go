package grpc

import (
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestOrderToPBIncludesDigitalEntitlements(t *testing.T) {
	issuedAt := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		DigitalEntitlements: []domain.DigitalEntitlement{
			{
				ProductID: 101,
				SKU:       "VIP-MONTH",
				Title:     "会员月卡",
				Quantity:  1,
				Code:      "BBS-ENTITLEMENT",
				IssuedAt:  issuedAt,
			},
		},
	}

	pbOrder := orderToPB(order)
	if len(pbOrder.GetDigitalEntitlements()) != 1 {
		t.Fatalf("digital entitlements length = %d, want 1", len(pbOrder.GetDigitalEntitlements()))
	}
	entitlement := pbOrder.GetDigitalEntitlements()[0]
	if entitlement.GetFulfillmentCode() != "BBS-ENTITLEMENT" {
		t.Fatalf("fulfillment code = %q, want BBS-ENTITLEMENT", entitlement.GetFulfillmentCode())
	}
	if entitlement.GetIssuedAt() != issuedAt.UnixMilli() {
		t.Fatalf("issued at = %d, want %d", entitlement.GetIssuedAt(), issuedAt.UnixMilli())
	}
}
