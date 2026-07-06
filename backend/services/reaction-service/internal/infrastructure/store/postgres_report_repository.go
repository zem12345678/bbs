package store

import (
	"context"
	"errors"
	"time"

	domain "reaction-service/internal/domain/reaction"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reportPO struct {
	ID          int64      `gorm:"primaryKey"`
	EntityType  string     `gorm:"size:32;not null;uniqueIndex:idx_reports_unique_open"`
	EntityID    int64      `gorm:"not null;uniqueIndex:idx_reports_unique_open;index:idx_reports_entity_status"`
	ReporterID  int64      `gorm:"not null;uniqueIndex:idx_reports_unique_open;index:idx_reports_reporter_created"`
	Reason      string     `gorm:"size:64;not null"`
	Description string     `gorm:"type:text;not null;default:''"`
	Status      int32      `gorm:"not null;default:1;uniqueIndex:idx_reports_unique_open;index:idx_reports_status_created"`
	HandledBy   int64      `gorm:"not null;default:0"`
	HandledAt   *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null;default:now();index:idx_reports_status_created;index:idx_reports_reporter_created"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()"`
}

func (reportPO) TableName() string {
	return "user_reports"
}

type PostgresReportRepository struct {
	db *gorm.DB
}

func NewPostgresReportRepository(db *gorm.DB) *PostgresReportRepository {
	return &PostgresReportRepository{db: db}
}

func (r *PostgresReportRepository) EnsureSchema(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&reportPO{})
}

func toReportPO(report *domain.Report) reportPO {
	if report == nil {
		return reportPO{}
	}
	return reportPO{
		ID:          report.ID,
		EntityType:  string(report.Entity.Type),
		EntityID:    report.Entity.ID,
		ReporterID:  report.ReporterID,
		Reason:      report.Reason,
		Description: report.Description,
		Status:      int32(report.Status),
		HandledBy:   report.HandledBy,
		HandledAt:   report.HandledAt,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
	}
}

func toReportEntity(po *reportPO) *domain.Report {
	if po == nil {
		return nil
	}
	return &domain.Report{
		ID:          po.ID,
		Entity:      domain.EntityRef{Type: domain.EntityType(po.EntityType), ID: po.EntityID},
		ReporterID:  po.ReporterID,
		Reason:      po.Reason,
		Description: po.Description,
		Status:      domain.ReportStatus(po.Status),
		HandledBy:   po.HandledBy,
		HandledAt:   po.HandledAt,
		CreatedAt:   po.CreatedAt,
		UpdatedAt:   po.UpdatedAt,
	}
}

func (r *PostgresReportRepository) CreateReport(ctx context.Context, report *domain.Report) (bool, error) {
	po := toReportPO(report)
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		var existing reportPO
		err := r.db.WithContext(ctx).
			Where("entity_type = ? AND entity_id = ? AND reporter_id = ? AND status = ?", po.EntityType, po.EntityID, po.ReporterID, int32(domain.ReportStatusPending)).
			First(&existing).Error
		if err != nil {
			return false, err
		}
		*report = *toReportEntity(&existing)
		return false, nil
	}
	*report = *toReportEntity(&po)
	return true, nil
}

func (r *PostgresReportRepository) ListReports(ctx context.Context, status domain.ReportStatus, entityType domain.EntityType, limit, offset int) ([]*domain.Report, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	query := r.db.WithContext(ctx).Model(&reportPO{})
	if status != 0 {
		query = query.Where("status = ?", int32(status))
	}
	if entityType != "" {
		query = query.Where("entity_type = ?", string(entityType))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []reportPO
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.Report, 0, len(rows))
	for i := range rows {
		out = append(out, toReportEntity(&rows[i]))
	}
	return out, total, nil
}

func (r *PostgresReportRepository) AuditReport(ctx context.Context, id int64, status domain.ReportStatus, handlerID int64) (*domain.Report, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&reportPO{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     int32(status),
			"handled_by": handlerID,
			"handled_at": &now,
			"updated_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, domain.ErrReportNotFound
	}
	var po reportPO
	err := r.db.WithContext(ctx).First(&po, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return toReportEntity(&po), nil
}
