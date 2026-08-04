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
	ID           int64      `gorm:"primaryKey"`
	EntityType   string     `gorm:"size:32;not null"`
	EntityID     int64      `gorm:"not null;index:idx_reports_entity_status"`
	ReporterID   int64      `gorm:"not null;index:idx_reports_reporter_created"`
	Reason       string     `gorm:"size:64;not null"`
	Description  string     `gorm:"type:text;not null;default:''"`
	Status       int32      `gorm:"not null;default:1;index:idx_reports_status_created"`
	HandledBy    int64      `gorm:"not null;default:0"`
	HandledAt    *time.Time `gorm:"index"`
	AuditNote    string     `gorm:"type:text;not null;default:''"`
	TargetAction string     `gorm:"size:32;not null;default:''"`
	CreatedAt    time.Time  `gorm:"not null;default:now();index:idx_reports_status_created;index:idx_reports_reporter_created"`
	UpdatedAt    time.Time  `gorm:"not null;default:now()"`
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
	if err := r.db.WithContext(ctx).AutoMigrate(&reportPO{}); err != nil {
		return err
	}
	statements := []string{
		`ALTER TABLE user_reports DROP CONSTRAINT IF EXISTS user_reports_entity_type_entity_id_reporter_id_status_key`,
		`DROP INDEX IF EXISTS idx_reports_unique_open`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_user_reports_live_identity_status
  ON user_reports(entity_type, entity_id, reporter_id, status)
  WHERE reporter_id > 0`,
	}
	for _, statement := range statements {
		if err := r.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func toReportPO(report *domain.Report) reportPO {
	if report == nil {
		return reportPO{}
	}
	return reportPO{
		ID:           report.ID,
		EntityType:   string(report.Entity.Type),
		EntityID:     report.Entity.ID,
		ReporterID:   report.ReporterID,
		Reason:       report.Reason,
		Description:  report.Description,
		Status:       int32(report.Status),
		HandledBy:    report.HandledBy,
		HandledAt:    report.HandledAt,
		AuditNote:    report.AuditNote,
		TargetAction: report.TargetAction,
		CreatedAt:    report.CreatedAt,
		UpdatedAt:    report.UpdatedAt,
	}
}

func toReportEntity(po *reportPO) *domain.Report {
	if po == nil {
		return nil
	}
	return &domain.Report{
		ID:           po.ID,
		Entity:       domain.EntityRef{Type: domain.EntityType(po.EntityType), ID: po.EntityID},
		ReporterID:   po.ReporterID,
		Reason:       po.Reason,
		Description:  po.Description,
		Status:       domain.ReportStatus(po.Status),
		HandledBy:    po.HandledBy,
		HandledAt:    po.HandledAt,
		AuditNote:    po.AuditNote,
		TargetAction: po.TargetAction,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
}

func (r *PostgresReportRepository) CreateReport(ctx context.Context, report *domain.Report) (bool, error) {
	po := toReportPO(report)
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, po.ReporterID); err != nil {
			return err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&po)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing reportPO
			if err := tx.Where("entity_type = ? AND entity_id = ? AND reporter_id = ? AND status = ?", po.EntityType, po.EntityID, po.ReporterID, int32(domain.ReportStatusPending)).First(&existing).Error; err != nil {
				return err
			}
			*report = *toReportEntity(&existing)
			return nil
		}
		created = true
		*report = *toReportEntity(&po)
		return nil
	})
	return created, err
}

func (r *PostgresReportRepository) GetReport(ctx context.Context, id int64) (*domain.Report, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidReportID
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

func (r *PostgresReportRepository) AuditReport(ctx context.Context, id int64, status domain.ReportStatus, handlerID int64, auditNote string, targetAction string) (*domain.Report, error) {
	var po reportPO
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureReactionUserActive(tx, handlerID); err != nil {
			return err
		}
		now := time.Now()
		result := tx.Model(&reportPO{}).Where("id = ?", id).Updates(map[string]any{
			"status": int32(status), "handled_by": handlerID, "handled_at": &now,
			"audit_note": auditNote, "target_action": targetAction, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrReportNotFound
		}
		if err := tx.First(&po, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrReportNotFound
		} else {
			return err
		}
	})
	if err != nil {
		return nil, err
	}
	return toReportEntity(&po), nil
}
