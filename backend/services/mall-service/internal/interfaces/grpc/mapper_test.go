package grpc

import (
	"testing"
	"time"

	pb "mall-service/api/proto/mallpb"
	domain "mall-service/internal/domain/mall"
)

func TestOrderToPBIncludesDigitalEntitlements(t *testing.T) {
	issuedAt := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(30 * 24 * time.Hour)
	revokedAt := time.Date(2026, 7, 13, 9, 45, 0, 0, time.UTC)
	order := domain.Order{
		ID:     9001,
		UserID: 7,
		DigitalEntitlements: []domain.DigitalEntitlement{
			{
				ID:           501,
				OrderID:      9001,
				OrderNo:      "O-9001",
				UserID:       7,
				ProductID:    101,
				SKU:          "VIP-MONTH",
				Title:        "会员月卡",
				Quantity:     1,
				Code:         "BBS-ENTITLEMENT",
				GrantType:    "membership",
				GrantKey:     "vip-month",
				IssuedAt:     issuedAt,
				ExpiresAt:    &expiresAt,
				Status:       domain.DigitalEntitlementStatusRevoked,
				RevokedAt:    &revokedAt,
				RefundID:     7001,
				RevokedBy:    "admin-7",
				RevokeReason: "manual audit",
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
	if entitlement.GetId() != 501 || entitlement.GetOrderId() != 9001 || entitlement.GetOrderNo() != "O-9001" {
		t.Fatalf("trace fields = (%d, %d, %q), want (501, 9001, O-9001)", entitlement.GetId(), entitlement.GetOrderId(), entitlement.GetOrderNo())
	}
	if entitlement.GetUserId() != 7 {
		t.Fatalf("user id = %d, want 7", entitlement.GetUserId())
	}
	if entitlement.GetGrantType() != "membership" || entitlement.GetGrantKey() != "vip-month" {
		t.Fatalf("grant = (%q, %q), want (membership, vip-month)", entitlement.GetGrantType(), entitlement.GetGrantKey())
	}
	if entitlement.GetIssuedAt() != issuedAt.UnixMilli() {
		t.Fatalf("issued at = %d, want %d", entitlement.GetIssuedAt(), issuedAt.UnixMilli())
	}
	if entitlement.GetExpiresAt() != expiresAt.UnixMilli() {
		t.Fatalf("expires at = %d, want %d", entitlement.GetExpiresAt(), expiresAt.UnixMilli())
	}
	if entitlement.GetStatus() != domain.DigitalEntitlementStatusRevoked {
		t.Fatalf("status = %q, want %s", entitlement.GetStatus(), domain.DigitalEntitlementStatusRevoked)
	}
	if entitlement.GetRevokedAt() != revokedAt.UnixMilli() {
		t.Fatalf("revoked at = %d, want %d", entitlement.GetRevokedAt(), revokedAt.UnixMilli())
	}
	if entitlement.GetRefundId() != 7001 {
		t.Fatalf("refund id = %d, want 7001", entitlement.GetRefundId())
	}
	if entitlement.GetRevokedBy() != "admin-7" || entitlement.GetRevokeReason() != "manual audit" {
		t.Fatalf("revoke audit = (%q, %q), want (admin-7, manual audit)", entitlement.GetRevokedBy(), entitlement.GetRevokeReason())
	}
}

func TestDigitalEntitlementStatusForResponseMarksExpiredActiveGrant(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Minute)

	status := digitalEntitlementStatusForResponse(domain.DigitalEntitlement{
		Status:    domain.DigitalEntitlementStatusActive,
		ExpiresAt: &expiresAt,
	}, now)

	if status != domain.DigitalEntitlementStatusExpired {
		t.Fatalf("status = %q, want %s", status, domain.DigitalEntitlementStatusExpired)
	}
}

func TestDigitalEntitlementStatusForResponseKeepsRevokedBeforeExpired(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Minute)
	revokedAt := now.Add(-time.Hour)

	status := digitalEntitlementStatusForResponse(domain.DigitalEntitlement{
		Status:    domain.DigitalEntitlementStatusActive,
		ExpiresAt: &expiresAt,
		RevokedAt: &revokedAt,
	}, now)

	if status != domain.DigitalEntitlementStatusRevoked {
		t.Fatalf("status = %q, want %s", status, domain.DigitalEntitlementStatusRevoked)
	}
}

func TestRefundRequestToPBIncludesCanceledStatusAndTime(t *testing.T) {
	canceledAt := time.Date(2026, 7, 19, 10, 30, 0, 0, time.UTC)
	refund := refundRequestToPB(domain.RefundRequest{
		ID:         7603,
		Status:     domain.RefundStatusCanceled,
		CanceledAt: &canceledAt,
	})

	if refund.GetStatus() != pb.RefundStatus_REFUND_STATUS_CANCELED {
		t.Fatalf("refund status = %s, want canceled", refund.GetStatus())
	}
	if refund.GetCanceledAt() != canceledAt.UnixMilli() {
		t.Fatalf("refund canceled_at = %d, want %d", refund.GetCanceledAt(), canceledAt.UnixMilli())
	}
	if status := refundStatusFromPB(pb.RefundStatus_REFUND_STATUS_CANCELED); status != domain.RefundStatusCanceled {
		t.Fatalf("refundStatusFromPB() = %q, want canceled", status)
	}
}
