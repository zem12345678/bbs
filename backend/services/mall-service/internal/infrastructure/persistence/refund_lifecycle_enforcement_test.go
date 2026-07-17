package persistence

import (
	"strings"
	"testing"
)

func TestRefundSchemaEnforcesLifecycleAuditFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_refund_requests_lifecycle_check",
		"status = UPPER(TRIM(status))",
		"status = 'REQUESTED' AND operator_id = '' AND reviewed_at IS NULL AND refunded_at IS NULL AND restore_stock = false",
		"status = 'PROCESSING' AND BTRIM(operator_id) <> '' AND reviewed_at IS NOT NULL AND refunded_at IS NULL",
		"status = 'APPROVED' AND BTRIM(operator_id) <> '' AND reviewed_at IS NOT NULL AND refunded_at IS NOT NULL",
		"status = 'REJECTED' AND BTRIM(operator_id) <> '' AND reviewed_at IS NOT NULL AND refunded_at IS NULL AND restore_stock = false",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing refund lifecycle constraint %q", want)
		}
	}
}
