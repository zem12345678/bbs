package persistence

import (
	"testing"
	"time"

	domain "user-service/internal/domain/user"
)

func TestUserChartWindowUsesUTCAndKeepsExplicitEpochOffset(t *testing.T) {
	now := time.Date(2026, time.August, 7, 12, 34, 56, 0, time.FixedZone("CST", 8*60*60))
	first, last, step, unit := userChartWindow(domain.UserChartQuery{
		Span:  domain.UserChartSpanHour,
		Limit: 3,
	}, now)
	if unit != domain.UserChartSpanHour || step != time.Hour {
		t.Fatalf("step = %v, unit = %q", step, unit)
	}
	wantLast := time.Date(2026, time.August, 7, 4, 0, 0, 0, time.UTC)
	if !last.Equal(wantLast) || !first.Equal(wantLast.Add(-2*time.Hour)) {
		t.Fatalf("window = [%v, %v], want [%v, %v]", first, last, wantLast.Add(-2*time.Hour), wantLast)
	}

	zero := int64(0)
	first, last, step, unit = userChartWindow(domain.UserChartQuery{
		Span: domain.UserChartSpanDay, Limit: 2, Offset: &zero,
	}, now)
	if unit != domain.UserChartSpanDay || step != 24*time.Hour {
		t.Fatalf("step = %v, unit = %q", step, unit)
	}
	if !last.Equal(time.UnixMilli(0).UTC()) || !first.Equal(time.UnixMilli(0).UTC().Add(-24*time.Hour)) {
		t.Fatalf("epoch window = [%v, %v]", first, last)
	}

	aligned := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	offset := aligned.UnixMilli()
	_, last, _, _ = userChartWindow(domain.UserChartQuery{
		Span: domain.UserChartSpanHour, Limit: 1, Offset: &offset,
	}, now)
	if !last.Equal(aligned) {
		t.Fatalf("aligned offset bucket = %v, want %v", last, aligned)
	}
	offset++
	_, last, _, _ = userChartWindow(domain.UserChartQuery{
		Span: domain.UserChartSpanHour, Limit: 1, Offset: &offset,
	}, now)
	if want := aligned.Add(time.Hour); !last.Equal(want) {
		t.Fatalf("unaligned offset bucket = %v, want %v", last, want)
	}

	maximum := domain.MaxUserChartOffsetMillis
	_, last, _, _ = userChartWindow(domain.UserChartQuery{
		Span: domain.UserChartSpanDay, Limit: domain.MaxUserChartLimit, Offset: &maximum,
	}, now)
	if last.Before(time.UnixMilli(maximum).UTC()) {
		t.Fatalf("maximum offset bucket = %v, before offset", last)
	}
}
