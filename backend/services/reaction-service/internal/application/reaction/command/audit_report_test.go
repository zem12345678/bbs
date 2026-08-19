package command

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "reaction-service/internal/domain/reaction"
)

func TestAuditReportTrimsAuditNote(t *testing.T) {
	reports := &fakeReportRepository{}
	service := NewService(nil, reports, nil, nil, nil, nil, nil, nil)

	report, err := service.AuditReport(context.Background(), 10, domain.ReportStatusResolved, 20, "  handled by policy  ", "hide")
	if err != nil {
		t.Fatalf("AuditReport() error = %v", err)
	}
	if !reports.auditCalled {
		t.Fatalf("repository AuditReport was not called")
	}
	if reports.auditNote != "handled by policy" {
		t.Fatalf("audit note = %q, want trimmed value", reports.auditNote)
	}
	if reports.targetAction != "hide" {
		t.Fatalf("target action = %q, want hide", reports.targetAction)
	}
	if report.AuditNote != "handled by policy" {
		t.Fatalf("report audit note = %q, want trimmed value", report.AuditNote)
	}
}

func TestAuditReportRejectsLongAuditNote(t *testing.T) {
	reports := &fakeReportRepository{}
	service := NewService(nil, reports, nil, nil, nil, nil, nil, nil)

	_, err := service.AuditReport(context.Background(), 10, domain.ReportStatusResolved, 20, strings.Repeat("字", domain.MaxReportAuditNoteRunes+1), "")
	if !errors.Is(err, domain.ErrInvalidReportNote) {
		t.Fatalf("AuditReport() error = %v, want %v", err, domain.ErrInvalidReportNote)
	}
	if reports.auditCalled {
		t.Fatalf("repository AuditReport should not be called for invalid audit note")
	}
}

func TestAuditReportRejectsInvalidTargetAction(t *testing.T) {
	reports := &fakeReportRepository{}
	service := NewService(nil, reports, nil, nil, nil, nil, nil, nil)

	_, err := service.AuditReport(context.Background(), 10, domain.ReportStatusResolved, 20, "", "delete")
	if !errors.Is(err, domain.ErrInvalidReportAction) {
		t.Fatalf("AuditReport() error = %v, want %v", err, domain.ErrInvalidReportAction)
	}
	if reports.auditCalled {
		t.Fatalf("repository AuditReport should not be called for invalid target action")
	}
}

type fakeReportRepository struct {
	auditCalled  bool
	auditNote    string
	targetAction string
}

func (f *fakeReportRepository) CreateReport(context.Context, *domain.Report) (bool, error) {
	return false, nil
}

func (f *fakeReportRepository) ListReports(context.Context, domain.ReportStatus, domain.EntityType, int, int) ([]*domain.Report, int64, error) {
	return nil, 0, nil
}

func (f *fakeReportRepository) GetReport(context.Context, int64) (*domain.Report, error) {
	return nil, nil
}

func (f *fakeReportRepository) AuditReport(_ context.Context, id int64, status domain.ReportStatus, handlerID int64, auditNote string, targetAction string) (*domain.Report, error) {
	f.auditCalled = true
	f.auditNote = auditNote
	f.targetAction = targetAction
	now := time.Now()
	return &domain.Report{
		ID:           id,
		Status:       status,
		HandledBy:    handlerID,
		AuditNote:    auditNote,
		TargetAction: targetAction,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}
