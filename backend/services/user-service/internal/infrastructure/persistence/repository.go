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

type userPO struct {
	ID             int64     `gorm:"primaryKey"`
	Username       string    `gorm:"uniqueIndex;size:32;not null"`
	Email          string    `gorm:"uniqueIndex;size:255;not null"`
	PasswordHash   string    `gorm:"type:text;not null"`
	Nickname       string    `gorm:"size:64;not null"`
	AvatarURL      string    `gorm:"type:text;not null;default:''"`
	Bio            string    `gorm:"type:text;not null;default:''"`
	Status         int32     `gorm:"not null;default:1;index"`
	FollowerCount  int64     `gorm:"not null;default:0"`
	FollowingCount int64     `gorm:"not null;default:0"`
	CreatedAt      time.Time `gorm:"index"`
	UpdatedAt      time.Time
	LastLoginAt    *time.Time `gorm:"index"`
}

func (userPO) TableName() string {
	return "users"
}

type followPO struct {
	FollowerID int64     `gorm:"primaryKey"`
	FolloweeID int64     `gorm:"primaryKey"`
	CreatedAt  time.Time `gorm:"index"`
}

func (followPO) TableName() string {
	return "user_follows"
}

type Repo struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repo)(nil)

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func toPO(u *domain.User) userPO {
	return userPO{
		ID:             u.ID,
		Username:       u.Username,
		Email:          u.Email,
		PasswordHash:   u.PasswordHash,
		Nickname:       u.Nickname,
		AvatarURL:      u.AvatarURL,
		Bio:            u.Bio,
		Status:         int32(u.Status),
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
		LastLoginAt:    u.LastLoginAt,
	}
}

func toEntity(p *userPO) *domain.User {
	return &domain.User{
		ID:             p.ID,
		Username:       p.Username,
		Email:          p.Email,
		PasswordHash:   p.PasswordHash,
		Nickname:       p.Nickname,
		AvatarURL:      p.AvatarURL,
		Bio:            p.Bio,
		Status:         domain.Status(p.Status),
		FollowerCount:  p.FollowerCount,
		FollowingCount: p.FollowingCount,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		LastLoginAt:    p.LastLoginAt,
	}
}

func toEntities(rows []userPO) []*domain.User {
	out := make([]*domain.User, 0, len(rows))
	for i := range rows {
		out = append(out, toEntity(&rows[i]))
	}
	return out
}

func (r *Repo) Create(ctx context.Context, u *domain.User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	po := toPO(u)
	if err := r.db.WithContext(ctx).Create(&po).Error; err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repo) UpdateProfile(ctx context.Context, u *domain.User) error {
	res := r.db.WithContext(ctx).Model(&userPO{}).Where("id = ?", u.ID).Updates(map[string]any{
		"nickname":   u.Nickname,
		"avatar_url": u.AvatarURL,
		"bio":        u.Bio,
		"updated_at": u.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) UpdatePassword(ctx context.Context, u *domain.User) error {
	res := r.db.WithContext(ctx).Model(&userPO{}).Where("id = ?", u.ID).Updates(map[string]any{
		"password_hash": u.PasswordHash,
		"updated_at":    u.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateStatus(ctx context.Context, u *domain.User) error {
	res := r.db.WithContext(ctx).Model(&userPO{}).Where("id = ?", u.ID).Updates(map[string]any{
		"status":     int32(u.Status),
		"updated_at": u.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateLastLogin(ctx context.Context, u *domain.User) error {
	res := r.db.WithContext(ctx).Model(&userPO{}).Where("id = ?", u.ID).Updates(map[string]any{
		"last_login_at": u.LastLoginAt,
		"updated_at":    u.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidID
	}
	var p userPO
	err := r.db.WithContext(ctx).First(&p, id).Error
	return foundUser(&p, err)
}

func (r *Repo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var p userPO
	err := r.db.WithContext(ctx).Where("username = ?", domain.NormalizeUsername(username)).First(&p).Error
	return foundUser(&p, err)
}

func (r *Repo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var p userPO
	err := r.db.WithContext(ctx).Where("email = ?", domain.NormalizeEmail(email)).First(&p).Error
	return foundUser(&p, err)
}

func (r *Repo) FindByAccount(ctx context.Context, account string) (*domain.User, error) {
	account = strings.TrimSpace(account)
	var p userPO
	q := r.db.WithContext(ctx)
	if strings.Contains(account, "@") {
		q = q.Where("email = ?", domain.NormalizeEmail(account))
	} else {
		q = q.Where("username = ?", domain.NormalizeUsername(account))
	}
	err := q.First(&p).Error
	return foundUser(&p, err)
}

func (r *Repo) Follow(ctx context.Context, followerID, followeeID int64) error {
	if followerID == followeeID {
		return domain.ErrCannotFollowSelf
	}
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&followPO{
			FollowerID: followerID,
			FolloweeID: followeeID,
			CreatedAt:  now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrAlreadyFollowing
		}
		if err := tx.Model(&userPO{}).Where("id = ?", followeeID).UpdateColumn("follower_count", gorm.Expr("follower_count + 1")).Error; err != nil {
			return err
		}
		if err := tx.Model(&userPO{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("following_count + 1")).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *Repo) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&followPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFollowing
		}
		if err := tx.Model(&userPO{}).Where("id = ?", followeeID).UpdateColumn("follower_count", gorm.Expr("GREATEST(follower_count - 1, 0)")).Error; err != nil {
			return err
		}
		if err := tx.Model(&userPO{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("GREATEST(following_count - 1, 0)")).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *Repo) IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&followPO{}).
		Where("follower_id = ? AND followee_id = ?", followerID, followeeID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repo) ListUsers(ctx context.Context, q domain.UserListQuery) ([]*domain.User, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	db := r.db.WithContext(ctx).Model(&userPO{})
	if q.Status > 0 {
		db = db.Where("status = ?", q.Status)
	}
	query := strings.ToLower(strings.TrimSpace(q.Query))
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(username) LIKE ? OR LOWER(email) LIKE ? OR LOWER(nickname) LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userPO
	err := db.Order("created_at DESC, id DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return toEntities(rows), total, nil
}

func (r *Repo) ListFollowers(ctx context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	filter := r.db.WithContext(ctx).Table("user_follows").Where("followee_id = ?", q.UserID)
	var total int64
	if err := filter.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userPO
	err := r.db.WithContext(ctx).Table("users").
		Joins("JOIN user_follows ON user_follows.follower_id = users.id").
		Where("user_follows.followee_id = ?", q.UserID).
		Order("user_follows.created_at DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return toEntities(rows), total, nil
}

func (r *Repo) ListFollowing(ctx context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	filter := r.db.WithContext(ctx).Table("user_follows").Where("follower_id = ?", q.UserID)
	var total int64
	if err := filter.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userPO
	err := r.db.WithContext(ctx).Table("users").
		Joins("JOIN user_follows ON user_follows.followee_id = users.id").
		Where("user_follows.follower_id = ?", q.UserID).
		Order("user_follows.created_at DESC").
		Limit(q.PageSize).
		Offset((q.Page - 1) * q.PageSize).
		Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return toEntities(rows), total, nil
}

func foundUser(p *userPO, err error) (*domain.User, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toEntity(p), nil
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uk_users_username", "users_username_key":
			return domain.ErrUsernameExists
		case "uk_users_email", "users_email_key":
			return domain.ErrEmailExists
		default:
			if strings.Contains(pgErr.Detail, "username") {
				return domain.ErrUsernameExists
			}
			if strings.Contains(pgErr.Detail, "email") {
				return domain.ErrEmailExists
			}
		}
	}
	return err
}

func normalizeList(page *int, pageSize *int) {
	if *page <= 0 {
		*page = 1
	}
	if *pageSize <= 0 {
		*pageSize = 20
	}
	if *pageSize > 100 {
		*pageSize = 100
	}
}
