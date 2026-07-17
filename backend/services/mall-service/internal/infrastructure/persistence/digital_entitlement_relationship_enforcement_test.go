package persistence

import (
	"strings"
	"testing"
)

func TestDigitalEntitlementSchemaEnforcesOrderItemAndRefundLinks(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_digital_entitlements_order_item_fkey",
		"FOREIGN KEY (order_id, product_id) REFERENCES mall_order_items(order_id, product_id) ON DELETE CASCADE NOT VALID",
		"idx_mall_refund_requests_id_order_user",
		"mall_digital_entitlements_refund_order_user_fkey",
		"FOREIGN KEY (refund_id, order_id, user_id) REFERENCES mall_refund_requests(id, order_id, user_id) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing entitlement relationship enforcement %q", want)
		}
	}
}
