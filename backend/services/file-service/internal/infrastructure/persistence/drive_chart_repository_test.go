package persistence

import (
	"testing"
	"time"

	domain "file-service/internal/domain/file"
)

func TestDriveChartWindowUsesUTCAndKeepsExplicitEpochOffset(t *testing.T) {
	zero := int64(0)
	first, last, step, unit := driveChartWindow(domain.DriveChartQuery{
		Span: domain.DriveChartSpanHour, Limit: 3, Offset: &zero,
	}, time.Date(2026, time.August, 7, 12, 34, 0, 0, time.FixedZone("other", 8*60*60)))
	if unit != domain.DriveChartSpanHour || step != time.Hour {
		t.Fatalf("hour window step=%s unit=%q", step, unit)
	}
	if want := time.UnixMilli(0).UTC(); !last.Equal(want) || !first.Equal(want.Add(-2*time.Hour)) {
		t.Fatalf("epoch window first=%s last=%s", first, last)
	}

	offset := time.Date(2026, time.August, 7, 12, 0, 1, 0, time.UTC).UnixMilli()
	_, last, _, _ = driveChartWindow(domain.DriveChartQuery{Span: "hour", Limit: 1, Offset: &offset}, time.Time{})
	if want := time.Date(2026, time.August, 7, 13, 0, 0, 0, time.UTC); !last.Equal(want) {
		t.Fatalf("non-aligned last=%s, want %s", last, want)
	}
	offset = time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC).UnixMilli()
	_, last, _, _ = driveChartWindow(domain.DriveChartQuery{Span: "hour", Limit: 1, Offset: &offset}, time.Time{})
	if want := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC); !last.Equal(want) {
		t.Fatalf("aligned last=%s, want %s", last, want)
	}
}
