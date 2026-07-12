package grpc

import (
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"
)

func TestMallOverviewToPBIncludesPendingOutboxTotal(t *testing.T) {
	lastErrorAt := time.UnixMilli(1700000000000)
	nextAttemptAt := time.UnixMilli(1700000600000)
	overview := domain.MallOverview{
		PendingOutboxTotal: 7,
		PendingRefundTotal: 3,
		OutboxStatusCounts: []domain.StatusCount{
			{Status: "failed", Count: 2},
			{Status: "dead_letter", Count: 1},
		},
		OutboxLastError:     "kafka timeout",
		OutboxLastErrorAt:   &lastErrorAt,
		OutboxNextAttemptAt: &nextAttemptAt,
	}

	got := mallOverviewToPB(overview)
	if got.GetPendingOutboxTotal() != overview.PendingOutboxTotal {
		t.Fatalf("PendingOutboxTotal = %d, want %d", got.GetPendingOutboxTotal(), overview.PendingOutboxTotal)
	}
	if len(got.GetOutboxStatusCounts()) != 2 {
		t.Fatalf("OutboxStatusCounts len = %d, want 2", len(got.GetOutboxStatusCounts()))
	}
	if got.GetOutboxLastError() != overview.OutboxLastError {
		t.Fatalf("OutboxLastError = %q, want %q", got.GetOutboxLastError(), overview.OutboxLastError)
	}
	if got.GetOutboxLastErrorAt() != lastErrorAt.UnixMilli() {
		t.Fatalf("OutboxLastErrorAt = %d, want %d", got.GetOutboxLastErrorAt(), lastErrorAt.UnixMilli())
	}
	if got.GetOutboxNextAttemptAt() != nextAttemptAt.UnixMilli() {
		t.Fatalf("OutboxNextAttemptAt = %d, want %d", got.GetOutboxNextAttemptAt(), nextAttemptAt.UnixMilli())
	}
}
