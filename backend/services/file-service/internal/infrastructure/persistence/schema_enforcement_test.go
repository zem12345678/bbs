package persistence

import (
	"strings"
	"testing"
)

func TestSchemaEnforcesAttachmentAndDownloadInvariants(t *testing.T) {
	schema := strings.Join(schemaStatements, "\n")
	for _, want := range []string{
		"file_user_capacity_overrides_identity_check",
		"file_user_capacity_overrides_capacity_check",
		"files_object_key_unique",
		"files_snapshot_check",
		"idx_files_owner_created",
		"attachments_object_key_unique",
		"attachments_snapshot_check",
		"PRIMARY KEY(attachment_id, user_id)",
		"attachment_downloads_source_event_unique",
		"attachment_downloads_lifecycle_check",
		"file_erased_users_identity_check",
		"file_erased_attachment_objects_attachment_unique",
		"idx_file_erased_attachment_objects_pending",
		"file_erased_file_objects_file_unique",
		"idx_file_erased_file_objects_pending",
		"status = 'PENDING' AND authorized_at IS NULL",
		"status = 'AUTHORIZED' AND authorized_at IS NOT NULL",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("schema missing invariant %q", want)
		}
	}
}
