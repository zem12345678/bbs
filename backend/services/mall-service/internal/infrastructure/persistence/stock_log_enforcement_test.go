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
