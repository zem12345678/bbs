package persistence

import (
	"strings"
	"testing"
)

func TestOutboxSchemaEnforcesLifecycleFields(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"mall_outbox_events_lifecycle_check",
		"attempts >= 0",
		"status = 'pending' AND lease_owner = '' AND lease_expires_at IS NULL",
		"status = 'publishing' AND BTRIM(lease_owner) <> '' AND lease_expires_at IS NOT NULL",
		"status = 'failed' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error <> '' AND next_attempt_at IS NOT NULL",
		"status = 'dead_letter' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error <> '' AND next_attempt_at IS NULL",
		"status = 'published' AND lease_owner = '' AND lease_expires_at IS NULL AND last_error = '' AND published_at IS NOT NULL",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("schemaStatements missing outbox lifecycle constraint %q", want)
		}
	}
}
