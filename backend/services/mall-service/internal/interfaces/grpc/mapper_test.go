package grpc

import (
	"testing"

	domain "mall-service/internal/domain/mall"
)

func TestMallOverviewToPBIncludesPendingOutboxTotal(t *testing.T) {
	overview := domain.MallOverview{
		PendingOutboxTotal: 7,
		PendingRefundTotal: 3,
	}

	got := mallOverviewToPB(overview)
	if got.GetPendingOutboxTotal() != overview.PendingOutboxTotal {
		t.Fatalf("PendingOutboxTotal = %d, want %d", got.GetPendingOutboxTotal(), overview.PendingOutboxTotal)
	}
}
