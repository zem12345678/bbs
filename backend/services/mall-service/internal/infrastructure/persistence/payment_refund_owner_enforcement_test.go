package persistence

import (
	"strings"
	"testing"
)

func TestPaymentAndRefundSchemasEnforceOrderOwner(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"idx_mall_orders_id_user",
		"mall_payments_order_user_fkey",
		"mall_refund_requests_order_user_fkey",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order owner enforcement %q", want)
		}
	}

	const ownerForeignKey = "FOREIGN KEY (order_id, user_id) REFERENCES mall_orders(id, user_id) ON DELETE CASCADE NOT VALID"
	if got := strings.Count(joined, ownerForeignKey); got < 3 {
		t.Fatalf("schemaStatements have %d order owner foreign keys, want at least 3", got)
	}
}

func TestRefundSchemaEnforcesOrderFinancialSnapshot(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"idx_mall_orders_refund_snapshot",
		"mall_refund_requests_order_snapshot_fkey",
		"FOREIGN KEY (order_id, user_id, order_no, amount_credits) REFERENCES mall_orders(id, user_id, order_no, total_credits) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing refund order snapshot enforcement %q", want)
		}
	}
}
