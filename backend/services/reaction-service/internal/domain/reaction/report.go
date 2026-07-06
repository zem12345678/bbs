package reaction

import (
	"context"
	"strings"
	"time"
)

type ReportStatus int32

const (
	ReportStatusPending  ReportStatus = 1
	ReportStatusResolved ReportStatus = 2
	ReportStatusRejected ReportStatus = 3
)

func (s ReportStatus) Valid() bool {
	switch s {
	case ReportStatusPending, ReportStatusResolved, ReportStatusRejected:
		return true
	default:
		return false
	}
}

type Report struct {
	ID          int64
	Entity      EntityRef
	ReporterID  int64
	Reason      string
	Description string
	Status      ReportStatus
	HandledBy   int64
	HandledAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SubmitReportCmd struct {
	Entity      EntityRef
	ReporterID  int64
	Reason      string
	Description string
}

func NewReport(cmd SubmitReportCmd) (*Report, error) {
	if err := cmd.Entity.Validate(); err != nil {
		return nil, err
	}
	if cmd.ReporterID <= 0 {
		return nil, ErrInvalidUserID
	}
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		return nil, ErrInvalidReportReason
	}
	now := time.Now()
	return &Report{
		Entity:      cmd.Entity,
		ReporterID:  cmd.ReporterID,
		Reason:      reason,
		Description: strings.TrimSpace(cmd.Description),
		Status:      ReportStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

type ReportRepository interface {
	CreateReport(ctx context.Context, report *Report) (created bool, err error)
	ListReports(ctx context.Context, status ReportStatus, entityType EntityType, limit, offset int) ([]*Report, int64, error)
	AuditReport(ctx context.Context, id int64, status ReportStatus, handlerID int64) (*Report, error)
}
