package persistence

import (
	"strings"
	"testing"
)

func TestOrderSchemaEnforcesFinancialCouponSnapshot(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_orders_financial_snapshot_check",
		"original_credits >= discount_credits",
		"total_credits = original_credits - discount_credits",
		"coupon_id IS NULL AND coupon_code = '' AND discount_credits = 0",
		"coupon_id IS NOT NULL",
		"coupon_code = UPPER(TRIM(coupon_code))",
		"mall_orders_coupon_id_fkey",
		"FOREIGN KEY (coupon_id) REFERENCES mall_coupons(id) NOT VALID",
		"mall_order_items_financial_snapshot_check",
		"subtotal_credits = quantity::BIGINT * unit_price_credits",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order financial constraint %q", want)
		}
	}
}
