package persistence

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "admin/internal/domain/admin"
)

func TestRepositoryListOperationLogsSearchesRequestParams(t *testing.T) {
	dsn := os.Getenv("BBS_ADMIN_TEST_DSN")
	if dsn == "" {
		t.Skip("set BBS_ADMIN_TEST_DSN to run postgres-backed repository tests")
	}

	ctx := context.Background()
	repo, cleanup := repositoryForProtectedRoleTest(t, ctx, dsn)
	defer cleanup()

	marker := fmt.Sprintf("payload_marker_%d", time.Now().UnixNano())
	err := repo.RecordOperationLog(ctx, domain.RecordOperationLogCommand{
		Title:         "系统角色",
		BusinessType:  "create",
		Method:        "/api/v1/admin/system/roles",
		RequestMethod: "POST",
		OperatorType:  "admin",
		OperatorName:  "admin",
		URL:           "/api/v1/admin/system/roles",
		IP:            "127.0.0.1",
		Params:        fmt.Sprintf(`{"key":"%s"}`, marker),
		Status:        1,
		Result:        "200",
	})
	if err != nil {
		t.Fatalf("RecordOperationLog() error = %v", err)
	}

	logs, err := repo.ListOperationLogs(ctx, -1, strings.ToUpper(marker), 20, 0)
	if err != nil {
		t.Fatalf("ListOperationLogs() error = %v", err)
	}
	if logs.Total != 1 {
		t.Fatalf("ListOperationLogs() total = %d, want 1", logs.Total)
	}
	if len(logs.Items) != 1 || !strings.Contains(logs.Items[0].Params, marker) {
		t.Fatalf("ListOperationLogs() items = %#v, want params containing %q", logs.Items, marker)
	}
}
