package persistence

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

const roleRelationWriteBatchSize = 100

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) EnsureSchema(ctx context.Context) error {
	if err := r.db.WithContext(ctx).AutoMigrate(po.Models()...); err != nil {
		return err
	}
	if err := r.syncSystemSequences(ctx); err != nil {
		return err
	}
	return r.ensureSystemIntegrityIndexes(ctx)
}

func (r *Repository) syncSystemSequences(ctx context.Context) error {
	tables := []string{
		"sys_api",
		"casbin_rule",
		"sys_dept",
		"sys_dict_data",
		"sys_dict_type",
		"sys_login_log",
		"sys_menu",
		"sys_menu_param",
		"sys_menu_button",
		"admin_link",
		"admin_task",
		"admin_badge",
		"admin_level",
		"admin_forbidden_word",
		"admin_site_setting",
		"admin_email_log",
		"sys_opera_log",
		"sys_post",
		"sys_role",
		"sys_user",
		"sys_user_token",
	}
	for _, table := range tables {
		if err := syncPostgresSequence(ctx, r.db, table, "id"); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ensureSystemIntegrityIndexes(ctx context.Context) error {
	statements := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_user_username_lower_unique ON sys_user (LOWER(user_name)) WHERE COALESCE(user_name, '') <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_user_email_lower_unique ON sys_user (LOWER(email)) WHERE COALESCE(email, '') <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_role_key_lower_unique ON sys_role (LOWER("key")) WHERE COALESCE("key", '') <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_menu_name_lower_unique ON sys_menu (LOWER(name)) WHERE COALESCE(name, '') <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sys_dept_parent_name_lower_unique ON sys_dept (parent_id, LOWER(name)) WHERE COALESCE(name, '') <> ''`,
	}
	for _, statement := range statements {
		if err := r.db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) SeedDefaults(ctx context.Context, bootstrapAdmins []string, defaultPassword string) error {
	passwordHash, err := hashPassword(defaultPassword)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := seedDefaultOrg(ctx, tx); err != nil {
			return err
		}
		roles := []po.Role{
			{Name: "普通用户", Key: "user", Status: "1", Sort: 10, Remark: "Default backend user role"},
			{Name: "内容审核员", Key: "moderator", Status: "1", Sort: 20, Remark: "Can list and audit reports"},
			{Name: "管理员", Key: "admin", Status: "1", Sort: 30, Admin: true, Remark: "Can manage users and moderation"},
			{Name: "超级管理员", Key: "superadmin", Status: "1", Sort: 40, Admin: true, Remark: "Full backend access"},
		}
		for _, role := range roles {
			if err := upsertRole(ctx, tx, role); err != nil {
				return err
			}
		}
		if err := seedDefaultSystemManagement(ctx, tx); err != nil {
			return err
		}
		for _, rule := range defaultCasbinRules() {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rule).Error; err != nil {
				return err
			}
		}
		for _, username := range normalizeList(bootstrapAdmins) {
			if err := ensureAdminUser(ctx, tx, username, passwordHash); err != nil {
				return err
			}
		}
		if err := seedDefaultOperations(ctx, tx); err != nil {
			return err
		}
		return nil
	})
}

func seedDefaultOrg(ctx context.Context, tx *gorm.DB) error {
	now := time.Now()
	dept := po.Dept{
		ID:     1,
		Name:   "默认部门",
		Path:   "/1",
		Sort:   1,
		Status: 1,
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&dept).Error; err != nil {
		return err
	}
	post := po.Post{
		ID:     1,
		Name:   "默认岗位",
		Code:   "default",
		Sort:   1,
		Status: 1,
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&post).Error; err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Model(&po.Dept{}).Where("id = ?", 1).Updates(map[string]any{
		"name":        dept.Name,
		"path":        dept.Path,
		"sort":        dept.Sort,
		"status":      dept.Status,
		"update_time": now,
	}).Error; err != nil {
		return err
	}
	if err := syncPostgresSequence(ctx, tx, "sys_dept", "id"); err != nil {
		return err
	}
	return syncPostgresSequence(ctx, tx, "sys_post", "id")
}

func syncPostgresSequence(ctx context.Context, tx *gorm.DB, table string, column string) error {
	stmt := fmt.Sprintf(
		`WITH seq AS (SELECT pg_get_serial_sequence('%s', '%s') AS name)
SELECT setval(seq.name, GREATEST(COALESCE((SELECT MAX(%s) FROM %s), 0) + 1, 1), false)
FROM seq
WHERE seq.name IS NOT NULL`,
		table,
		column,
		column,
		table,
	)
	return tx.WithContext(ctx).Exec(stmt).Error
}

func (r *Repository) FindAdminUserByAccount(ctx context.Context, account string) (domain.AdminUser, error) {
	account = normalize(account)
	if account == "" {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	var user po.User
	err := r.db.WithContext(ctx).
		Where("LOWER(user_name) = ? OR LOWER(email) = ? OR phone = ?", account, account, account).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return domain.AdminUser{}, err
	}
	return r.toDomainAdminUserWithProfile(ctx, r.db, user)
}

func (r *Repository) FindAdminUserByID(ctx context.Context, id int64) (domain.AdminUser, error) {
	if id <= 0 {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	var user po.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return domain.AdminUser{}, err
	}
	return r.toDomainAdminUserWithProfile(ctx, r.db, user)
}

func (r *Repository) UpdateAdminProfile(ctx context.Context, command domain.UpdateAdminProfileCommand) (domain.AdminUser, error) {
	if command.UserID <= 0 || strings.TrimSpace(command.Nickname) == "" {
		return domain.AdminUser{}, domain.ErrInvalidAdminProfile
	}
	var updated domain.AdminUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user po.User
		if err := tx.WithContext(ctx).Where("id = ?", command.UserID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		email := normalize(command.Email)
		if email == "" {
			email = user.Email
		}
		phone := strings.TrimSpace(command.Phone)
		if phone == "" {
			phone = user.Phone
		}
		exists, err := adminUserExistsExcluding(ctx, tx, user.Username, email, phone, user.ID)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrAdminUserExists
		}
		now := time.Now()
		if err := tx.WithContext(ctx).Model(&user).Updates(map[string]any{
			"email":       email,
			"phone":       phone,
			"update_time": now,
		}).Error; err != nil {
			if isAdminUserIdentityUniqueViolation(err) {
				return domain.ErrAdminUserExists
			}
			return err
		}
		profile, err := adminProfileByUserID(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		avatarURL := strings.TrimSpace(command.AvatarURL)
		if avatarURL == "" {
			avatarURL = profile.AvatarURL
		}
		if avatarURL == "" {
			avatarURL = user.Img
		}
		row := po.AdminUserProfile{
			UserID:      user.ID,
			DisplayName: strings.TrimSpace(command.Nickname),
			AvatarURL:   avatarURL,
			Bio:         strings.TrimSpace(command.Bio),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"display_name": row.DisplayName,
				"avatar_url":   row.AvatarURL,
				"bio":          row.Bio,
				"updated_at":   row.UpdatedAt,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", command.UserID).First(&user).Error; err != nil {
			return err
		}
		item, err := r.toDomainAdminUserWithProfile(ctx, tx, user)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	return updated, err
}

func (r *Repository) UpdateAdminPassword(ctx context.Context, userID int64, passwordHash string) (domain.AdminUser, error) {
	if userID <= 0 {
		return domain.AdminUser{}, domain.ErrInvalidAdminUserID
	}
	if strings.TrimSpace(passwordHash) == "" {
		return domain.AdminUser{}, domain.ErrInvalidPassword
	}
	var updated domain.AdminUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user po.User
		if err := tx.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		if err := tx.WithContext(ctx).Model(&user).Updates(map[string]any{
			"password":    passwordHash,
			"update_time": time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		item, err := r.toDomainAdminUserWithProfile(ctx, tx, user)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	return updated, err
}

func (r *Repository) RoleKeysByUsername(ctx context.Context, username string) ([]string, error) {
	username = normalize(username)
	if username == "" {
		return nil, nil
	}
	var keys []string
	err := r.db.WithContext(ctx).
		Table("sys_role").
		Select("sys_role.key").
		Joins("JOIN sys_user_role ON sys_user_role.role_id = sys_role.id").
		Joins("JOIN sys_user ON sys_user.id = sys_user_role.user_id").
		Where("LOWER(sys_user.user_name) = ?", username).
		Where("sys_role.status = '' OR sys_role.status = ?", "1").
		Pluck("sys_role.key", &keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *Repository) RoleKeysByUserID(ctx context.Context, userID int64) ([]string, error) {
	if userID <= 0 {
		return nil, nil
	}
	var keys []string
	err := r.db.WithContext(ctx).
		Table("sys_role").
		Select("sys_role.key").
		Joins("JOIN sys_user_role ON sys_user_role.role_id = sys_role.id").
		Where("sys_user_role.user_id = ?", userID).
		Where("sys_role.status = '' OR sys_role.status = ?", "1").
		Order("sys_role.sort ASC, sys_role.id ASC").
		Pluck("sys_role.key", &keys).Error
	if err != nil {
		return nil, err
	}
	return normalizeList(keys), nil
}

func (r *Repository) PermissionsByRoleKeys(ctx context.Context, roles []string) ([]string, error) {
	roles = normalizeList(roles)
	if len(roles) == 0 {
		return nil, nil
	}
	rules, err := r.CasbinRules(ctx)
	if err != nil {
		return nil, err
	}
	return permissionsByRoleKeysFromRules(rules, roles), nil
}

func permissionsByRoleKeysFromRules(rules []po.CasbinRule, roles []string) []string {
	roles = normalizeList(roles)
	roleSet := map[string]struct{}{}
	for _, role := range roles {
		roleSet[role] = struct{}{}
	}
	for changed := true; changed; {
		changed = false
		for _, rule := range rules {
			if rule.Ptype != "g" {
				continue
			}
			child := normalize(rule.V0)
			parent := normalize(rule.V1)
			if child == "" || parent == "" {
				continue
			}
			if _, ok := roleSet[child]; !ok {
				continue
			}
			if _, ok := roleSet[parent]; ok {
				continue
			}
			roleSet[parent] = struct{}{}
			changed = true
		}
	}
	permissions := make([]string, 0)
	seen := map[string]struct{}{}
	for _, rule := range rules {
		if rule.Ptype != "p" {
			continue
		}
		role := normalize(rule.V0)
		resource := normalize(rule.V1)
		action := normalize(rule.V2)
		if role == "" || resource == "" || action == "" {
			continue
		}
		if _, ok := roleSet[role]; !ok {
			continue
		}
		permission := resource + ":" + action
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		permissions = append(permissions, permission)
	}
	return permissions
}

func individualPermissionsByRoleKeysFromRules(rules []po.CasbinRule, roleKeys []string) map[string][]string {
	roleKeys = normalizeList(roleKeys)
	permissionsByRole := make(map[string][]string, len(roleKeys))
	for _, roleKey := range roleKeys {
		permissionsByRole[roleKey] = permissionsByRoleKeysFromRules(rules, []string{roleKey})
	}
	return permissionsByRole
}

func (r *Repository) UpdateAdminLastLogin(ctx context.Context, userID int64, loginIP string) error {
	if userID <= 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&po.User{}).Where("id = ?", userID).Updates(map[string]any{
		"last_login_time": time.Now(),
		"last_login_ip":   strings.TrimSpace(loginIP),
		"update_time":     time.Now(),
	}).Error
}

func (r *Repository) ListAdminUsers(ctx context.Context, query string, limit int32, offset int32) (domain.AdminUserList, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	db := r.db.WithContext(ctx).Model(&po.User{})
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(user_name) LIKE ? OR LOWER(email) LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.AdminUserList{}, err
	}
	var users []po.User
	if err := db.Order("id ASC").Limit(int(limit)).Offset(int(offset)).Find(&users).Error; err != nil {
		return domain.AdminUserList{}, err
	}
	items, err := r.toDomainAdminUsersWithRoles(ctx, users)
	if err != nil {
		return domain.AdminUserList{}, err
	}
	return domain.AdminUserList{Items: items, Total: total}, nil
}

func (r *Repository) CreateAdminUser(ctx context.Context, command domain.CreateAdminUserCommand, passwordHash string) (domain.AdminUser, error) {
	username := normalize(command.Username)
	email := normalize(command.Email)
	phone := strings.TrimSpace(command.Phone)
	roleKeys := normalizeList(command.RoleKeys)
	if username == "" {
		return domain.AdminUser{}, domain.ErrInvalidCredentials
	}
	if strings.TrimSpace(passwordHash) == "" {
		return domain.AdminUser{}, domain.ErrInvalidPassword
	}
	if len(roleKeys) == 0 {
		return domain.AdminUser{}, domain.ErrInvalidRoleKeys
	}
	if phone == "" {
		phone = adminPhone(username)
	}
	var created domain.AdminUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exists, err := adminUserExists(ctx, tx, username, email, phone)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrAdminUserExists
		}
		roles, err := rolesByKeys(ctx, tx, roleKeys)
		if err != nil {
			return err
		}
		if len(roles) != len(roleKeys) {
			return domain.ErrInvalidRoleKeys
		}
		now := time.Now()
		user := po.User{
			Username:      username,
			Phone:         phone,
			Email:         email,
			Password:      passwordHash,
			Status:        1,
			RoleId:        roles[0].ID,
			DeptId:        1,
			PostId:        1,
			LastLoginTime: now,
		}
		user.CreateTime = now
		user.UpdateTime = now
		if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
			if isAdminUserIdentityUniqueViolation(err) {
				return domain.ErrAdminUserExists
			}
			return err
		}
		if err := replaceUserRoles(ctx, tx, user.ID, roles); err != nil {
			return err
		}
		displayName := strings.TrimSpace(command.Nickname)
		if displayName == "" {
			displayName = username
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po.AdminUserProfile{
			UserID:      user.ID,
			DisplayName: displayName,
			CreatedAt:   now,
			UpdatedAt:   now,
		}).Error; err != nil {
			return err
		}
		item, err := r.toDomainAdminUserWithProfile(ctx, tx, user)
		if err != nil {
			return err
		}
		created = item
		created.Roles = roleKeys
		return nil
	})
	return created, err
}

func (r *Repository) ListRoles(ctx context.Context) (domain.RoleList, error) {
	var roles []po.Role
	if err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&roles).Error; err != nil {
		return domain.RoleList{}, err
	}
	items := make([]domain.Role, 0, len(roles))
	if len(roles) == 0 {
		return domain.RoleList{Items: items, Total: int64(len(items))}, nil
	}
	rules, err := r.CasbinRules(ctx)
	if err != nil {
		return domain.RoleList{}, err
	}
	roleKeys := make([]string, 0, len(roles))
	for _, role := range roles {
		roleKeys = append(roleKeys, role.Key)
	}
	permissionsByRole := individualPermissionsByRoleKeysFromRules(rules, roleKeys)
	for _, role := range roles {
		item := toDomainRole(role)
		item.Permissions = permissionsByRole[normalize(role.Key)]
		items = append(items, item)
	}
	return domain.RoleList{Items: items, Total: int64(len(items))}, nil
}

func (r *Repository) AssignRoles(ctx context.Context, userID int64, roleKeys []string) (domain.AdminUser, error) {
	roleKeys = normalizeList(roleKeys)
	if userID <= 0 {
		return domain.AdminUser{}, domain.ErrInvalidAdminUserID
	}
	if len(roleKeys) == 0 {
		return domain.AdminUser{}, domain.ErrInvalidRoleKeys
	}
	var updated domain.AdminUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user po.User
		if err := tx.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		roles, err := rolesByKeys(ctx, tx, roleKeys)
		if err != nil {
			return err
		}
		if len(roles) != len(roleKeys) {
			return domain.ErrInvalidRoleKeys
		}
		if err := replaceUserRoles(ctx, tx, userID, roles); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&user).Updates(map[string]any{
			"role_id":     roles[0].ID,
			"update_time": time.Now(),
		}).Error; err != nil {
			return err
		}
		item, err := r.toDomainAdminUserWithProfile(ctx, tx, user)
		if err != nil {
			return err
		}
		updated = item
		updated.Roles = roleKeys
		return nil
	})
	return updated, err
}

func (r *Repository) ListBadges(ctx context.Context, status int32, limit int32, offset int32) (domain.BadgeList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.Badge{})
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.BadgeList{}, err
	}
	var rows []po.Badge
	if err := db.Order("sort ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.BadgeList{}, err
	}
	items := make([]domain.Badge, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainBadge(row))
	}
	return domain.BadgeList{Items: items, Total: total}, nil
}

func (r *Repository) UpsertBadge(ctx context.Context, command domain.UpsertBadgeCommand) (domain.Badge, error) {
	now := time.Now()
	if command.Status <= 0 {
		command.Status = 2
	}
	if strings.TrimSpace(command.RuleType) == "" {
		command.RuleType = "manual"
	}
	if command.ID > 0 {
		var row po.Badge
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Badge{}, domain.ErrInvalidBadgeID
			}
			return domain.Badge{}, err
		}
		updates := map[string]any{
			"name":        strings.TrimSpace(command.Name),
			"description": strings.TrimSpace(command.Description),
			"icon_url":    strings.TrimSpace(command.IconURL),
			"rule_type":   normalize(command.RuleType),
			"rule_value":  command.RuleValue,
			"status":      command.Status,
			"sort":        command.Sort,
			"updated_at":  now,
		}
		if key := normalizeKey(command.Key); key != "" {
			updates["key"] = key
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.Badge{}, err
		}
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			return domain.Badge{}, err
		}
		return toDomainBadge(row), nil
	}
	row := po.Badge{
		Key:         fallbackKey("badge", command.Key, command.Name),
		Name:        strings.TrimSpace(command.Name),
		Description: strings.TrimSpace(command.Description),
		IconURL:     strings.TrimSpace(command.IconURL),
		RuleType:    normalize(command.RuleType),
		RuleValue:   command.RuleValue,
		Status:      command.Status,
		Sort:        command.Sort,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if row.RuleType == "" {
		row.RuleType = "manual"
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Badge{}, err
	}
	return toDomainBadge(row), nil
}

func (r *Repository) DeleteBadge(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&po.Badge{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrInvalidBadgeID
	}
	return nil
}

func (r *Repository) ListLevels(ctx context.Context, status int32, limit int32, offset int32) (domain.LevelList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.Level{})
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.LevelList{}, err
	}
	var rows []po.Level
	if err := db.Order("sort ASC, min_score ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.LevelList{}, err
	}
	items := make([]domain.Level, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainLevel(row))
	}
	return domain.LevelList{Items: items, Total: total}, nil
}

func (r *Repository) UpsertLevel(ctx context.Context, command domain.UpsertLevelCommand) (domain.Level, error) {
	now := time.Now()
	if command.Status <= 0 {
		command.Status = 2
	}
	if command.ID > 0 {
		var row po.Level
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Level{}, domain.ErrInvalidLevelID
			}
			return domain.Level{}, err
		}
		updates := map[string]any{
			"name":        strings.TrimSpace(command.Name),
			"description": strings.TrimSpace(command.Description),
			"min_score":   command.MinScore,
			"max_score":   command.MaxScore,
			"status":      command.Status,
			"sort":        command.Sort,
			"updated_at":  now,
		}
		if key := normalizeKey(command.Key); key != "" {
			updates["key"] = key
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.Level{}, err
		}
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			return domain.Level{}, err
		}
		return toDomainLevel(row), nil
	}
	row := po.Level{
		Key:         fallbackKey("level", command.Key, command.Name),
		Name:        strings.TrimSpace(command.Name),
		Description: strings.TrimSpace(command.Description),
		MinScore:    command.MinScore,
		MaxScore:    command.MaxScore,
		Status:      command.Status,
		Sort:        command.Sort,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Level{}, err
	}
	return toDomainLevel(row), nil
}

func (r *Repository) DeleteLevel(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&po.Level{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrInvalidLevelID
	}
	return nil
}

func (r *Repository) ListForbiddenWords(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.ForbiddenWordList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.ForbiddenWord{})
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(word) LIKE ? OR LOWER(scene) LIKE ? OR LOWER(action) LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.ForbiddenWordList{}, err
	}
	var rows []po.ForbiddenWord
	if err := db.Order("updated_at DESC, id DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.ForbiddenWordList{}, err
	}
	items := make([]domain.ForbiddenWord, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainForbiddenWord(row))
	}
	return domain.ForbiddenWordList{Items: items, Total: total}, nil
}

func (r *Repository) UpsertForbiddenWord(ctx context.Context, command domain.UpsertForbiddenWordCommand) (domain.ForbiddenWord, error) {
	now := time.Now()
	if command.Status <= 0 {
		command.Status = 2
	}
	scene := normalize(command.Scene)
	if scene == "" {
		scene = "content"
	}
	action := normalize(command.Action)
	if action == "" {
		action = "reject"
	}
	if command.ID > 0 {
		var row po.ForbiddenWord
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ForbiddenWord{}, domain.ErrInvalidForbiddenWordID
			}
			return domain.ForbiddenWord{}, err
		}
		updates := map[string]any{
			"word":        strings.TrimSpace(command.Word),
			"scene":       scene,
			"action":      action,
			"replacement": strings.TrimSpace(command.Replacement),
			"description": strings.TrimSpace(command.Description),
			"status":      command.Status,
			"updated_at":  now,
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.ForbiddenWord{}, err
		}
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			return domain.ForbiddenWord{}, err
		}
		return toDomainForbiddenWord(row), nil
	}
	row := po.ForbiddenWord{
		Word:        strings.TrimSpace(command.Word),
		Scene:       scene,
		Action:      action,
		Replacement: strings.TrimSpace(command.Replacement),
		Description: strings.TrimSpace(command.Description),
		Status:      command.Status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ForbiddenWord{}, err
	}
	return toDomainForbiddenWord(row), nil
}

func (r *Repository) DeleteForbiddenWord(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&po.ForbiddenWord{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrInvalidForbiddenWordID
	}
	return nil
}

func (r *Repository) ListSettings(ctx context.Context, group string, status int32, limit int32, offset int32) (domain.SettingList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.SiteSetting{})
	group = normalize(group)
	if group != "" {
		db = db.Where("setting_group = ?", group)
	}
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.SettingList{}, err
	}
	var rows []po.SiteSetting
	if err := db.Order("setting_group ASC, key ASC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.SettingList{}, err
	}
	items := make([]domain.Setting, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainSetting(row))
	}
	return domain.SettingList{Items: items, Total: total}, nil
}

func (r *Repository) UpsertSetting(ctx context.Context, command domain.UpsertSettingCommand) (domain.Setting, error) {
	now := time.Now()
	if command.Status <= 0 {
		command.Status = 2
	}
	group := normalize(command.Group)
	if group == "" {
		group = "site"
	}
	valueType := normalize(command.ValueType)
	if valueType == "" {
		valueType = "string"
	}
	if command.ID > 0 {
		var row po.SiteSetting
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Setting{}, domain.ErrInvalidSettingID
			}
			return domain.Setting{}, err
		}
		updates := map[string]any{
			"key":           normalizeSettingKey(command.Key),
			"setting_group": group,
			"value_type":    valueType,
			"description":   strings.TrimSpace(command.Description),
			"status":        command.Status,
			"updated_at":    now,
		}
		if !command.PreserveValue {
			updates["value"] = strings.TrimSpace(command.Value)
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.Setting{}, err
		}
		var refreshed po.SiteSetting
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&refreshed).Error; err != nil {
			return domain.Setting{}, err
		}
		return toDomainSetting(refreshed), nil
	}
	key := normalizeSettingKey(command.Key)
	var row po.SiteSetting
	res := r.db.WithContext(ctx).Where("key = ?", key).Find(&row)
	if res.Error != nil {
		return domain.Setting{}, res.Error
	}
	if res.RowsAffected > 0 {
		updates := map[string]any{
			"setting_group": group,
			"value_type":    valueType,
			"description":   strings.TrimSpace(command.Description),
			"status":        command.Status,
			"updated_at":    now,
		}
		if !command.PreserveValue {
			updates["value"] = strings.TrimSpace(command.Value)
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.Setting{}, err
		}
		var refreshed po.SiteSetting
		if err := r.db.WithContext(ctx).Where("key = ?", key).First(&refreshed).Error; err != nil {
			return domain.Setting{}, err
		}
		return toDomainSetting(refreshed), nil
	}
	row = po.SiteSetting{
		Key:         key,
		Group:       group,
		ValueType:   valueType,
		Description: strings.TrimSpace(command.Description),
		Status:      command.Status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if !command.PreserveValue {
		row.Value = strings.TrimSpace(command.Value)
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Setting{}, err
	}
	return toDomainSetting(row), nil
}

func (r *Repository) ListEmailLogs(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.EmailLogList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.EmailLog{})
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(mail_to) LIKE ? OR LOWER(subject) LIKE ? OR LOWER(template_key) LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.EmailLogList{}, err
	}
	var rows []po.EmailLog
	if err := db.Order("created_at DESC, id DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.EmailLogList{}, err
	}
	items := make([]domain.EmailLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainEmailLog(row))
	}
	return domain.EmailLogList{Items: items, Total: total}, nil
}

func (r *Repository) ListLinks(ctx context.Context, status int32, limit int32, offset int32) (domain.LinkList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.Link{})
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.LinkList{}, err
	}
	var rows []po.Link
	if err := db.Order("sort ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.LinkList{}, err
	}
	items := make([]domain.Link, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainLink(row))
	}
	return domain.LinkList{Items: items, Total: total}, nil
}

func (r *Repository) UpsertLink(ctx context.Context, command domain.UpsertLinkCommand) (domain.Link, error) {
	now := time.Now()
	if command.Status <= 0 {
		command.Status = 2
	}
	if command.ID > 0 {
		var row po.Link
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Link{}, domain.ErrInvalidLinkID
			}
			return domain.Link{}, err
		}
		updates := map[string]any{
			"title":       strings.TrimSpace(command.Title),
			"url":         strings.TrimSpace(command.URL),
			"description": strings.TrimSpace(command.Description),
			"status":      command.Status,
			"sort":        command.Sort,
			"updated_at":  now,
		}
		if key := normalizeKey(command.Key); key != "" {
			updates["key"] = key
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.Link{}, err
		}
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			return domain.Link{}, err
		}
		return toDomainLink(row), nil
	}
	row := po.Link{
		Key:         fallbackKey("link", command.Key, command.Title),
		Title:       strings.TrimSpace(command.Title),
		URL:         strings.TrimSpace(command.URL),
		Description: strings.TrimSpace(command.Description),
		Status:      command.Status,
		Sort:        command.Sort,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Link{}, err
	}
	return toDomainLink(row), nil
}

func (r *Repository) DeleteLink(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&po.Link{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrInvalidLinkID
	}
	return nil
}

func (r *Repository) ListTasks(ctx context.Context, status int32, limit int32, offset int32) (domain.TaskList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.Task{})
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.TaskList{}, err
	}
	var rows []po.Task
	if err := db.Order("sort ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.TaskList{}, err
	}
	items := make([]domain.Task, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainTask(row))
	}
	return domain.TaskList{Items: items, Total: total}, nil
}

func (r *Repository) UpsertTask(ctx context.Context, command domain.UpsertTaskCommand) (domain.Task, error) {
	now := time.Now()
	if command.Status <= 0 {
		command.Status = 2
	}
	if command.ID > 0 {
		var row po.Task
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Task{}, domain.ErrInvalidTaskID
			}
			return domain.Task{}, err
		}
		updates := map[string]any{
			"title":         strings.TrimSpace(command.Title),
			"description":   strings.TrimSpace(command.Description),
			"reward_points": command.RewardPoints,
			"status":        command.Status,
			"sort":          command.Sort,
			"updated_at":    now,
		}
		if key := normalizeTaskKey(command.Key); key != "" {
			updates["key"] = key
		}
		if err := r.db.WithContext(ctx).Model(&row).Updates(updates).Error; err != nil {
			return domain.Task{}, err
		}
		if err := r.db.WithContext(ctx).Where("id = ?", command.ID).First(&row).Error; err != nil {
			return domain.Task{}, err
		}
		return toDomainTask(row), nil
	}
	row := po.Task{
		Key:          fallbackTaskKey(command.Key, command.Title),
		Title:        strings.TrimSpace(command.Title),
		Description:  strings.TrimSpace(command.Description),
		RewardPoints: command.RewardPoints,
		Status:       command.Status,
		Sort:         command.Sort,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Task{}, err
	}
	return toDomainTask(row), nil
}

func (r *Repository) DeleteTask(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&po.Task{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrInvalidTaskID
	}
	return nil
}

func (r *Repository) CasbinRules(ctx context.Context) ([]po.CasbinRule, error) {
	var rules []po.CasbinRule
	err := r.db.WithContext(ctx).Order("id ASC").Find(&rules).Error
	return rules, err
}

func (r *Repository) toDomainAdminUserWithRoles(ctx context.Context, user po.User) (domain.AdminUser, error) {
	item, err := r.toDomainAdminUserWithProfile(ctx, r.db, user)
	if err != nil {
		return domain.AdminUser{}, err
	}
	roles, err := r.RoleKeysByUserID(ctx, user.ID)
	if err != nil {
		return domain.AdminUser{}, err
	}
	item.Roles = roles
	return item, nil
}

func (r *Repository) toDomainAdminUsersWithRoles(ctx context.Context, users []po.User) ([]domain.AdminUser, error) {
	items := make([]domain.AdminUser, 0, len(users))
	if len(users) == 0 {
		return items, nil
	}
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	profiles, err := adminProfilesByUserIDs(ctx, r.db, userIDs)
	if err != nil {
		return nil, err
	}
	roleAssignments, err := systemRoleAssignmentsByUserIDs(ctx, r.db, userIDs, true)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		item := toDomainAdminUser(user)
		applyAdminUserProfile(&item, profiles[user.ID])
		item.Roles = roleAssignments[user.ID].RoleKeys
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) toDomainAdminUserWithProfile(ctx context.Context, tx *gorm.DB, user po.User) (domain.AdminUser, error) {
	item := toDomainAdminUser(user)
	profile, err := adminProfileByUserID(ctx, tx, user.ID)
	if err != nil {
		return domain.AdminUser{}, err
	}
	applyAdminUserProfile(&item, profile)
	return item, nil
}

func applyAdminUserProfile(item *domain.AdminUser, profile po.AdminUserProfile) {
	if item == nil || profile.UserID <= 0 {
		return
	}
	if strings.TrimSpace(profile.DisplayName) != "" {
		item.Nickname = profile.DisplayName
	}
	item.AvatarURL = profile.AvatarURL
	item.Bio = profile.Bio
}

func adminProfileByUserID(ctx context.Context, tx *gorm.DB, userID int64) (po.AdminUserProfile, error) {
	var profile po.AdminUserProfile
	if userID <= 0 {
		return profile, nil
	}
	err := tx.WithContext(ctx).Where("user_id = ?", userID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return po.AdminUserProfile{}, nil
	}
	return profile, err
}

func adminProfilesByUserIDs(ctx context.Context, tx *gorm.DB, userIDs []int64) (map[int64]po.AdminUserProfile, error) {
	userIDs = uniqueInt64s(userIDs)
	profilesByUserID := make(map[int64]po.AdminUserProfile, len(userIDs))
	if len(userIDs) == 0 {
		return profilesByUserID, nil
	}
	var profiles []po.AdminUserProfile
	if err := tx.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&profiles).Error; err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		profilesByUserID[profile.UserID] = profile
	}
	return profilesByUserID, nil
}

func adminUserExists(ctx context.Context, tx *gorm.DB, username string, email string, phone string) (bool, error) {
	return adminUserExistsExcluding(ctx, tx, username, email, phone, 0)
}

func adminUserExistsExcluding(ctx context.Context, tx *gorm.DB, username string, email string, phone string, excludeID int64) (bool, error) {
	username = normalize(username)
	email = normalize(email)
	phone = strings.TrimSpace(phone)
	conditions := []string{"LOWER(user_name) = ?"}
	args := []any{username}
	if email != "" {
		conditions = append(conditions, "LOWER(email) = ?")
		args = append(args, email)
	}
	if phone != "" {
		conditions = append(conditions, "phone = ?")
		args = append(args, phone)
	}
	db := tx.WithContext(ctx).Model(&po.User{}).Where("("+strings.Join(conditions, " OR ")+")", args...)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func isAdminUserIdentityUniqueViolation(err error) bool {
	constraintName, ok := postgresUniqueConstraintName(err)
	if !ok {
		return false
	}
	switch constraintName {
	case "idx_sys_user_phone",
		"idx_sys_user_username_lower_unique",
		"idx_sys_user_email_lower_unique":
		return true
	default:
		return false
	}
}

func systemManagementUniqueViolationError(err error) error {
	constraintName, ok := postgresUniqueConstraintName(err)
	if !ok {
		return nil
	}
	switch constraintName {
	case "idx_sys_role_key_lower_unique":
		return domain.ErrSystemRoleExists
	case "idx_sys_menu_name_lower_unique":
		return domain.ErrSystemMenuExists
	case "idx_sys_dept_parent_name_lower_unique":
		return domain.ErrSystemDeptExists
	default:
		return nil
	}
}

func postgresUniqueConstraintName(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return "", false
	}
	return pgErr.ConstraintName, true
}

func rolesByKeys(ctx context.Context, tx *gorm.DB, roleKeys []string) ([]po.Role, error) {
	roleKeys = normalizeList(roleKeys)
	if len(roleKeys) == 0 {
		return nil, nil
	}
	var found []po.Role
	if err := tx.WithContext(ctx).
		Where("key IN ?", roleKeys).
		Where("status = '' OR status = ?", "1").
		Find(&found).Error; err != nil {
		return nil, err
	}
	byKey := make(map[string]po.Role, len(found))
	for _, role := range found {
		byKey[normalize(role.Key)] = role
	}
	ordered := make([]po.Role, 0, len(roleKeys))
	for _, key := range roleKeys {
		role, ok := byKey[key]
		if !ok {
			continue
		}
		ordered = append(ordered, role)
	}
	return ordered, nil
}

func replaceUserRoles(ctx context.Context, tx *gorm.DB, userID int64, roles []po.Role) error {
	if err := tx.WithContext(ctx).Where("user_id = ?", userID).Delete(&po.UserRole{}).Error; err != nil {
		return err
	}
	now := time.Now()
	relations := make([]po.UserRole, 0, len(roles))
	for _, role := range roles {
		relations = append(relations, po.UserRole{UserID: userID, RoleID: role.ID, CreatedAt: now})
	}
	if len(relations) == 0 {
		return nil
	}
	return tx.WithContext(ctx).CreateInBatches(&relations, roleRelationWriteBatchSize).Error
}

func upsertRole(ctx context.Context, tx *gorm.DB, role po.Role) error {
	role.Key = normalize(role.Key)
	var existing po.Role
	res := tx.WithContext(ctx).Where("key = ?", role.Key).Find(&existing)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		updates := map[string]any{
			"name":        role.Name,
			"status":      role.Status,
			"sort":        role.Sort,
			"admin":       role.Admin,
			"remark":      role.Remark,
			"update_time": time.Now(),
		}
		return tx.Model(&existing).Updates(updates).Error
	}
	return tx.WithContext(ctx).Create(&role).Error
}

func ensureAdminUser(ctx context.Context, tx *gorm.DB, username string, passwordHash string) error {
	username = normalize(username)
	if username == "" {
		return nil
	}
	var role po.Role
	if err := tx.WithContext(ctx).Where("key = ?", "admin").First(&role).Error; err != nil {
		return err
	}
	var user po.User
	res := tx.WithContext(ctx).Where("LOWER(user_name) = ?", username).Find(&user)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		now := time.Now()
		user = po.User{
			Username:      username,
			Phone:         adminPhone(username),
			Email:         username + "@admin.local",
			Password:      passwordHash,
			Status:        1,
			RoleId:        role.ID,
			DeptId:        1,
			PostId:        1,
			LastLoginTime: now,
		}
		user.CreateTime = now
		user.UpdateTime = now
		if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
			return err
		}
	} else {
		updates := map[string]any{}
		if user.RoleId != role.ID {
			updates["role_id"] = role.ID
		}
		if strings.TrimSpace(user.Phone) == "" {
			updates["phone"] = adminPhone(username)
		}
		if strings.TrimSpace(user.Password) == "" && passwordHash != "" {
			updates["password"] = passwordHash
		}
		if user.DeptId == 0 {
			updates["dept_id"] = 1
		}
		if user.PostId == 0 {
			updates["post_id"] = 1
		}
		if len(updates) > 0 {
			updates["update_time"] = time.Now()
			if err := tx.Model(&user).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&po.UserRole{
		UserID:    user.ID,
		RoleID:    role.ID,
		CreatedAt: time.Now(),
	}).Error
}

func hashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return "", nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func toDomainAdminUser(user po.User) domain.AdminUser {
	return domain.AdminUser{
		ID:           user.ID,
		Username:     user.Username,
		Email:        user.Email,
		Phone:        user.Phone,
		Nickname:     user.Username,
		AvatarURL:    user.Img,
		Bio:          user.IntroduceSign,
		Status:       int32(user.Status),
		LockedFlag:   int32(user.LockedFlag),
		PasswordHash: user.Password,
	}
}

func toDomainRole(role po.Role) domain.Role {
	return domain.Role{
		ID:     role.ID,
		Name:   role.Name,
		Key:    role.Key,
		Status: role.Status,
		Sort:   int32(role.Sort),
		Admin:  role.Admin,
		Remark: role.Remark,
	}
}

func toDomainBadge(badge po.Badge) domain.Badge {
	return domain.Badge{
		ID:          badge.ID,
		Key:         badge.Key,
		Name:        badge.Name,
		Description: badge.Description,
		IconURL:     badge.IconURL,
		RuleType:    badge.RuleType,
		RuleValue:   badge.RuleValue,
		Status:      badge.Status,
		Sort:        badge.Sort,
		CreatedAt:   badge.CreatedAt.UnixMilli(),
		UpdatedAt:   badge.UpdatedAt.UnixMilli(),
	}
}

func toDomainLevel(level po.Level) domain.Level {
	return domain.Level{
		ID:          level.ID,
		Key:         level.Key,
		Name:        level.Name,
		Description: level.Description,
		MinScore:    level.MinScore,
		MaxScore:    level.MaxScore,
		Status:      level.Status,
		Sort:        level.Sort,
		CreatedAt:   level.CreatedAt.UnixMilli(),
		UpdatedAt:   level.UpdatedAt.UnixMilli(),
	}
}

func toDomainForbiddenWord(word po.ForbiddenWord) domain.ForbiddenWord {
	return domain.ForbiddenWord{
		ID:          word.ID,
		Word:        word.Word,
		Scene:       word.Scene,
		Action:      word.Action,
		Replacement: word.Replacement,
		Description: word.Description,
		Status:      word.Status,
		CreatedAt:   word.CreatedAt.UnixMilli(),
		UpdatedAt:   word.UpdatedAt.UnixMilli(),
	}
}

func toDomainSetting(setting po.SiteSetting) domain.Setting {
	return domain.Setting{
		ID:          setting.ID,
		Key:         setting.Key,
		Value:       setting.Value,
		Group:       setting.Group,
		ValueType:   setting.ValueType,
		Description: setting.Description,
		Status:      setting.Status,
		CreatedAt:   setting.CreatedAt.UnixMilli(),
		UpdatedAt:   setting.UpdatedAt.UnixMilli(),
	}
}

func toDomainEmailLog(log po.EmailLog) domain.EmailLog {
	return domain.EmailLog{
		ID:          log.ID,
		To:          log.To,
		Subject:     log.Subject,
		TemplateKey: log.TemplateKey,
		Provider:    log.Provider,
		Status:      log.Status,
		Error:       log.Error,
		CreatedAt:   log.CreatedAt.UnixMilli(),
		UpdatedAt:   log.UpdatedAt.UnixMilli(),
	}
}

func toDomainLink(link po.Link) domain.Link {
	return domain.Link{
		ID:          link.ID,
		Key:         link.Key,
		Title:       link.Title,
		URL:         link.URL,
		Description: link.Description,
		Status:      link.Status,
		Sort:        link.Sort,
		CreatedAt:   link.CreatedAt.UnixMilli(),
		UpdatedAt:   link.UpdatedAt.UnixMilli(),
	}
}

func toDomainTask(task po.Task) domain.Task {
	return domain.Task{
		ID:           task.ID,
		Key:          task.Key,
		Title:        task.Title,
		Description:  task.Description,
		RewardPoints: task.RewardPoints,
		Status:       task.Status,
		Sort:         task.Sort,
		CreatedAt:    task.CreatedAt.UnixMilli(),
		UpdatedAt:    task.UpdatedAt.UnixMilli(),
	}
}

func seedDefaultOperations(ctx context.Context, tx *gorm.DB) error {
	now := time.Now()
	badges := []po.Badge{
		{Key: "community-member", Name: "社区成员", Description: "已创建社区账号并加入讨论。", RuleType: "account_created", RuleValue: 1, Status: 2, Sort: 10, CreatedAt: now, UpdatedAt: now},
		{Key: "first-following", Name: "开始关注", Description: "已经关注第一位社区成员。", RuleType: "following_count", RuleValue: 1, Status: 2, Sort: 20, CreatedAt: now, UpdatedAt: now},
		{Key: "first-follower", Name: "获得关注", Description: "已经获得第一位粉丝。", RuleType: "follower_count", RuleValue: 1, Status: 2, Sort: 30, CreatedAt: now, UpdatedAt: now},
		{Key: "trusted-author", Name: "受信作者", Description: "粉丝数达到 10，具备稳定影响力。", RuleType: "follower_count", RuleValue: 10, Status: 2, Sort: 40, CreatedAt: now, UpdatedAt: now},
	}
	for _, badge := range badges {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "icon_url", "rule_type", "rule_value", "status", "sort", "updated_at"}),
		}).Create(&badge).Error; err != nil {
			return err
		}
	}
	levels := []po.Level{
		{Key: "lv1", Name: "Lv.1 新成员", Description: "刚加入社区的基础等级。", MinScore: 0, MaxScore: 99, Status: 2, Sort: 10, CreatedAt: now, UpdatedAt: now},
		{Key: "lv2", Name: "Lv.2 活跃成员", Description: "开始稳定参与社区互动。", MinScore: 100, MaxScore: 499, Status: 2, Sort: 20, CreatedAt: now, UpdatedAt: now},
		{Key: "lv3", Name: "Lv.3 贡献者", Description: "持续发布内容并获得认可。", MinScore: 500, MaxScore: 1999, Status: 2, Sort: 30, CreatedAt: now, UpdatedAt: now},
		{Key: "lv4", Name: "Lv.4 核心成员", Description: "长期贡献社区内容和互动。", MinScore: 2000, MaxScore: 0, Status: 2, Sort: 40, CreatedAt: now, UpdatedAt: now},
	}
	for _, level := range levels {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "min_score", "max_score", "status", "sort", "updated_at"}),
		}).Create(&level).Error; err != nil {
			return err
		}
	}
	forbiddenWords := []po.ForbiddenWord{
		{Word: "spam", Scene: "content", Action: "reject", Replacement: "", Description: "垃圾内容基础拦截词。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Word: "广告", Scene: "content", Action: "review", Replacement: "", Description: "疑似广告内容进入审核。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Word: "测试敏感词", Scene: "comment", Action: "replace", Replacement: "***", Description: "本地联调用默认替换词。", Status: 1, CreatedAt: now, UpdatedAt: now},
	}
	for _, word := range forbiddenWords {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "word"}, {Name: "scene"}},
			DoNothing: true,
		}).Create(&word).Error; err != nil {
			return err
		}
	}
	settings := []po.SiteSetting{
		{Key: "site_name", Value: "BBS 社区", Group: "site", ValueType: "string", Description: "站点名称。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "site_description", Value: "面向内容沉淀、圈子协作和技术交流的社区论坛。", Group: "site", ValueType: "string", Description: "站点描述。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "site_logo_url", Value: "", Group: "site", ValueType: "string", Description: "C 端站点 Logo URL；为空时显示站点名称。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "site_navigation", Value: `[{"key":"home","label":"首页"},{"key":"plaza","label":"广场"},{"key":"circles","label":"圈子"},{"key":"chat","label":"聊天室"},{"key":"help","label":"求助"},{"key":"resources","label":"资源"},{"key":"shop","label":"商城"},{"key":"member","label":"会员"},{"key":"more","label":"更多"}]`, Group: "site", ValueType: "json", Description: "C 端主导航 JSON；仅支持内置页面 key 的排序、显示和改名。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "seo_keywords", Value: "bbs,community,forum", Group: "seo", ValueType: "string", Description: "SEO 关键词。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "upload_max_size_mb", Value: "20", Group: "upload", ValueType: "int", Description: "单文件上传大小限制。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "email_from_name", Value: "BBS 社区", Group: "email", ValueType: "string", Description: "邮件发件人名称。", Status: 2, CreatedAt: now, UpdatedAt: now},
	}
	for _, setting := range settings {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"setting_group": setting.Group,
				"value_type":    setting.ValueType,
				"description":   setting.Description,
				"status":        setting.Status,
				"updated_at":    setting.UpdatedAt,
			}),
		}).Create(&setting).Error; err != nil {
			return err
		}
	}
	authSettings := []po.SiteSetting{
		{Key: "auth.password.enabled", Value: "true", Group: "auth", ValueType: "bool", Description: "是否允许 C 端账号密码登录。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.register.enabled", Value: "true", Group: "auth", ValueType: "bool", Description: "是否允许 C 端账号密码注册。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.email_verification.required", Value: "false", Group: "auth", ValueType: "bool", Description: "是否要求 C 端用户完成邮箱验证后才能发布内容或评论。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.github.enabled", Value: "false", Group: "auth", ValueType: "bool", Description: "是否开启 GitHub 登录。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.github.client_id", Value: "", Group: "auth", ValueType: "string", Description: "GitHub OAuth Client ID。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.github.client_secret", Value: "", Group: "auth", ValueType: "password", Description: "GitHub OAuth Client Secret。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.github.min_account_years", Value: "3", Group: "auth", ValueType: "int", Description: "GitHub 登录要求账号至少创建的年限。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.google.enabled", Value: "false", Group: "auth", ValueType: "bool", Description: "是否开启 Google 登录。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.google.client_id", Value: "", Group: "auth", ValueType: "string", Description: "Google OAuth Client ID。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.google.client_secret", Value: "", Group: "auth", ValueType: "password", Description: "Google OAuth Client Secret。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.qq.enabled", Value: "false", Group: "auth", ValueType: "bool", Description: "是否开启 QQ 登录。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.qq.client_id", Value: "", Group: "auth", ValueType: "string", Description: "QQ Connect App ID。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.qq.client_secret", Value: "", Group: "auth", ValueType: "password", Description: "QQ Connect App Key。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "auth.oauth.frontend_callback_url", Value: "http://127.0.0.1:8850/auth/callback", Group: "auth", ValueType: "string", Description: "C 端 OAuth 登录完成后的回跳地址。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "site.webmaster.username", Value: "webmaster", Group: "site", ValueType: "string", Description: "C 端站长账号用户名。", Status: 2, CreatedAt: now, UpdatedAt: now},
		{Key: "site.webmaster.password", Value: "", Group: "site", ValueType: "password", Description: "C 端站长账号密码；为空时不启用站长账号直登。", Status: 2, CreatedAt: now, UpdatedAt: now},
	}
	for _, setting := range authSettings {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"setting_group": setting.Group,
				"value_type":    setting.ValueType,
				"description":   setting.Description,
				"status":        setting.Status,
				"updated_at":    setting.UpdatedAt,
			}),
		}).Create(&setting).Error; err != nil {
			return err
		}
	}
	links := []po.Link{
		{Key: "go-docs", Title: "Go 官方文档", URL: "https://go.dev/doc/", Description: "语言、标准库和工具链文档入口。", Status: 2, Sort: 10, CreatedAt: now, UpdatedAt: now},
		{Key: "react-router", Title: "React Router", URL: "https://reactrouter.com/", Description: "前端路由能力参考，当前社区前端已接入。", Status: 2, Sort: 20, CreatedAt: now, UpdatedAt: now},
		{Key: "elasticsearch-guide", Title: "Elasticsearch Guide", URL: "https://www.elastic.co/guide/", Description: "搜索服务索引、查询和运维参考。", Status: 2, Sort: 30, CreatedAt: now, UpdatedAt: now},
	}
	for _, link := range links {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "url", "description", "status", "sort", "updated_at"}),
		}).Create(&link).Error; err != nil {
			return err
		}
	}
	legacyTaskKeys := []string{"first-topic", "first-comment", "complete-profile"}
	if err := tx.WithContext(ctx).Model(&po.Task{}).Where("key IN ? AND status = ?", legacyTaskKeys, 2).Updates(map[string]any{
		"status":     1,
		"updated_at": now,
	}).Error; err != nil {
		return err
	}
	tasks := []po.Task{
		{Key: "daily_check_in", Title: "每日签到", Description: "完成当天签到后领取额外积分奖励。", RewardPoints: 5, Status: 2, Sort: 10, CreatedAt: now, UpdatedAt: now},
		{Key: "first_topic", Title: "发布第一条话题", Description: "成功发布首个社区话题后领取积分奖励。", RewardPoints: 20, Status: 2, Sort: 20, CreatedAt: now, UpdatedAt: now},
		{Key: "first_comment", Title: "完成一次评论", Description: "成功发表首条话题或文章评论后领取积分奖励。", RewardPoints: 10, Status: 2, Sort: 30, CreatedAt: now, UpdatedAt: now},
	}
	for _, task := range tasks {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoNothing: true,
		}).Create(&task).Error; err != nil {
			return err
		}
	}
	return nil
}

func adminPhone(username string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(username))
	return fmt.Sprintf("%011d", h.Sum64()%100000000000)
}

func policy(sub string, obj string, act string) po.CasbinRule {
	return po.CasbinRule{Ptype: "p", V0: sub, V1: obj, V2: act}
}

func grouping(sub string, role string) po.CasbinRule {
	return po.CasbinRule{Ptype: "g", V0: sub, V1: role}
}

func defaultCasbinRules() []po.CasbinRule {
	return []po.CasbinRule{
		policy("admin", domain.ResourceSystem, string(domain.ActionViewDashboard)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionListReports)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionAuditReport)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionListArticles)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionPublishArticle)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionHideArticle)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionListTopics)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionPublishTopic)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionHideTopic)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionListComments)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionHideComment)),
		policy("moderator", domain.ResourceGovernance, string(domain.ActionRestoreComment)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListUsers)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionMuteUser)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUnmuteUser)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionArchiveArticle)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionArchiveTopic)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListCategories)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateCategory)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateCategory)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionDeleteCategory)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListAdminUsers)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateAdminUser)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListRoles)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionAssignRoles)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListBadges)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateBadge)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateBadge)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionDeleteBadge)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListLevels)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateLevel)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateLevel)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionDeleteLevel)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListForbiddenWords)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateForbiddenWord)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateForbiddenWord)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionDeleteForbiddenWord)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListEmailLogs)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListSettings)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateSetting)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListLinks)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateLink)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateLink)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionDeleteLink)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListTasks)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionCreateTask)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionUpdateTask)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionDeleteTask)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionListUserCredits)),
		policy("admin", domain.ResourceGovernance, string(domain.ActionAdjustUserCredits)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallProductCategories)),
		policy("admin", domain.ResourceMall, string(domain.ActionCreateMallProductCategory)),
		policy("admin", domain.ResourceMall, string(domain.ActionUpdateMallProductCategory)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallProducts)),
		policy("admin", domain.ResourceMall, string(domain.ActionCreateMallProduct)),
		policy("admin", domain.ResourceMall, string(domain.ActionUpdateMallProduct)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallProductReviews)),
		policy("admin", domain.ResourceMall, string(domain.ActionUpdateMallProductReview)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallCoupons)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallCouponUsages)),
		policy("admin", domain.ResourceMall, string(domain.ActionCreateMallCoupon)),
		policy("admin", domain.ResourceMall, string(domain.ActionUpdateMallCoupon)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallOrders)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallDigitalEntitlements)),
		policy("admin", domain.ResourceMall, string(domain.ActionRevokeMallDigitalEntitlement)),
		policy("admin", domain.ResourceMall, string(domain.ActionCloseExpiredMall)),
		policy("admin", domain.ResourceMall, string(domain.ActionRecoverPayingMallOrders)),
		policy("admin", domain.ResourceMall, string(domain.ActionRequeueMallOutboxEvents)),
		policy("admin", domain.ResourceMall, string(domain.ActionUpdateMallOrder)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallOrderLogs)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallPayments)),
		policy("admin", domain.ResourceMall, string(domain.ActionListMallRefunds)),
		policy("admin", domain.ResourceMall, string(domain.ActionReviewMallRefunds)),
		policy("admin", domain.ResourceSystem, string(domain.ActionListSystemUsers)),
		policy("admin", domain.ResourceSystem, string(domain.ActionCreateSystemUser)),
		policy("admin", domain.ResourceSystem, string(domain.ActionUpdateSystemUser)),
		policy("admin", domain.ResourceSystem, string(domain.ActionDeleteSystemUser)),
		policy("admin", domain.ResourceSystem, string(domain.ActionResetSystemUserPass)),
		policy("admin", domain.ResourceSystem, string(domain.ActionAssignSystemUserRoles)),
		policy("admin", domain.ResourceSystem, string(domain.ActionListSystemRoles)),
		policy("admin", domain.ResourceSystem, string(domain.ActionCreateSystemRole)),
		policy("admin", domain.ResourceSystem, string(domain.ActionUpdateSystemRole)),
		policy("admin", domain.ResourceSystem, string(domain.ActionDeleteSystemRole)),
		policy("admin", domain.ResourceSystem, string(domain.ActionAssignSystemRoleMenus)),
		policy("admin", domain.ResourceSystem, string(domain.ActionListSystemMenus)),
		policy("admin", domain.ResourceSystem, string(domain.ActionCreateSystemMenu)),
		policy("admin", domain.ResourceSystem, string(domain.ActionUpdateSystemMenu)),
		policy("admin", domain.ResourceSystem, string(domain.ActionDeleteSystemMenu)),
		policy("admin", domain.ResourceSystem, string(domain.ActionListSystemDepts)),
		policy("admin", domain.ResourceSystem, string(domain.ActionCreateSystemDept)),
		policy("admin", domain.ResourceSystem, string(domain.ActionUpdateSystemDept)),
		policy("admin", domain.ResourceSystem, string(domain.ActionDeleteSystemDept)),
		policy("admin", domain.ResourceSystem, string(domain.ActionSendSystemNotification)),
		policy("admin", domain.ResourceSystem, string(domain.ActionRebuildSearch)),
		policy("admin", domain.ResourceSystem, string(domain.ActionViewSearchRebuild)),
		policy("admin", domain.ResourceSystem, string(domain.ActionListLoginLogs)),
		policy("admin", domain.ResourceSystem, string(domain.ActionListOperationLogs)),
		grouping("admin", "moderator"),
		grouping("superadmin", "admin"),
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = normalize(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizePage(limit int32, offset int32) (int32, int32) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func fallbackKey(prefix string, key string, title string) string {
	key = normalizeKey(key)
	if key != "" {
		return key
	}
	key = normalizeKey(title)
	if key != "" {
		return key
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func fallbackTaskKey(key string, title string) string {
	key = normalizeTaskKey(key)
	if key != "" {
		return key
	}
	key = normalizeTaskKey(title)
	if key != "" {
		return key
	}
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func normalizeKey(value string) string {
	value = normalize(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeTaskKey(value string) string {
	value = normalize(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeSettingKey(value string) string {
	value = normalize(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastSep := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSep = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSep = false
		case r == '_' || r == '.' || r == '-':
			if !lastSep {
				b.WriteRune(r)
				lastSep = true
			}
		default:
			if !lastSep {
				b.WriteByte('_')
				lastSep = true
			}
		}
	}
	return strings.Trim(b.String(), "_.-")
}
