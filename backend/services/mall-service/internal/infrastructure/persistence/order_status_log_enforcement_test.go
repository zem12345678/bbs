package persistence

import (
	"strings"
	"testing"
)

func TestOrderStatusLogSchemaEnforcesNormalizedAuditFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_order_status_logs_normalized_check",
		"from_status = '' OR from_status IN ('PENDING_PAYMENT', 'PAYING', 'PAID', 'CANCELED', 'SHIPPED', 'COMPLETED', 'CLOSED', 'REFUNDED')",
		"to_status IN ('PENDING_PAYMENT', 'PAYING', 'PAID', 'CANCELED', 'SHIPPED', 'COMPLETED', 'CLOSED', 'REFUNDED')",
		"BTRIM(reason) <> ''",
		"operator_type IN ('user', 'admin')",
		"BTRIM(operator_id) <> ''",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order status log constraint %q", want)
		}
	}
}
