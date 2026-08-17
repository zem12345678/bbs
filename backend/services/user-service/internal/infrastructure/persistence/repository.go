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
	ID                     int64      `gorm:"primaryKey"`
	Username               string     `gorm:"uniqueIndex;size:32;not null"`
	Email                  string     `gorm:"uniqueIndex;size:255;not null"`
	PasswordHash           string     `gorm:"type:text;not null"`
	CredentialVersion      string     `gorm:"type:text;not null;default:'0'"`
	Nickname               string     `gorm:"size:64;not null"`
	AvatarURL              string     `gorm:"type:text;not null;default:''"`
	BackgroundURL          string     `gorm:"type:text;not null;default:''"`
	ProfileTheme           string     `gorm:"type:text;not null;default:'default'"`
	Bio                    string     `gorm:"type:text;not null;default:''"`
	Status                 int32      `gorm:"not null;default:1;index"`
	AccountState           string     `gorm:"size:24;not null;default:'active';index"`
	AccountStateVersion    int64      `gorm:"not null;default:1"`
	ProtectedAccount       bool       `gorm:"not null;default:false"`
	FollowApprovalRequired bool       `gorm:"not null;default:false"`
	DeletionRequestedAt    *time.Time `gorm:"index"`
	DeletedAt              *time.Time `gorm:"index"`
	FollowerCount          int64      `gorm:"not null;default:0"`
	FollowingCount         int64      `gorm:"not null;default:0"`
	CreatedAt              time.Time  `gorm:"index"`
	UpdatedAt              time.Time
	LastLoginAt            *time.Time `gorm:"index"`
	EmailVerifiedAt        *time.Time `gorm:"index"`
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

type followLifecyclePO struct {
	ID           int64      `gorm:"primaryKey"`
	FollowerID   int64      `gorm:"not null;index"`
	FolloweeID   int64      `gorm:"not null;index"`
	FollowedAt   time.Time  `gorm:"not null;index"`
	UnfollowedAt *time.Time `gorm:"index"`
}

func (followLifecyclePO) TableName() string {
	return "user_follow_lifecycles"
}

type blockPO struct {
	ActorID   int64     `gorm:"primaryKey"`
	TargetID  int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"index"`
}

func (blockPO) TableName() string {
	return "user_blocks"
}

type mutePO struct {
	ActorID   int64     `gorm:"primaryKey"`
	TargetID  int64     `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"index"`
}

func (mutePO) TableName() string {
	return "user_mutes"
}

type oauthAccountPO struct {
	ID             int64     `gorm:"primaryKey;autoIncrement"`
	Provider       string    `gorm:"size:32;not null;uniqueIndex:uk_user_oauth_provider_user"`
	ProviderUserID string    `gorm:"size:128;not null;uniqueIndex:uk_user_oauth_provider_user"`
	UserID         int64     `gorm:"not null;index"`
	Username       string    `gorm:"size:128;not null;default:''"`
	Email          string    `gorm:"size:255;not null;default:''"`
	Nickname       string    `gorm:"size:128;not null;default:''"`
	AvatarURL      string    `gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time `gorm:"index"`
	UpdatedAt      time.Time
	LastLoginAt    *time.Time `gorm:"index"`
}

func (oauthAccountPO) TableName() string {
	return "user_oauth_accounts"
}

type passwordResetTokenPO struct {
	TokenHash string     `gorm:"primaryKey;type:text"`
	UserID    int64      `gorm:"not null;index"`
	Email     string     `gorm:"size:255;not null"`
	ExpiresAt time.Time  `gorm:"not null;index"`
	UsedAt    *time.Time `gorm:"index"`
	CreatedAt time.Time  `gorm:"index"`
}

func (passwordResetTokenPO) TableName() string {
	return "user_password_reset_tokens"
}

type emailVerificationTokenPO struct {
	TokenHash string     `gorm:"primaryKey;type:text"`
	UserID    int64      `gorm:"not null;index"`
	Email     string     `gorm:"size:255;not null"`
	ExpiresAt time.Time  `gorm:"not null;index"`
	UsedAt    *time.Time `gorm:"index"`
	CreatedAt time.Time  `gorm:"index"`
}

func (emailVerificationTokenPO) TableName() string {
	return "user_email_verification_tokens"
}

type Repo struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repo)(nil)
var _ domain.SafetyRepository = (*Repo)(nil)
var _ domain.UserChartRepository = (*Repo)(nil)

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func toPO(u *domain.User) userPO {
	return userPO{
		ID:                     u.ID,
		Username:               u.Username,
		Email:                  u.Email,
		PasswordHash:           u.PasswordHash,
		CredentialVersion:      domain.NormalizeCredentialVersion(u.CredentialVersion),
		Nickname:               u.Nickname,
		AvatarURL:              u.AvatarURL,
		BackgroundURL:          u.BackgroundURL,
		ProfileTheme:           domain.NormalizeProfileTheme(u.ProfileTheme),
		Bio:                    u.Bio,
		Status:                 int32(u.Status),
		AccountState:           string(domain.NormalizeAccountState(u.AccountState)),
		AccountStateVersion:    u.AccountStateVersion,
		ProtectedAccount:       u.ProtectedAccount,
		FollowApprovalRequired: u.FollowApprovalRequired,
		DeletionRequestedAt:    u.DeletionRequestedAt,
		DeletedAt:              u.DeletedAt,
		FollowerCount:          u.FollowerCount,
		FollowingCount:         u.FollowingCount,
		CreatedAt:              u.CreatedAt,
		UpdatedAt:              u.UpdatedAt,
		LastLoginAt:            u.LastLoginAt,
		EmailVerifiedAt:        u.EmailVerifiedAt,
	}
}

func toEntity(p *userPO) *domain.User {
	return &domain.User{
		ID:                     p.ID,
		Username:               p.Username,
		Email:                  p.Email,
		PasswordHash:           p.PasswordHash,
		CredentialVersion:      domain.NormalizeCredentialVersion(p.CredentialVersion),
		Nickname:               p.Nickname,
		AvatarURL:              p.AvatarURL,
		BackgroundURL:          p.BackgroundURL,
		ProfileTheme:           domain.NormalizeProfileTheme(p.ProfileTheme),
		Bio:                    p.Bio,
		Status:                 domain.Status(p.Status),
		AccountState:           domain.NormalizeAccountState(domain.AccountState(p.AccountState)),
		AccountStateVersion:    p.AccountStateVersion,
		ProtectedAccount:       p.ProtectedAccount,
		FollowApprovalRequired: p.FollowApprovalRequired,
		DeletionRequestedAt:    p.DeletionRequestedAt,
		DeletedAt:              p.DeletedAt,
		FollowerCount:          p.FollowerCount,
		FollowingCount:         p.FollowingCount,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
		LastLoginAt:            p.LastLoginAt,
		EmailVerifiedAt:        p.EmailVerifiedAt,
	}
}

func toEntities(rows []userPO) []*domain.User {
	out := make([]*domain.User, 0, len(rows))
	for i := range rows {
		out = append(out, toEntity(&rows[i]))
	}
	return out
}

func toOAuthPO(account domain.OAuthAccount) oauthAccountPO {
	now := time.Now()
	if account.CreatedAt.IsZero() {
		account.CreatedAt = now
	}
	if account.UpdatedAt.IsZero() {
		account.UpdatedAt = now
	}
	return oauthAccountPO{
		Provider:       domain.NormalizeProvider(account.Provider),
		ProviderUserID: strings.TrimSpace(account.ProviderUserID),
		UserID:         account.UserID,
		Username:       strings.TrimSpace(account.Username),
		Email:          domain.NormalizeEmail(account.Email),
		Nickname:       strings.TrimSpace(account.Nickname),
		AvatarURL:      strings.TrimSpace(account.AvatarURL),
		CreatedAt:      account.CreatedAt,
		UpdatedAt:      account.UpdatedAt,
		LastLoginAt:    account.LastLoginAt,
	}
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
		"nickname":       u.Nickname,
		"avatar_url":     u.AvatarURL,
		"background_url": u.BackgroundURL,
		"profile_theme":  domain.NormalizeProfileTheme(u.ProfileTheme),
		"bio":            u.Bio,
		"updated_at":     u.UpdatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) UpdatePasswordAndCredentialVersion(ctx context.Context, u *domain.User, expectedPasswordHash string) error {
	if u == nil || u.ID <= 0 {
		return domain.ErrInvalidID
	}
	if strings.TrimSpace(expectedPasswordHash) == "" {
		return domain.ErrInvalidPassword
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return domain.ErrPasswordRequired
	}
	if !domain.ValidCredentialVersion(u.CredentialVersion) || domain.NormalizeCredentialVersion(u.CredentialVersion) == domain.InitialCredentialVersion {
		return domain.ErrInvalidCredentialVersion
	}
	res := r.db.WithContext(ctx).Model(&userPO{}).
		Where("id = ? AND password_hash = ?", u.ID, expectedPasswordHash).
		Updates(map[string]any{
			"password_hash":      u.PasswordHash,
			"credential_version": domain.NormalizeCredentialVersion(u.CredentialVersion),
			"updated_at":         u.UpdatedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// The caller had already loaded this user. A zero-row conditional update
		// therefore means another password rotation won the race.
		return domain.ErrInvalidPassword
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

func (r *Repo) UpdateOAuthLogin(ctx context.Context, u *domain.User, account domain.OAuthAccount) error {
	if u == nil || u.ID <= 0 {
		return domain.ErrInvalidID
	}
	account.UserID = u.ID
	row := toOAuthPO(account)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&userPO{}).Where("id = ?", u.ID).Updates(map[string]any{
			"last_login_at": u.LastLoginAt,
			"updated_at":    u.UpdatedAt,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "provider"}, {Name: "provider_user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"user_id":       row.UserID,
				"username":      row.Username,
				"email":         row.Email,
				"nickname":      row.Nickname,
				"avatar_url":    row.AvatarURL,
				"updated_at":    row.UpdatedAt,
				"last_login_at": row.LastLoginAt,
			}),
		}).Create(&row).Error
		return mapOAuthWriteError(err)
	})
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

func (r *Repo) FindByOAuth(ctx context.Context, provider string, providerUserID string) (*domain.User, error) {
	provider = domain.NormalizeProvider(provider)
	providerUserID = strings.TrimSpace(providerUserID)
	if !domain.ValidOAuthProvider(provider) || providerUserID == "" {
		return nil, domain.ErrInvalidOAuth
	}
	var account oauthAccountPO
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
		First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, account.UserID)
}

func (r *Repo) CreateWithOAuth(ctx context.Context, u *domain.User, account domain.OAuthAccount) error {
	if err := u.Validate(); err != nil {
		return err
	}
	account.UserID = u.ID
	row := toOAuthPO(account)
	if row.Provider == "" || row.ProviderUserID == "" {
		return domain.ErrInvalidOAuth
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toPO(u)).Error; err != nil {
			return mapWriteError(err)
		}
		if err := tx.Create(&row).Error; err != nil {
			return mapOAuthWriteError(err)
		}
		return nil
	})
}

func (r *Repo) EnsureWebmaster(ctx context.Context, u *domain.User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing userPO
		err := tx.Where("username = ?", u.Username).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			po := toPO(u)
			if err := tx.Create(&po).Error; err != nil {
				return mapWriteError(err)
			}
			return nil
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			"email":                 u.Email,
			"password_hash":         u.PasswordHash,
			"nickname":              u.Nickname,
			"status":                int32(domain.StatusActive),
			"account_state":         string(domain.AccountStateActive),
			"protected_account":     true,
			"deletion_requested_at": nil,
			"deleted_at":            nil,
			"last_login_at":         u.LastLoginAt,
			"updated_at":            u.UpdatedAt,
		}
		if err := tx.Model(&existing).Updates(updates).Error; err != nil {
			return mapWriteError(err)
		}
		var refreshed userPO
		if err := tx.Where("id = ?", existing.ID).First(&refreshed).Error; err != nil {
			return err
		}
		*u = *toEntity(&refreshed)
		return nil
	})
}

func (r *Repo) CreatePasswordResetToken(ctx context.Context, token domain.PasswordResetToken) error {
	token.TokenHash = strings.TrimSpace(token.TokenHash)
	token.Email = domain.NormalizeEmail(token.Email)
	if token.TokenHash == "" || token.UserID <= 0 || token.Email == "" || token.ExpiresAt.IsZero() {
		return domain.ErrResetTokenInvalid
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}
	row := passwordResetTokenPO{
		TokenHash: token.TokenHash,
		UserID:    token.UserID,
		Email:     token.Email,
		ExpiresAt: token.ExpiresAt,
		CreatedAt: token.CreatedAt,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&passwordResetTokenPO{}).
			Where("user_id = ? AND used_at IS NULL", token.UserID).
			Update("used_at", token.CreatedAt).Error; err != nil {
			return err
		}
		return tx.Create(&row).Error
	})
}

func (r *Repo) ResetPasswordWithToken(ctx context.Context, tokenHash string, passwordHash string, credentialVersion string, now time.Time) (*domain.User, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return nil, domain.ErrResetTokenInvalid
	}
	if passwordHash == "" {
		return nil, domain.ErrPasswordRequired
	}
	credentialVersion = strings.TrimSpace(credentialVersion)
	if credentialVersion == "" || credentialVersion == domain.InitialCredentialVersion {
		return nil, domain.ErrInvalidCredentialVersion
	}
	if now.IsZero() {
		now = time.Now()
	}
	var out *domain.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token passwordResetTokenPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(&token).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrResetTokenInvalid
		}
		if err != nil {
			return err
		}
		if token.UsedAt != nil {
			return domain.ErrResetTokenInvalid
		}
		if !token.ExpiresAt.After(now) {
			return domain.ErrResetTokenExpired
		}
		res := tx.Model(&userPO{}).Where("id = ?", token.UserID).Updates(map[string]any{
			"password_hash":      passwordHash,
			"credential_version": credentialVersion,
			"updated_at":         now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		if err := tx.Model(&passwordResetTokenPO{}).Where("token_hash = ?", tokenHash).Update("used_at", now).Error; err != nil {
			return err
		}
		var refreshed userPO
		if err := tx.Where("id = ?", token.UserID).First(&refreshed).Error; err != nil {
			return err
		}
		out = toEntity(&refreshed)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repo) GetCredentialVersion(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", domain.ErrInvalidID
	}
	var row userPO
	err := r.db.WithContext(ctx).
		Select("credential_version").
		Where("id = ?", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(row.CredentialVersion)
	if version == "" {
		return "", domain.ErrInvalidCredentialVersion
	}
	return version, nil
}

func (r *Repo) CreateEmailVerificationToken(ctx context.Context, token domain.EmailVerificationToken) error {
	token.TokenHash = strings.TrimSpace(token.TokenHash)
	token.Email = domain.NormalizeEmail(token.Email)
	if token.TokenHash == "" || token.UserID <= 0 || token.Email == "" || token.ExpiresAt.IsZero() {
		return domain.ErrEmailVerificationTokenInvalid
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}
	row := emailVerificationTokenPO{
		TokenHash: token.TokenHash,
		UserID:    token.UserID,
		Email:     token.Email,
		ExpiresAt: token.ExpiresAt,
		CreatedAt: token.CreatedAt,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&emailVerificationTokenPO{}).
			Where("user_id = ? AND used_at IS NULL", token.UserID).
			Update("used_at", token.CreatedAt).Error; err != nil {
			return err
		}
		return tx.Create(&row).Error
	})
}

func (r *Repo) VerifyEmailWithToken(ctx context.Context, tokenHash string, now time.Time) (*domain.User, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	if tokenHash == "" {
		return nil, domain.ErrEmailVerificationTokenInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	var out *domain.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token emailVerificationTokenPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(&token).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrEmailVerificationTokenInvalid
		}
		if err != nil {
			return err
		}
		if token.UsedAt != nil {
			return domain.ErrEmailVerificationTokenInvalid
		}
		if !token.ExpiresAt.After(now) {
			return domain.ErrEmailVerificationTokenExpired
		}
		var user userPO
		if err := tx.Where("id = ?", token.UserID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}
		if domain.NormalizeEmail(user.Email) != token.Email {
			return domain.ErrEmailVerificationTokenInvalid
		}
		if err := tx.Model(&userPO{}).Where("id = ?", token.UserID).Updates(map[string]any{
			"email_verified_at": now,
			"updated_at":        now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&emailVerificationTokenPO{}).Where("token_hash = ?", tokenHash).Update("used_at", now).Error; err != nil {
			return err
		}
		var refreshed userPO
		if err := tx.Where("id = ?", token.UserID).First(&refreshed).Error; err != nil {
			return err
		}
		out = toEntity(&refreshed)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repo) Follow(ctx context.Context, followerID, followeeID int64) error {
	if followerID == followeeID {
		return domain.ErrCannotFollowSelf
	}
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveUserPair(tx, followerID, followeeID); err != nil {
			return err
		}
		if err := ensureUserPairNotBlocked(tx, followerID, followeeID); err != nil {
			return err
		}
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
		if err := createFollowLifecycle(tx, followerID, followeeID, now); err != nil {
			return err
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
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveUserPair(tx, followerID, followeeID); err != nil {
			return err
		}
		res := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&followPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotFollowing
		}
		if err := closeFollowLifecycle(tx, followerID, followeeID, now); err != nil {
			return err
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

func (r *Repo) Block(ctx context.Context, actorID, targetID int64) error {
	if actorID == targetID {
		return domain.ErrCannotRelateSelf
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActiveUserPair(tx, actorID, targetID); err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&blockPO{
			ActorID: actorID, TargetID: targetID, CreatedAt: time.Now(),
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrAlreadyBlocking
		}
		// A block also suppresses the target's content. Keep this operation
		// idempotent so an existing explicit mute does not make blocking fail.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mutePO{
			ActorID: actorID, TargetID: targetID, CreatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := removeFollow(tx, actorID, targetID); err != nil {
			return err
		}
		if err := removeFollow(tx, targetID, actorID); err != nil {
			return err
		}
		// A blocked pair must not retain a pending request that can later be
		// accepted into a new follow relationship.
		return tx.Where(
			"(requester_id = ? AND target_id = ?) OR (requester_id = ? AND target_id = ?)",
			actorID, targetID, targetID, actorID,
		).Delete(&followRequestPO{}).Error
	})
}

// lockActiveUserPair serializes relationship creation and blocking for one
// unordered user pair. Locking by ascending ID avoids deadlocks when callers
// present the same pair in opposite directions.
func lockActiveUserPair(tx *gorm.DB, leftID, rightID int64) error {
	firstID, secondID := leftID, rightID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}
	for _, id := range []int64{firstID, secondID} {
		var row userPO
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := toEntity(&row).EnsureActive(); err != nil {
			return err
		}
	}
	return nil
}

func ensureUserPairNotBlocked(tx *gorm.DB, leftID, rightID int64) error {
	var count int64
	err := tx.Model(&blockPO{}).Where(
		"(actor_id = ? AND target_id = ?) OR (actor_id = ? AND target_id = ?)",
		leftID, rightID, rightID, leftID,
	).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrFollowBlocked
	}
	return nil
}

func (r *Repo) Unblock(ctx context.Context, actorID, targetID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("actor_id = ? AND target_id = ?", actorID, targetID).Delete(&blockPO{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrNotBlocking
		}
		// Block-created mutes have no separate source column; match the
		// reference behavior and clear the pair's mute when unblocking.
		return tx.Where("actor_id = ? AND target_id = ?", actorID, targetID).Delete(&mutePO{}).Error
	})
}

func (r *Repo) Mute(ctx context.Context, actorID, targetID int64) error {
	if actorID == targetID {
		return domain.ErrCannotRelateSelf
	}
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&mutePO{
		ActorID: actorID, TargetID: targetID, CreatedAt: time.Now(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrAlreadyMuted
	}
	return nil
}

func (r *Repo) Unmute(ctx context.Context, actorID, targetID int64) error {
	res := r.db.WithContext(ctx).Where("actor_id = ? AND target_id = ?", actorID, targetID).Delete(&mutePO{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotMuted
	}
	return nil
}

func (r *Repo) GetSafetyRelation(ctx context.Context, actorID, targetID int64) (domain.SafetyRelation, error) {
	blocked, err := relationExists(ctx, r.db, &blockPO{}, actorID, targetID)
	if err != nil {
		return domain.SafetyRelation{}, err
	}
	blockedBy, err := relationExists(ctx, r.db, &blockPO{}, targetID, actorID)
	if err != nil {
		return domain.SafetyRelation{}, err
	}
	muted, err := relationExists(ctx, r.db, &mutePO{}, actorID, targetID)
	if err != nil {
		return domain.SafetyRelation{}, err
	}
	return domain.SafetyRelation{Blocked: blocked, BlockedBy: blockedBy, Muted: muted}, nil
}

func (r *Repo) ListBlockedUsers(ctx context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	return r.listSafetyUsers(ctx, q, "user_blocks")
}

func (r *Repo) ListMutedUsers(ctx context.Context, q domain.FollowListQuery) ([]*domain.User, int64, error) {
	return r.listSafetyUsers(ctx, q, "user_mutes")
}

func removeFollow(tx *gorm.DB, followerID, followeeID int64) error {
	res := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&followPO{})
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error
	}
	if err := closeFollowLifecycle(tx, followerID, followeeID, time.Now()); err != nil {
		return err
	}
	if err := tx.Model(&userPO{}).Where("id = ?", followeeID).UpdateColumn("follower_count", gorm.Expr("GREATEST(follower_count - 1, 0)")).Error; err != nil {
		return err
	}
	return tx.Model(&userPO{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("GREATEST(following_count - 1, 0)")).Error
}

func relationExists(ctx context.Context, db *gorm.DB, model any, actorID, targetID int64) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Model(model).Where("actor_id = ? AND target_id = ?", actorID, targetID).Count(&count).Error
	return count > 0, err
}

func (r *Repo) listSafetyUsers(ctx context.Context, q domain.FollowListQuery, table string) ([]*domain.User, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	filter := r.db.WithContext(ctx).Table(table).Where("actor_id = ?", q.UserID)
	var total int64
	if err := filter.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userPO
	query := r.db.WithContext(ctx).Table("users").
		Joins("JOIN "+table+" ON "+table+".target_id = users.id").
		Where(table+".actor_id = ?", q.UserID)
	if q.AscendingByID {
		query = query.Where(table+".target_id > ?", q.AfterID).
			Order(table + ".target_id ASC").
			Limit(q.PageSize)
	} else {
		query = query.Order(table + ".created_at DESC").
			Order(table + ".target_id DESC").
			Limit(q.PageSize).
			Offset((q.Page - 1) * q.PageSize)
	}
	err := query.Find(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return toEntities(rows), total, nil
}

func (r *Repo) ListUsers(ctx context.Context, q domain.UserListQuery) ([]*domain.User, int64, error) {
	normalizeList(&q.Page, &q.PageSize)
	db := r.db.WithContext(ctx).Model(&userPO{})
	if q.Status > 0 {
		db = db.Where("status = ?", q.Status)
	}
	if len(q.IDs) > 0 {
		db = db.Where("id IN ?", q.IDs)
	}
	query := strings.ToLower(strings.TrimSpace(q.Query))
	if query != "" {
		var candidates []userPO
		err := db.Order("created_at DESC, id DESC").Find(&candidates).Error
		if err != nil {
			return nil, 0, err
		}
		rows, total := searchUserRows(candidates, query, q.Page, q.PageSize)
		return toEntities(rows), total, nil
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

func mapOAuthWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrInvalidOAuth
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
