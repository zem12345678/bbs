package persistence

import (
	"strings"
	"testing"
)

func TestOrderSchemaEnforcesLifecycleFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"WHERE status = 'SHIPPED'",
		"AND shipped_at IS NULL",
		"mall_orders_lifecycle_check",
		"user_id > 0",
		"status = UPPER(TRIM(status))",
		"status IN ('PENDING_PAYMENT', 'PAYING', 'PAID', 'CANCELED', 'SHIPPED', 'COMPLETED', 'CLOSED', 'REFUNDED')",
		"status = 'PAID' AND paid_at IS NOT NULL AND shipped_at IS NULL AND completed_at IS NULL",
		"status = 'SHIPPED' AND paid_at IS NOT NULL AND shipped_at IS NOT NULL AND completed_at IS NULL",
		"status = 'COMPLETED' AND paid_at IS NOT NULL AND completed_at IS NOT NULL",
		"status = 'REFUNDED' AND paid_at IS NOT NULL",
		"shipped_at >= paid_at",
		"completed_at >= shipped_at",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing order lifecycle constraint %q", want)
		}
	}
}
