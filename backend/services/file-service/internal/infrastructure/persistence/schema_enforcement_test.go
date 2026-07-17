package persistence

import (
	"strings"
	"testing"
)

func TestSchemaEnforcesAttachmentAndDownloadInvariants(t *testing.T) {
	schema := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"attachments_object_key_unique",
		"attachments_snapshot_check",
		"PRIMARY KEY(attachment_id, user_id)",
		"attachment_downloads_source_event_unique",
		"attachment_downloads_lifecycle_check",
		"status = 'PENDING' AND authorized_at IS NULL",
		"status = 'AUTHORIZED' AND authorized_at IS NOT NULL",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("schema missing invariant %q", want)
		}
	}
}
