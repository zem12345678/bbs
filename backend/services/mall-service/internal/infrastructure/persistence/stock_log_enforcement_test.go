package persistence

import (
	"strings"
	"testing"
)

func TestProductStockLogSchemaEnforcesSnapshotArithmetic(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_product_stock_logs_snapshot_check",
		"after_stock = before_stock + delta",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing stock log constraint %q", want)
		}
	}
}

func TestProductStockLogSchemaEnforcesAuditContract(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_product_stock_logs_audit_contract_check",
		"reason = LOWER(TRIM(reason))",
		"reference_type = LOWER(TRIM(reference_type))",
		"reference_id > 0",
		"operator_type = LOWER(TRIM(operator_type))",
		"reason = 'product_created' AND reference_type = 'product' AND reference_id = product_id",
		"reason = 'order_created' AND reference_type = 'order' AND operator_type = 'user' AND delta < 0",
		"reason = 'refund_restored' AND reference_type = 'refund' AND operator_type = 'admin' AND delta > 0",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing stock log audit contract %q", want)
		}
	}
}

func TestProductStockLogSchemaEnforcesOrderAndRefundReferences(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"order_reference_id BIGINT GENERATED ALWAYS AS",
		"CASE WHEN reference_type = 'order' THEN reference_id ELSE NULL END",
		"refund_reference_id BIGINT GENERATED ALWAYS AS",
		"CASE WHEN reference_type = 'refund' THEN reference_id ELSE NULL END",
		"mall_product_stock_logs_order_item_fkey",
		"NOT (condeferrable AND condeferred)",
		"FOREIGN KEY (order_reference_id, product_id) REFERENCES mall_order_items(order_id, product_id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED NOT VALID",
		"mall_product_stock_logs_refund_fkey",
		"FOREIGN KEY (refund_reference_id) REFERENCES mall_refund_requests(id) ON DELETE CASCADE NOT VALID",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing stock log reference enforcement %q", want)
		}
	}
}
