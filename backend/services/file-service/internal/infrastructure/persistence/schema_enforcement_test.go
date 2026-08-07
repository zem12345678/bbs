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
		"file_folders_snapshot_check",
		"idx_file_folders_owner_parent_created",
		"files_object_key_unique",
		"files_snapshot_check",
		"idx_files_owner_created",
		"idx_files_owner_folder_created",
		"ADD COLUMN IF NOT EXISTS folder_id",
		"ADD COLUMN IF NOT EXISTS is_sensitive",
		"ADD COLUMN IF NOT EXISTS comment",
		"idx_files_owner_deleted",
		"idx_files_chart_created",
		"idx_files_chart_deleted",
		"attachments_object_key_unique",
		"attachments_snapshot_check",
		"idx_attachments_owner_created",
		"idx_attachments_owner_archived",
		"idx_attachments_chart_created",
		"idx_attachments_chart_archived",
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
