package persistence

import (
	"strings"
	"testing"
)

func TestAccountErasureSchemaContracts(t *testing.T) {
	schema := strings.Join(accountErasureSchemaStatements, "\n")
	for _, fragment := range []string{
		"CREATE SEQUENCE IF NOT EXISTS mall_erased_user_id_seq AS BIGINT",
		"INCREMENT BY -1",
		"CREATE TABLE IF NOT EXISTS mall_erased_users",
		"anonymized_user_id < 0",
		"user_id <> 0",
		"CREATE OR REPLACE FUNCTION mall_reject_erased_identity()",
		"bbs-mall-user:",
		"mall_orders_erased_user_guard",
		"mall_payments_erased_user_guard",
		"mall_coupon_usages_erased_user_guard",
		"mall_cart_items_erased_user_guard",
		"mall_product_favorites_erased_user_guard",
		"mall_addresses_erased_user_guard",
		"mall_refund_requests_erased_user_guard",
		"mall_digital_entitlements_erased_user_guard",
		"mall_product_reviews_erased_user_guard",
	} {
		if !strings.Contains(schema, fragment) {
			t.Fatalf("account erasure schema missing %q", fragment)
		}
	}
	for _, constraint := range []string{
		"mall_digital_entitlements_order_user_fkey",
		"mall_payments_order_user_fkey",
		"mall_coupon_usages_order_user_fkey",
		"mall_coupon_usages_order_coupon_snapshot_fkey",
		"mall_refund_requests_order_user_fkey",
		"mall_refund_requests_order_snapshot_fkey",
		"mall_digital_entitlements_refund_order_user_fkey",
		"mall_product_reviews_order_user_fkey",
	} {
		if !strings.Contains(schema, constraint) {
			t.Fatalf("account erasure schema missing deferred constraint %q", constraint)
		}
	}
	if count := strings.Count(schema, "DEFERRABLE INITIALLY DEFERRED"); count != 1 {
		t.Fatalf("schema deferral command count = %d, want shared loop command", count)
	}
}
