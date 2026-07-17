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
