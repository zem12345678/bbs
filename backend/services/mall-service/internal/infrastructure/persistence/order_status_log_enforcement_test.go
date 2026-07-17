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

func TestOrderStatusLogSchemaEnforcesTransitionContract(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_order_status_logs_transition_check",
		"reason = LOWER(TRIM(reason))",
		"from_status = '' AND to_status = 'PENDING_PAYMENT' AND reason = 'created' AND operator_type = 'user'",
		"from_status IN ('PENDING_PAYMENT', 'PAYING') AND to_status = 'CLOSED' AND reason = 'expired' AND operator_type = 'admin'",
		"from_status IN ('PAID', 'SHIPPED', 'COMPLETED') AND to_status = 'REFUNDED' AND reason = 'refunded' AND operator_type = 'admin'",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order status log transition contract %q", want)
		}
	}
}
