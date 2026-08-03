package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type inviteCodePO struct {
	ID               int64      `gorm:"primaryKey"`
	Code             string     `gorm:"size:32;not null;uniqueIndex"`
	CreatedByAdminID int64      `gorm:"not null;index"`
	UsedByUserID     *int64     `gorm:"uniqueIndex"`
	ExpiresAt        *time.Time `gorm:"index"`
	UsedAt           *time.Time `gorm:"index"`
	RevokedAt        *time.Time `gorm:"index"`
	RevokedByAdminID *int64
	CreatedAt        time.Time `gorm:"index"`
}

func (inviteCodePO) TableName() string { return "user_invite_codes" }

var _ domain.InviteRepository = (*Repo)(nil)

func toInvitePO(code domain.InviteCode) inviteCodePO {
	return inviteCodePO{
		ID:               code.ID,
		Code:             domain.NormalizeInviteCode(code.Code),
		CreatedByAdminID: code.CreatedByAdminID,
		UsedByUserID:     code.UsedByUserID,
		ExpiresAt:        code.ExpiresAt,
		UsedAt:           code.UsedAt,
		RevokedAt:        code.RevokedAt,
		RevokedByAdminID: code.RevokedByAdminID,
		CreatedAt:        code.CreatedAt,
	}
}

func toInviteEntity(code inviteCodePO) domain.InviteCode {
	return domain.InviteCode{
		ID:               code.ID,
		Code:             code.Code,
		CreatedByAdminID: code.CreatedByAdminID,
		UsedByUserID:     code.UsedByUserID,
		ExpiresAt:        code.ExpiresAt,
		UsedAt:           code.UsedAt,
		RevokedAt:        code.RevokedAt,
		RevokedByAdminID: code.RevokedByAdminID,
		CreatedAt:        code.CreatedAt,
	}
}

func (r *Repo) CreateWithInvite(ctx context.Context, u *domain.User, code string, requireInvite bool) error {
	if err := u.Validate(); err != nil {
		return err
	}
	code = domain.NormalizeInviteCode(code)
	if code == "" && requireInvite {
		return domain.ErrInviteCodeRequired
	}
	if code == "" {
		return r.Create(ctx, u)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invite inviteCodePO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&invite).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrInviteCodeInvalid
		}
		if err != nil {
			return err
		}
		now := time.Now()
		switch {
		case invite.UsedAt != nil || invite.UsedByUserID != nil:
			return domain.ErrInviteCodeUsed
		case invite.RevokedAt != nil:
			return domain.ErrInviteCodeRevoked
		case invite.ExpiresAt != nil && !invite.ExpiresAt.After(now):
			return domain.ErrInviteCodeExpired
		}
		if err := tx.Create(toPO(u)).Error; err != nil {
			return mapWriteError(err)
		}
		res := tx.Model(&inviteCodePO{}).
			Where("id = ? AND used_at IS NULL AND revoked_at IS NULL", invite.ID).
			Updates(map[string]any{"used_by_user_id": u.ID, "used_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return domain.ErrInviteCodeUsed
		}
		return nil
	})
}

func (r *Repo) CreateInviteCodes(ctx context.Context, codes []domain.InviteCode) error {
	if len(codes) < 1 || len(codes) > 100 {
		return domain.ErrInviteCountInvalid
	}
	now := time.Now()
	rows := make([]inviteCodePO, 0, len(codes))
	for _, code := range codes {
		if code.CreatedAt.IsZero() {
			code.CreatedAt = now
		}
		if err := code.ValidateForCreate(now); err != nil {
			return err
		}
		rows = append(rows, toInvitePO(code))
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return mapInviteWriteError(err)
	}
	return nil
}

func (r *Repo) ListInviteCodes(ctx context.Context, q domain.InviteCodeListQuery) ([]domain.InviteCode, int64, error) {
	status := domain.NormalizeInviteStatus(q.Status)
	if !domain.ValidInviteStatus(status) {
		return nil, 0, domain.ErrInviteStatusInvalid
	}
	normalizeList(&q.Page, &q.PageSize)
	now := time.Now()
	query := r.db.WithContext(ctx).Model(&inviteCodePO{})
	switch status {
	case domain.InviteStatusUnused:
		query = query.Where("used_at IS NULL AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", now)
	case domain.InviteStatusUsed:
		query = query.Where("used_at IS NOT NULL")
	case domain.InviteStatusExpired:
		query = query.Where("used_at IS NULL AND revoked_at IS NULL AND expires_at IS NOT NULL AND expires_at <= ?", now)
	case domain.InviteStatusRevoked:
		query = query.Where("used_at IS NULL AND revoked_at IS NOT NULL")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []inviteCodePO
	if err := query.Order("created_at DESC, id DESC").Limit(q.PageSize).Offset((q.Page - 1) * q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]domain.InviteCode, 0, len(rows))
	for _, row := range rows {
		items = append(items, toInviteEntity(row))
	}
	return items, total, nil
}

func (r *Repo) RevokeInviteCode(ctx context.Context, id, actorID int64) error {
	if id <= 0 || actorID <= 0 {
		return domain.ErrInvalidID
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invite inviteCodePO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&invite).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrInviteCodeNotFound
		}
		if err != nil {
			return err
		}
		if invite.UsedAt != nil || invite.UsedByUserID != nil {
			return domain.ErrInviteCodeUsed
		}
		if invite.RevokedAt != nil {
			return domain.ErrInviteCodeRevoked
		}
		now := time.Now()
		return tx.Model(&inviteCodePO{}).Where("id = ?", id).Updates(map[string]any{
			"revoked_at":          now,
			"revoked_by_admin_id": actorID,
		}).Error
	})
}

func mapInviteWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "invite") || strings.Contains(pgErr.Detail, "code") {
			return domain.ErrInviteCodeExists
		}
	}
	return err
}
