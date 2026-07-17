package persistence

import (
	"strings"
	"testing"
)

func TestCouponUsageAndProductReviewSchemasEnforceOrderRelationships(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_coupon_usages_order_user_fkey",
		"mall_product_reviews_order_user_fkey",
		"mall_product_reviews_order_item_fkey",
		"FOREIGN KEY (order_id, product_id) REFERENCES mall_order_items(order_id, product_id) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order relationship enforcement %q", want)
		}
	}

	const orderOwnerForeignKey = "FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID"
	if got := strings.Count(joined, orderOwnerForeignKey); got < 5 {
		t.Fatalf("schemaStatements have %d order owner foreign keys, want at least 5", got)
	}
}
