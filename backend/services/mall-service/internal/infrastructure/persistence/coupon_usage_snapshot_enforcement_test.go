package persistence

import (
	"strings"
	"testing"
)

func TestCouponUsageSchemaEnforcesOrderSnapshot(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"idx_mall_orders_coupon_usage_snapshot",
		"mall_coupon_usages_order_coupon_snapshot_fkey",
		"FOREIGN KEY (order_id, user_id, coupon_id, code, discount_credits) REFERENCES mall_orders(id, user_id, coupon_id, coupon_code, discount_credits) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing coupon usage snapshot enforcement %q", want)
		}
	}
}

func TestOrderCouponSchemaEnforcesCouponIdentitySnapshot(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"idx_mall_coupons_id_code",
		"mall_orders_coupon_snapshot_fkey",
		"FOREIGN KEY (coupon_id, coupon_code) REFERENCES mall_coupons(id, code) NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order coupon snapshot enforcement %q", want)
		}
	}
}
