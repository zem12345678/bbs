package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) ListSystemUsers(ctx context.Context, query string, status int32, page int32, pageSize int32) (domain.SystemUserList, error) {
	limit, offset := pageToLimitOffset(page, pageSize)
	db := r.db.WithContext(ctx).Model(&po.User{})
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(user_name) LIKE ? OR LOWER(email) LIKE ? OR phone LIKE ?", like, like, like)
	}
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.SystemUserList{}, err
	}
	var users []po.User
	if err := db.Order("id ASC").Limit(int(limit)).Offset(int(offset)).Find(&users).Error; err != nil {
		return domain.SystemUserList{}, err
	}
	items := make([]domain.SystemUser, 0, len(users))
	for _, user := range users {
		item, err := r.toDomainSystemUser(ctx, user)
		if err != nil {
			return domain.SystemUserList{}, err
		}
		items = append(items, item)
	}
	return domain.SystemUserList{Items: items, Total: total}, nil
}

func (r *Repository) CreateSystemUser(ctx context.Context, command domain.UpsertSystemUserCommand, passwordHash string) (domain.SystemUser, error) {
	username := normalize(command.Username)
	if username == "" || strings.TrimSpace(passwordHash) == "" {
		return domain.SystemUser{}, domain.ErrInvalidSystemUser
	}
	if domain.IsProtectedSystemUserName(username) {
		return domain.SystemUser{}, domain.ErrProtectedSystemUser
	}
	var created domain.SystemUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		phone := strings.TrimSpace(command.Phone)
		if phone == "" {
			phone = adminPhone(username)
		}
		exists, err := adminUserExists(ctx, tx, username, normalize(command.Email), phone)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrAdminUserExists
		}
		roles, err := systemRolesByIDs(ctx, tx, command.RoleIDs)
		if err != nil {
			return err
		}
		if len(roles) == 0 {
			role, err := defaultSystemRole(ctx, tx)
			if err != nil {
				return err
			}
			roles = []po.Role{role}
		}
		now := time.Now()
		user := po.User{
			Username:      username,
			Phone:         phone,
			Email:         normalize(command.Email),
			Img:           strings.TrimSpace(command.AvatarURL),
			Password:      passwordHash,
			Status:        normalizeSystemUserStatus(command.Status),
			RoleId:        roles[0].ID,
			DeptId:        fallbackInt64(command.DeptID, 1),
			PostId:        fallbackInt64(command.PostID, 1),
			LastLoginTime: now,
		}
		user.CreateTime = now
		user.UpdateTime = now
		if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
			return err
		}
		if err := replaceUserRoles(ctx, tx, user.ID, roles); err != nil {
			return err
		}
		item, err := r.toDomainSystemUserWithTx(ctx, tx, user)
		if err != nil {
			return err
		}
		created = item
		return nil
	})
	return created, err
}

func (r *Repository) UpdateSystemUser(ctx context.Context, command domain.UpsertSystemUserCommand) (domain.SystemUser, error) {
	var updated domain.SystemUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user po.User
		if err := tx.WithContext(ctx).Where("id = ?", command.ID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		if domain.IsProtectedSystemUserName(user.Username) || domain.IsProtectedSystemUserName(command.Username) {
			return domain.ErrProtectedSystemUser
		}
		roles, err := systemRolesByIDs(ctx, tx, command.RoleIDs)
		if err != nil {
			return err
		}
		updates := map[string]any{
			"user_name":   normalize(command.Username),
			"email":       normalize(command.Email),
			"phone":       strings.TrimSpace(command.Phone),
			"img":         strings.TrimSpace(command.AvatarURL),
			"status":      normalizeSystemUserStatus(command.Status),
			"dept_id":     fallbackInt64(command.DeptID, user.DeptId),
			"post_id":     fallbackInt64(command.PostID, user.PostId),
			"update_time": time.Now(),
		}
		if len(roles) > 0 {
			updates["role_id"] = roles[0].ID
		}
		if err := tx.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
			return err
		}
		if len(roles) > 0 {
			if err := replaceUserRoles(ctx, tx, user.ID, roles); err != nil {
				return err
			}
		}
		if err := tx.WithContext(ctx).Where("id = ?", command.ID).First(&user).Error; err != nil {
			return err
		}
		item, err := r.toDomainSystemUserWithTx(ctx, tx, user)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	return updated, err
}

func (r *Repository) DeleteSystemUser(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user po.User
		if err := tx.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		if domain.IsProtectedSystemUserName(user.Username) {
			return domain.ErrProtectedSystemUser
		}
		if err := tx.WithContext(ctx).Where("user_id = ?", id).Delete(&po.UserRole{}).Error; err != nil {
			return err
		}
		res := tx.WithContext(ctx).Where("id = ?", id).Delete(&po.User{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrInvalidAdminUserID
		}
		return nil
	})
}

func (r *Repository) ResetSystemUserPassword(ctx context.Context, id int64, passwordHash string) (domain.SystemUser, error) {
	var user po.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		if domain.IsProtectedSystemUserName(user.Username) {
			return domain.ErrProtectedSystemUser
		}
		return tx.WithContext(ctx).Model(&user).Updates(map[string]any{
			"password":    passwordHash,
			"update_time": time.Now(),
		}).Error
	})
	if err != nil {
		return domain.SystemUser{}, err
	}
	return r.toDomainSystemUser(ctx, user)
}

func (r *Repository) AssignSystemUserRoles(ctx context.Context, userID int64, roleIDs []int64) (domain.SystemUser, error) {
	var updated domain.SystemUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user po.User
		if err := tx.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidAdminUserID
			}
			return err
		}
		if domain.IsProtectedSystemUserName(user.Username) {
			return domain.ErrProtectedSystemUser
		}
		roles, err := systemRolesByIDs(ctx, tx, roleIDs)
		if err != nil {
			return err
		}
		if len(roles) == 0 {
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
		item, err := r.toDomainSystemUserWithTx(ctx, tx, user)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	return updated, err
}

func (r *Repository) ListSystemRoles(ctx context.Context, query string, status string, page int32, pageSize int32) (domain.SystemRoleList, error) {
	limit, offset := pageToLimitOffset(page, pageSize)
	db := r.db.WithContext(ctx).Model(&po.Role{})
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(name) LIKE ? OR LOWER(key) LIKE ?", like, like)
	}
	status = strings.TrimSpace(status)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.SystemRoleList{}, err
	}
	var roles []po.Role
	if err := db.Order("sort ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&roles).Error; err != nil {
		return domain.SystemRoleList{}, err
	}
	items := make([]domain.SystemRole, 0, len(roles))
	for _, role := range roles {
		item, err := r.toDomainSystemRole(ctx, role)
		if err != nil {
			return domain.SystemRoleList{}, err
		}
		items = append(items, item)
	}
	return domain.SystemRoleList{Items: items, Total: total}, nil
}

func (r *Repository) CreateSystemRole(ctx context.Context, command domain.UpsertSystemRoleCommand) (domain.SystemRole, error) {
	if domain.IsProtectedSystemRoleKey(command.Key) {
		return domain.SystemRole{}, domain.ErrProtectedSystemRole
	}
	role := po.Role{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		applySystemRoleCommand(&role, command)
		if err := ensureUniqueSystemRoleKey(ctx, tx, role.Key, 0); err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(&role).Error
	})
	if err != nil {
		return domain.SystemRole{}, err
	}
	return r.toDomainSystemRole(ctx, role)
}

func (r *Repository) UpdateSystemRole(ctx context.Context, command domain.UpsertSystemRoleCommand) (domain.SystemRole, error) {
	var role po.Role
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("id = ?", command.ID).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemRole
			}
			return err
		}
		if domain.IsProtectedSystemRoleKey(role.Key) || domain.IsProtectedSystemRoleKey(command.Key) {
			return domain.ErrProtectedSystemRole
		}
		previousKey := normalize(role.Key)
		applySystemRoleCommand(&role, command)
		if err := ensureUniqueSystemRoleKey(ctx, tx, role.Key, role.ID); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Save(&role).Error; err != nil {
			return err
		}
		currentKey := normalize(role.Key)
		if previousKey != "" && previousKey != currentKey {
			if err := deleteRolePoliciesByKey(ctx, tx, previousKey); err != nil {
				return err
			}
			menuIDs, err := roleMenuIDs(ctx, tx, role.ID)
			if err != nil {
				return err
			}
			return replaceRoleSystemPolicies(ctx, tx, role, menuIDs)
		}
		return nil
	})
	if err != nil {
		return domain.SystemRole{}, err
	}
	return r.toDomainSystemRole(ctx, role)
}

func (r *Repository) DeleteSystemRole(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role po.Role
		if err := tx.WithContext(ctx).Where("id = ?", id).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemRole
			}
			return err
		}
		if domain.IsProtectedSystemRoleKey(role.Key) {
			return domain.ErrProtectedSystemRole
		}
		var userRoleCount int64
		if err := tx.WithContext(ctx).Model(&po.UserRole{}).Where("role_id = ?", id).Count(&userRoleCount).Error; err != nil {
			return err
		}
		if userRoleCount > 0 {
			return domain.ErrSystemRoleHasUsers
		}
		var primaryUserCount int64
		if err := tx.WithContext(ctx).Model(&po.User{}).Where("role_id = ?", id).Count(&primaryUserCount).Error; err != nil {
			return err
		}
		if primaryUserCount > 0 {
			return domain.ErrSystemRoleHasUsers
		}
		if err := tx.WithContext(ctx).Where("role_id = ?", id).Delete(&po.RoleMenu{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("role_id = ?", id).Delete(&po.RoleDept{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("role_id = ?", id).Delete(&po.UserRole{}).Error; err != nil {
			return err
		}
		if err := deleteRolePoliciesByKey(ctx, tx, role.Key); err != nil {
			return err
		}
		res := tx.WithContext(ctx).Where("id = ?", id).Delete(&po.Role{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrInvalidSystemRole
		}
		return nil
	})
}

func (r *Repository) AssignSystemRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) (domain.SystemRole, error) {
	var role po.Role
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("id = ?", roleID).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemRole
			}
			return err
		}
		if domain.IsProtectedSystemRoleKey(role.Key) {
			return domain.ErrProtectedSystemRole
		}
		if err := replaceRoleMenus(ctx, tx, roleID, menuIDs); err != nil {
			return err
		}
		return replaceRoleSystemPolicies(ctx, tx, role, menuIDs)
	})
	if err != nil {
		return domain.SystemRole{}, err
	}
	return r.toDomainSystemRole(ctx, role)
}

func (r *Repository) ListSystemMenus(ctx context.Context, query string, status string) (domain.SystemMenuList, error) {
	db := r.db.WithContext(ctx).Model(&po.Menu{})
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(name) LIKE ? OR LOWER(title) LIKE ? OR LOWER(permission) LIKE ?", like, like, like)
	}
	status = strings.TrimSpace(status)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var menus []po.Menu
	if err := db.Order("sort ASC, id ASC").Find(&menus).Error; err != nil {
		return domain.SystemMenuList{}, err
	}
	items := make([]domain.SystemMenu, 0, len(menus))
	for _, menu := range menus {
		items = append(items, toDomainSystemMenu(menu))
	}
	return domain.SystemMenuList{Items: items, Total: int64(len(items))}, nil
}

func (r *Repository) ListCurrentSystemMenus(ctx context.Context, userID int64) (domain.SystemMenuList, error) {
	if userID <= 0 {
		return domain.SystemMenuList{}, domain.ErrInvalidAdminUserID
	}
	var menus []po.Menu
	if err := r.db.WithContext(ctx).
		Table("sys_menu").
		Select("DISTINCT sys_menu.*").
		Joins("JOIN sys_role_menu ON sys_role_menu.menu_id = sys_menu.id").
		Joins("JOIN sys_user_role ON sys_user_role.role_id = sys_role_menu.role_id").
		Joins("JOIN sys_role ON sys_role.id = sys_user_role.role_id").
		Where("sys_user_role.user_id = ?", userID).
		Where("sys_role.status = '' OR sys_role.status = ?", "1").
		Where("sys_menu.status = '' OR sys_menu.status = ?", "0").
		Order("sys_menu.sort ASC, sys_menu.id ASC").
		Find(&menus).Error; err != nil {
		return domain.SystemMenuList{}, err
	}
	menus, err := r.withParentSystemMenus(ctx, menus)
	if err != nil {
		return domain.SystemMenuList{}, err
	}
	items := make([]domain.SystemMenu, 0, len(menus))
	for _, menu := range menus {
		items = append(items, toDomainSystemMenu(menu))
	}
	return domain.SystemMenuList{Items: items, Total: int64(len(items))}, nil
}

func (r *Repository) withParentSystemMenus(ctx context.Context, menus []po.Menu) ([]po.Menu, error) {
	byID := make(map[int64]po.Menu, len(menus))
	pendingParents := make(map[int64]struct{})
	for _, menu := range menus {
		byID[menu.ID] = menu
		if menu.ParentId > 0 {
			pendingParents[menu.ParentId] = struct{}{}
		}
	}
	for len(pendingParents) > 0 {
		ids := make([]int64, 0, len(pendingParents))
		for id := range pendingParents {
			if _, ok := byID[id]; !ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			break
		}
		pendingParents = make(map[int64]struct{})
		var parents []po.Menu
		if err := r.db.WithContext(ctx).
			Where("id IN ?", ids).
			Where("status = '' OR status = ?", "0").
			Find(&parents).Error; err != nil {
			return nil, err
		}
		for _, parent := range parents {
			byID[parent.ID] = parent
			if parent.ParentId > 0 {
				pendingParents[parent.ParentId] = struct{}{}
			}
		}
	}
	out := make([]po.Menu, 0, len(byID))
	for _, menu := range byID {
		out = append(out, menu)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort == out[j].Sort {
			return out[i].ID < out[j].ID
		}
		return out[i].Sort < out[j].Sort
	})
	return out, nil
}

func (r *Repository) CreateSystemMenu(ctx context.Context, command domain.UpsertSystemMenuCommand) (domain.SystemMenu, error) {
	menu := po.Menu{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateSystemMenuParent(ctx, tx, 0, command.ParentID); err != nil {
			return err
		}
		applySystemMenuCommand(&menu, command)
		if err := ensureUniqueSystemMenuName(ctx, tx, menu.Name, 0); err != nil {
			return err
		}
		return tx.WithContext(ctx).Create(&menu).Error
	})
	if err != nil {
		return domain.SystemMenu{}, err
	}
	return toDomainSystemMenu(menu), nil
}

func (r *Repository) UpdateSystemMenu(ctx context.Context, command domain.UpsertSystemMenuCommand) (domain.SystemMenu, error) {
	var menu po.Menu
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("id = ?", command.ID).First(&menu).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemMenu
			}
			return err
		}
		previousPermission := strings.TrimSpace(menu.Permission)
		if err := validateSystemMenuParent(ctx, tx, command.ID, command.ParentID); err != nil {
			return err
		}
		applySystemMenuCommand(&menu, command)
		if err := ensureUniqueSystemMenuName(ctx, tx, menu.Name, menu.ID); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Save(&menu).Error; err != nil {
			return err
		}
		return syncRolePoliciesForMenuPermissionChange(ctx, tx, menu.ID, previousPermission, menu.Permission)
	})
	if err != nil {
		return domain.SystemMenu{}, err
	}
	return toDomainSystemMenu(menu), nil
}

func (r *Repository) DeleteSystemMenu(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var menu po.Menu
		if err := tx.WithContext(ctx).Where("id = ?", id).First(&menu).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemMenu
			}
			return err
		}
		var childCount int64
		if err := tx.WithContext(ctx).Model(&po.Menu{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return domain.ErrSystemMenuHasChildren
		}
		roleIDs, err := roleIDsByMenuID(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("menu_id = ?", id).Delete(&po.RoleMenu{}).Error; err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if err := pruneRolePolicyIfPermissionUnassigned(ctx, tx, roleID, menu.Permission); err != nil {
				return err
			}
		}
		if err := tx.WithContext(ctx).Where("menu_id = ?", id).Delete(&po.MenuApiRule{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("menu_id = ?", id).Delete(&po.MenuParam{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("menu_id = ?", id).Delete(&po.MenuButton{}).Error; err != nil {
			return err
		}
		res := tx.WithContext(ctx).Where("id = ?", id).Delete(&po.Menu{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrInvalidSystemMenu
		}
		return nil
	})
}

func (r *Repository) ListSystemDepts(ctx context.Context, query string, status int32) (domain.SystemDeptList, error) {
	db := r.db.WithContext(ctx).Model(&po.Dept{})
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(name) LIKE ? OR LOWER(leader) LIKE ? OR phone LIKE ?", like, like, like)
	}
	if status > 0 {
		db = db.Where("status = ?", status)
	}
	var depts []po.Dept
	if err := db.Order("sort ASC, id ASC").Find(&depts).Error; err != nil {
		return domain.SystemDeptList{}, err
	}
	items := make([]domain.SystemDept, 0, len(depts))
	for _, dept := range depts {
		items = append(items, toDomainSystemDept(dept))
	}
	return domain.SystemDeptList{Items: items, Total: int64(len(items))}, nil
}

func (r *Repository) CreateSystemDept(ctx context.Context, command domain.UpsertSystemDeptCommand) (domain.SystemDept, error) {
	dept := po.Dept{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateSystemDeptParent(ctx, tx, 0, command.ParentID); err != nil {
			return err
		}
		applySystemDeptCommand(&dept, command)
		if err := ensureUniqueSystemDeptName(ctx, tx, dept.ParentId, dept.Name, 0); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Create(&dept).Error; err != nil {
			return err
		}
		return r.syncDeptPath(ctx, tx, &dept)
	})
	if err != nil {
		return domain.SystemDept{}, err
	}
	return toDomainSystemDept(dept), nil
}

func (r *Repository) UpdateSystemDept(ctx context.Context, command domain.UpsertSystemDeptCommand) (domain.SystemDept, error) {
	var dept po.Dept
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("id = ?", command.ID).First(&dept).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemDept
			}
			return err
		}
		if err := validateSystemDeptParent(ctx, tx, command.ID, command.ParentID); err != nil {
			return err
		}
		applySystemDeptCommand(&dept, command)
		if err := ensureUniqueSystemDeptName(ctx, tx, dept.ParentId, dept.Name, dept.ID); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Save(&dept).Error; err != nil {
			return err
		}
		if err := r.syncDeptPath(ctx, tx, &dept); err != nil {
			return err
		}
		return r.syncDeptSubtreePaths(ctx, tx, dept.ID)
	})
	if err != nil {
		return domain.SystemDept{}, err
	}
	return toDomainSystemDept(dept), nil
}

func (r *Repository) DeleteSystemDept(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dept po.Dept
		if err := tx.WithContext(ctx).Where("id = ?", id).First(&dept).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrInvalidSystemDept
			}
			return err
		}
		var childCount int64
		if err := tx.WithContext(ctx).Model(&po.Dept{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
			return err
		}
		if childCount > 0 {
			return domain.ErrSystemDeptHasChildren
		}
		var userCount int64
		if err := tx.WithContext(ctx).Model(&po.User{}).Where("dept_id = ?", id).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount > 0 {
			return domain.ErrSystemDeptHasUsers
		}
		res := tx.WithContext(ctx).Where("id = ?", id).Delete(&po.Dept{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrInvalidSystemDept
		}
		return nil
	})
}

func (r *Repository) toDomainSystemUser(ctx context.Context, user po.User) (domain.SystemUser, error) {
	return r.toDomainSystemUserWithTx(ctx, r.db, user)
}

func (r *Repository) toDomainSystemUserWithTx(ctx context.Context, tx *gorm.DB, user po.User) (domain.SystemUser, error) {
	roleIDs, roleKeys, err := roleIDsAndKeysByUserID(ctx, tx, user.ID)
	if err != nil {
		return domain.SystemUser{}, err
	}
	return domain.SystemUser{
		ID:         user.ID,
		Username:   user.Username,
		Nickname:   user.Username,
		Email:      user.Email,
		Phone:      user.Phone,
		AvatarURL:  user.Img,
		Status:     int32(user.Status),
		LockedFlag: int32(user.LockedFlag),
		RoleID:     user.RoleId,
		DeptID:     user.DeptId,
		PostID:     user.PostId,
		RoleIDs:    roleIDs,
		Roles:      roleKeys,
		CreatedAt:  timeMillis(user.CreateTime),
		UpdatedAt:  timeMillis(user.UpdateTime),
	}, nil
}

func (r *Repository) toDomainSystemRole(ctx context.Context, role po.Role) (domain.SystemRole, error) {
	menuIDs, err := roleMenuIDs(ctx, r.db, role.ID)
	if err != nil {
		return domain.SystemRole{}, err
	}
	permissions, err := r.PermissionsByRoleKeys(ctx, []string{role.Key})
	if err != nil {
		return domain.SystemRole{}, err
	}
	return domain.SystemRole{
		ID:          role.ID,
		Name:        role.Name,
		Key:         role.Key,
		Status:      role.Status,
		Sort:        int32(role.Sort),
		Admin:       role.Admin,
		DataScope:   role.DataScope,
		Remark:      role.Remark,
		MenuIDs:     menuIDs,
		Permissions: permissions,
	}, nil
}

func applySystemRoleCommand(role *po.Role, command domain.UpsertSystemRoleCommand) {
	role.Name = strings.TrimSpace(command.Name)
	role.Key = normalize(command.Key)
	role.Status = defaultString(command.Status, "1")
	role.Sort = int(command.Sort)
	role.Admin = command.Admin
	role.DataScope = defaultString(command.DataScope, "1")
	role.Remark = strings.TrimSpace(command.Remark)
	role.UpdateTime = time.Now()
	if role.CreateTime.IsZero() {
		role.CreateTime = role.UpdateTime
	}
}

func applySystemMenuCommand(menu *po.Menu, command domain.UpsertSystemMenuCommand) {
	menu.ParentId = command.ParentID
	menu.Name = strings.TrimSpace(command.Name)
	menu.Title = strings.TrimSpace(command.Title)
	menu.Icon = strings.TrimSpace(command.Icon)
	menu.Path = strings.TrimSpace(command.Path)
	menu.Paths = menu.Path
	menu.Component = strings.TrimSpace(command.Component)
	menu.Type = defaultString(command.Type, "C")
	menu.Permission = strings.TrimSpace(command.Permission)
	menu.Status = defaultString(command.Status, "0")
	menu.Visible = defaultString(command.Visible, "0")
	menu.IsHide = defaultString(command.IsHide, "0")
	menu.Sort = int(command.Sort)
	menu.Remark = strings.TrimSpace(command.Remark)
	menu.UpdateTime = time.Now()
	if menu.CreateTime.IsZero() {
		menu.CreateTime = menu.UpdateTime
	}
}

func applySystemDeptCommand(dept *po.Dept, command domain.UpsertSystemDeptCommand) {
	dept.ParentId = command.ParentID
	dept.Name = strings.TrimSpace(command.Name)
	dept.Sort = int64(command.Sort)
	dept.Leader = strings.TrimSpace(command.Leader)
	dept.Phone = strings.TrimSpace(command.Phone)
	dept.Email = strings.TrimSpace(command.Email)
	dept.Status = normalizeSystemDeptStatus(command.Status)
	dept.UpdateTime = time.Now()
	if dept.CreateTime.IsZero() {
		dept.CreateTime = dept.UpdateTime
	}
}

func ensureUniqueSystemRoleKey(ctx context.Context, tx *gorm.DB, key string, excludeID int64) error {
	key = normalize(key)
	if key == "" {
		return domain.ErrInvalidSystemRole
	}
	db := tx.WithContext(ctx).Model(&po.Role{}).Where("LOWER(key) = ?", key)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrSystemRoleExists
	}
	return nil
}

func ensureUniqueSystemMenuName(ctx context.Context, tx *gorm.DB, name string, excludeID int64) error {
	name = normalize(name)
	if name == "" {
		return domain.ErrInvalidSystemMenu
	}
	db := tx.WithContext(ctx).Model(&po.Menu{}).Where("LOWER(name) = ?", name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrSystemMenuExists
	}
	return nil
}

func ensureUniqueSystemDeptName(ctx context.Context, tx *gorm.DB, parentID int64, name string, excludeID int64) error {
	name = normalize(name)
	if name == "" {
		return domain.ErrInvalidSystemDept
	}
	db := tx.WithContext(ctx).Model(&po.Dept{}).Where("parent_id = ? AND LOWER(name) = ?", parentID, name)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return domain.ErrSystemDeptExists
	}
	return nil
}

func validateSystemMenuParent(ctx context.Context, tx *gorm.DB, currentID int64, parentID int64) error {
	return validateSystemTreeParent(ctx, tx, &po.Menu{}, currentID, parentID, domain.ErrSystemMenuParentNotFound, domain.ErrSystemMenuInvalidParent)
}

func validateSystemDeptParent(ctx context.Context, tx *gorm.DB, currentID int64, parentID int64) error {
	return validateSystemTreeParent(ctx, tx, &po.Dept{}, currentID, parentID, domain.ErrSystemDeptParentNotFound, domain.ErrSystemDeptInvalidParent)
}

func validateSystemTreeParent(ctx context.Context, tx *gorm.DB, model any, currentID int64, parentID int64, notFound error, invalid error) error {
	if parentID <= 0 {
		return nil
	}
	ancestorID := parentID
	seen := map[int64]struct{}{}
	for ancestorID > 0 {
		if currentID > 0 && ancestorID == currentID {
			return invalid
		}
		if _, ok := seen[ancestorID]; ok {
			return invalid
		}
		seen[ancestorID] = struct{}{}
		var node struct {
			ID       int64
			ParentId int64
		}
		if err := tx.WithContext(ctx).Model(model).Select("id", "parent_id").Where("id = ?", ancestorID).First(&node).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if ancestorID == parentID {
					return notFound
				}
				return invalid
			}
			return err
		}
		ancestorID = node.ParentId
	}
	return nil
}

func (r *Repository) syncDeptPath(ctx context.Context, tx *gorm.DB, dept *po.Dept) error {
	path := "/" + strconv.FormatInt(dept.ID, 10)
	if dept.ParentId > 0 {
		var parent po.Dept
		if err := tx.WithContext(ctx).Where("id = ?", dept.ParentId).First(&parent).Error; err == nil && parent.Path != "" {
			path = strings.TrimRight(parent.Path, "/") + "/" + strconv.FormatInt(dept.ID, 10)
		}
	}
	dept.Path = path
	return tx.WithContext(ctx).Model(dept).Update("path", path).Error
}

func (r *Repository) syncDeptSubtreePaths(ctx context.Context, tx *gorm.DB, parentID int64) error {
	return r.syncDeptSubtreePathsSeen(ctx, tx, parentID, map[int64]struct{}{parentID: struct{}{}})
}

func (r *Repository) syncDeptSubtreePathsSeen(ctx context.Context, tx *gorm.DB, parentID int64, seen map[int64]struct{}) error {
	var children []po.Dept
	if err := tx.WithContext(ctx).Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return err
	}
	for i := range children {
		child := &children[i]
		if _, ok := seen[child.ID]; ok {
			return domain.ErrSystemDeptInvalidParent
		}
		seen[child.ID] = struct{}{}
		if err := r.syncDeptPath(ctx, tx, child); err != nil {
			return err
		}
		if err := r.syncDeptSubtreePathsSeen(ctx, tx, child.ID, seen); err != nil {
			return err
		}
		delete(seen, child.ID)
	}
	return nil
}

func toDomainSystemMenu(menu po.Menu) domain.SystemMenu {
	return domain.SystemMenu{
		ID:         menu.ID,
		ParentID:   menu.ParentId,
		Name:       menu.Name,
		Title:      menu.Title,
		Icon:       menu.Icon,
		Path:       menu.Path,
		Component:  menu.Component,
		Type:       menu.Type,
		Permission: menu.Permission,
		Status:     menu.Status,
		Visible:    menu.Visible,
		IsHide:     menu.IsHide,
		Sort:       int32(menu.Sort),
		Remark:     menu.Remark,
	}
}

func toDomainSystemDept(dept po.Dept) domain.SystemDept {
	return domain.SystemDept{
		ID:        dept.ID,
		ParentID:  dept.ParentId,
		Path:      dept.Path,
		Name:      dept.Name,
		Sort:      int32(dept.Sort),
		Leader:    dept.Leader,
		Phone:     dept.Phone,
		Email:     dept.Email,
		Status:    int32(dept.Status),
		CreatedAt: timeMillis(dept.CreateTime),
		UpdatedAt: timeMillis(dept.UpdateTime),
	}
}

func systemRolesByIDs(ctx context.Context, tx *gorm.DB, roleIDs []int64) ([]po.Role, error) {
	roleIDs = uniqueInt64s(roleIDs)
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var found []po.Role
	if err := tx.WithContext(ctx).Where("id IN ?", roleIDs).Find(&found).Error; err != nil {
		return nil, err
	}
	byID := make(map[int64]po.Role, len(found))
	for _, role := range found {
		byID[role.ID] = role
	}
	ordered := make([]po.Role, 0, len(roleIDs))
	for _, id := range roleIDs {
		role, ok := byID[id]
		if !ok {
			return nil, domain.ErrInvalidRoleKeys
		}
		ordered = append(ordered, role)
	}
	return ordered, nil
}

func defaultSystemRole(ctx context.Context, tx *gorm.DB) (po.Role, error) {
	var role po.Role
	if err := tx.WithContext(ctx).Where("key = ?", "user").First(&role).Error; err != nil {
		return po.Role{}, err
	}
	return role, nil
}

func roleIDsAndKeysByUserID(ctx context.Context, tx *gorm.DB, userID int64) ([]int64, []string, error) {
	var roles []po.Role
	err := tx.WithContext(ctx).
		Table("sys_role").
		Select("sys_role.*").
		Joins("JOIN sys_user_role ON sys_user_role.role_id = sys_role.id").
		Where("sys_user_role.user_id = ?", userID).
		Order("sys_role.sort ASC, sys_role.id ASC").
		Find(&roles).Error
	if err != nil {
		return nil, nil, err
	}
	ids := make([]int64, 0, len(roles))
	keys := make([]string, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
		keys = append(keys, role.Key)
	}
	return ids, keys, nil
}

func roleMenuIDs(ctx context.Context, tx *gorm.DB, roleID int64) ([]int64, error) {
	var ids []int64
	err := tx.WithContext(ctx).Model(&po.RoleMenu{}).Where("role_id = ?", roleID).Order("menu_id ASC").Pluck("menu_id", &ids).Error
	return ids, err
}

func roleIDsByMenuID(ctx context.Context, tx *gorm.DB, menuID int64) ([]int64, error) {
	var ids []int64
	err := tx.WithContext(ctx).Model(&po.RoleMenu{}).Where("menu_id = ?", menuID).Order("role_id ASC").Pluck("role_id", &ids).Error
	return ids, err
}

func replaceRoleMenus(ctx context.Context, tx *gorm.DB, roleID int64, menuIDs []int64) error {
	if err := tx.WithContext(ctx).Where("role_id = ?", roleID).Delete(&po.RoleMenu{}).Error; err != nil {
		return err
	}
	for _, menuID := range uniqueInt64s(menuIDs) {
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&po.RoleMenu{RoleID: roleID, MenuID: menuID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func syncRolePoliciesForMenuPermissionChange(ctx context.Context, tx *gorm.DB, menuID int64, previousPermission string, nextPermission string) error {
	previousPermission = strings.TrimSpace(previousPermission)
	nextPermission = strings.TrimSpace(nextPermission)
	if previousPermission == nextPermission {
		return nil
	}
	roleIDs, err := roleIDsByMenuID(ctx, tx, menuID)
	if err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if err := pruneRolePolicyIfPermissionUnassigned(ctx, tx, roleID, previousPermission); err != nil {
			return err
		}
		if err := ensureRolePolicyForPermission(ctx, tx, roleID, nextPermission); err != nil {
			return err
		}
	}
	return nil
}

func ensureRolePolicyForPermission(ctx context.Context, tx *gorm.DB, roleID int64, permission string) error {
	resource, action, ok := splitPermission(permission)
	if !ok {
		return nil
	}
	var role po.Role
	if err := tx.WithContext(ctx).Where("id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	roleKey := normalize(role.Key)
	if roleKey == "" {
		return nil
	}
	rule := policy(roleKey, resource, action)
	return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rule).Error
}

func pruneRolePolicyIfPermissionUnassigned(ctx context.Context, tx *gorm.DB, roleID int64, permission string) error {
	resource, action, ok := splitPermission(permission)
	if !ok {
		return nil
	}
	var role po.Role
	if err := tx.WithContext(ctx).Where("id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	roleKey := normalize(role.Key)
	if roleKey == "" {
		return nil
	}
	var count int64
	if err := tx.WithContext(ctx).
		Table("sys_role_menu").
		Joins("JOIN sys_menu ON sys_menu.id = sys_role_menu.menu_id").
		Where("sys_role_menu.role_id = ? AND sys_menu.permission = ?", roleID, strings.TrimSpace(permission)).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.WithContext(ctx).
		Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?", "p", roleKey, resource, action).
		Delete(&po.CasbinRule{}).Error
}

func deleteRolePoliciesByKey(ctx context.Context, tx *gorm.DB, roleKey string) error {
	roleKey = normalize(roleKey)
	if roleKey == "" {
		return nil
	}
	return tx.WithContext(ctx).
		Where("ptype = ? AND v0 = ?", "p", roleKey).
		Or("ptype = ? AND (v0 = ? OR v1 = ?)", "g", roleKey, roleKey).
		Delete(&po.CasbinRule{}).Error
}

func replaceRoleSystemPolicies(ctx context.Context, tx *gorm.DB, role po.Role, menuIDs []int64) error {
	roleKey := normalize(role.Key)
	if roleKey == "" {
		return domain.ErrInvalidSystemRole
	}
	managedPermissions, err := menuManagedPermissions(ctx, tx)
	if err != nil {
		return err
	}
	for _, permission := range managedPermissions {
		resource, action, ok := splitPermission(permission)
		if !ok {
			continue
		}
		if err := tx.WithContext(ctx).
			Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ?", "p", roleKey, resource, action).
			Delete(&po.CasbinRule{}).Error; err != nil {
			return err
		}
	}
	if len(menuIDs) == 0 {
		return nil
	}
	var menus []po.Menu
	if err := tx.WithContext(ctx).Where("id IN ?", uniqueInt64s(menuIDs)).Find(&menus).Error; err != nil {
		return err
	}
	for _, menu := range menus {
		resource, action, ok := splitPermission(menu.Permission)
		if !ok {
			continue
		}
		rule := policy(roleKey, resource, action)
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rule).Error; err != nil {
			return err
		}
	}
	return nil
}

func menuManagedPermissions(ctx context.Context, tx *gorm.DB) ([]string, error) {
	var raw []string
	if err := tx.WithContext(ctx).
		Model(&po.Menu{}).
		Where("permission <> ''").
		Pluck("permission", &raw).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, permission := range raw {
		resource, action, ok := splitPermission(permission)
		if !ok {
			continue
		}
		key := resource + ":" + action
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out, nil
}

func splitPermission(permission string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(permission), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func pageToLimitOffset(page int32, pageSize int32) (int32, int32) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return pageSize, (page - 1) * pageSize
}

func normalizeSystemUserStatus(status int32) int {
	if status <= 0 {
		return 1
	}
	return int(status)
}

func normalizeSystemDeptStatus(status int32) int {
	if status <= 0 {
		return 1
	}
	return int(status)
}

func fallbackInt64(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func uniqueInt64s(values []int64) []int64 {
	out := make([]int64, 0, len(values))
	seen := map[int64]struct{}{}
	for _, value := range values {
		if value <= 0 {
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

func timeMillis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func systemPermission(action domain.Action) string {
	return fmt.Sprintf("%s:%s", domain.ResourceSystem, action)
}

func governancePermission(action domain.Action) string {
	return fmt.Sprintf("%s:%s", domain.ResourceGovernance, action)
}

type systemMenuButtonSeed struct {
	Name       string
	Title      string
	Permission string
	SortOffset int
}

type seededSystemMenuButton struct {
	ID         int64
	Permission string
}

func seedDefaultSystemManagement(ctx context.Context, tx *gorm.DB) error {
	governanceRoot := po.Menu{
		Name:       "governance",
		Title:      "社区管理",
		Icon:       "ri/community-line",
		Path:       "/governance",
		Paths:      "/governance",
		Type:       "M",
		Permission: "",
		ParentId:   0,
		Status:     "0",
		Visible:    "0",
		IsHide:     "0",
		Component:  "",
		Sort:       1000,
		Remark:     "bootstrap governance root",
	}
	if err := upsertSystemMenuSeed(ctx, tx, &governanceRoot); err != nil {
		return err
	}
	governanceChildren := []po.Menu{
		{Name: "governance.users", Title: "用户管理", Icon: "ri/user-search-line", Path: "/governance/users", Paths: "/governance/users", Type: "C", Permission: governancePermission(domain.ActionListUsers), ParentId: governanceRoot.ID, Status: "0", Visible: "0", IsHide: "0", Component: "governance/users/index", Sort: 1010, Remark: "bootstrap governance users"},
		{Name: "governance.articles", Title: "文章管理", Icon: "ri/article-line", Path: "/governance/articles", Paths: "/governance/articles", Type: "C", Permission: governancePermission(domain.ActionListArticles), ParentId: governanceRoot.ID, Status: "0", Visible: "0", IsHide: "0", Component: "governance/articles/index", Sort: 1020, Remark: "bootstrap governance articles"},
		{Name: "governance.topics", Title: "话题管理", Icon: "ri/discuss-line", Path: "/governance/topics", Paths: "/governance/topics", Type: "C", Permission: governancePermission(domain.ActionListTopics), ParentId: governanceRoot.ID, Status: "0", Visible: "0", IsHide: "0", Component: "governance/topics/index", Sort: 1030, Remark: "bootstrap governance topics"},
		{Name: "governance.comments", Title: "评论管理", Icon: "ri/chat-3-line", Path: "/governance/comments", Paths: "/governance/comments", Type: "C", Permission: governancePermission(domain.ActionListComments), ParentId: governanceRoot.ID, Status: "0", Visible: "0", IsHide: "0", Component: "governance/comments/index", Sort: 1040, Remark: "bootstrap governance comments"},
		{Name: "governance.reports", Title: "举报管理", Icon: "ri/file-search-line", Path: "/governance/reports", Paths: "/governance/reports", Type: "C", Permission: governancePermission(domain.ActionListReports), ParentId: governanceRoot.ID, Status: "0", Visible: "0", IsHide: "0", Component: "governance/reports/index", Sort: 1050, Remark: "bootstrap governance reports"},
	}
	governanceButtonSeeds := map[string][]systemMenuButtonSeed{
		"governance.users": {
			{Name: "query", Title: "查询", Permission: governancePermission(domain.ActionListUsers), SortOffset: 1},
			{Name: "mute", Title: "禁言", Permission: governancePermission(domain.ActionMuteUser), SortOffset: 2},
			{Name: "unmute", Title: "解禁", Permission: governancePermission(domain.ActionUnmuteUser), SortOffset: 3},
		},
		"governance.articles": {
			{Name: "query", Title: "查询", Permission: governancePermission(domain.ActionListArticles), SortOffset: 1},
			{Name: "hide", Title: "隐藏", Permission: governancePermission(domain.ActionHideArticle), SortOffset: 2},
			{Name: "archive", Title: "归档", Permission: governancePermission(domain.ActionArchiveArticle), SortOffset: 3},
		},
		"governance.topics": {
			{Name: "query", Title: "查询", Permission: governancePermission(domain.ActionListTopics), SortOffset: 1},
			{Name: "hide", Title: "隐藏", Permission: governancePermission(domain.ActionHideTopic), SortOffset: 2},
			{Name: "archive", Title: "归档", Permission: governancePermission(domain.ActionArchiveTopic), SortOffset: 3},
		},
		"governance.comments": {
			{Name: "query", Title: "查询", Permission: governancePermission(domain.ActionListComments), SortOffset: 1},
			{Name: "hide", Title: "隐藏", Permission: governancePermission(domain.ActionHideComment), SortOffset: 2},
		},
		"governance.reports": {
			{Name: "query", Title: "查询", Permission: governancePermission(domain.ActionListReports), SortOffset: 1},
			{Name: "audit", Title: "审核", Permission: governancePermission(domain.ActionAuditReport), SortOffset: 2},
			{Name: "mute", Title: "禁言", Permission: governancePermission(domain.ActionMuteUser), SortOffset: 3},
			{Name: "unmute", Title: "解禁", Permission: governancePermission(domain.ActionUnmuteUser), SortOffset: 4},
		},
	}
	moderatorPermissions := map[string]struct{}{
		governancePermission(domain.ActionListReports):  {},
		governancePermission(domain.ActionAuditReport):  {},
		governancePermission(domain.ActionListArticles): {},
		governancePermission(domain.ActionHideArticle):  {},
		governancePermission(domain.ActionListTopics):   {},
		governancePermission(domain.ActionHideTopic):    {},
		governancePermission(domain.ActionListComments): {},
		governancePermission(domain.ActionHideComment):  {},
	}
	adminMenuIDs := []int64{governanceRoot.ID}
	moderatorMenuIDs := []int64{}
	for i := range governanceChildren {
		if err := upsertSystemMenuSeed(ctx, tx, &governanceChildren[i]); err != nil {
			return err
		}
		buttons, err := upsertSystemMenuButtonSeeds(ctx, tx, governanceChildren[i], governanceButtonSeeds[governanceChildren[i].Name])
		if err != nil {
			return err
		}
		adminMenuIDs = append(adminMenuIDs, governanceChildren[i].ID)
		for _, button := range buttons {
			adminMenuIDs = append(adminMenuIDs, button.ID)
		}
		if governanceChildren[i].Name != "governance.users" {
			moderatorMenuIDs = append(moderatorMenuIDs, governanceChildren[i].ID)
			for _, button := range buttons {
				if _, ok := moderatorPermissions[button.Permission]; ok {
					moderatorMenuIDs = append(moderatorMenuIDs, button.ID)
				}
			}
		}
	}

	root := po.Menu{
		Name:       "system",
		Title:      "系统管理",
		Icon:       "ri/settings-3-line",
		Path:       "/system",
		Paths:      "/system",
		Type:       "M",
		Permission: systemPermission(domain.ActionListSystemMenus),
		ParentId:   0,
		Status:     "0",
		Visible:    "0",
		IsHide:     "0",
		Component:  "",
		Sort:       1400,
		Remark:     "bootstrap system root",
	}
	if err := upsertSystemMenuSeed(ctx, tx, &root); err != nil {
		return err
	}
	children := []po.Menu{
		{Name: "system.user", Title: "用户管理", Icon: "ri/user-3-line", Path: "/system/user", Paths: "/system/user", Type: "C", Permission: systemPermission(domain.ActionListSystemUsers), ParentId: root.ID, Status: "0", Visible: "0", IsHide: "0", Component: "system/user/index", Sort: 1410, Remark: "bootstrap system user"},
		{Name: "system.role", Title: "角色管理", Icon: "ri/admin-line", Path: "/system/role", Paths: "/system/role", Type: "C", Permission: systemPermission(domain.ActionListSystemRoles), ParentId: root.ID, Status: "0", Visible: "0", IsHide: "0", Component: "system/role/index", Sort: 1420, Remark: "bootstrap system role"},
		{Name: "system.menu", Title: "菜单管理", Icon: "ri/menu-2-line", Path: "/system/menu", Paths: "/system/menu", Type: "C", Permission: systemPermission(domain.ActionListSystemMenus), ParentId: root.ID, Status: "0", Visible: "0", IsHide: "0", Component: "system/menu/index", Sort: 1430, Remark: "bootstrap system menu"},
		{Name: "system.dept", Title: "部门管理", Icon: "ri/git-branch-line", Path: "/system/dept", Paths: "/system/dept", Type: "C", Permission: systemPermission(domain.ActionListSystemDepts), ParentId: root.ID, Status: "0", Visible: "0", IsHide: "0", Component: "system/dept/index", Sort: 1440, Remark: "bootstrap system dept"},
		{Name: "system.login-log", Title: "登录日志", Icon: "ri/login-box-line", Path: "/monitor/logs/login", Paths: "/monitor/logs/login", Type: "C", Permission: systemPermission(domain.ActionListLoginLogs), ParentId: root.ID, Status: "0", Visible: "0", IsHide: "0", Component: "monitor/logs/login/index", Sort: 1450, Remark: "bootstrap login log"},
		{Name: "system.operation-log", Title: "操作日志", Icon: "ri/file-list-3-line", Path: "/monitor/logs/operation", Paths: "/monitor/logs/operation", Type: "C", Permission: systemPermission(domain.ActionListOperationLogs), ParentId: root.ID, Status: "0", Visible: "0", IsHide: "0", Component: "monitor/logs/operation/index", Sort: 1460, Remark: "bootstrap operation log"},
	}
	systemButtonSeeds := map[string][]systemMenuButtonSeed{
		"system.user": {
			{Name: "query", Title: "查询", Permission: systemPermission(domain.ActionListSystemUsers), SortOffset: 1},
			{Name: "create", Title: "新增", Permission: systemPermission(domain.ActionCreateSystemUser), SortOffset: 2},
			{Name: "update", Title: "修改", Permission: systemPermission(domain.ActionUpdateSystemUser), SortOffset: 3},
			{Name: "delete", Title: "删除", Permission: systemPermission(domain.ActionDeleteSystemUser), SortOffset: 4},
			{Name: "reset-password", Title: "重置密码", Permission: systemPermission(domain.ActionResetSystemUserPass), SortOffset: 5},
			{Name: "assign-roles", Title: "分配角色", Permission: systemPermission(domain.ActionAssignSystemUserRoles), SortOffset: 6},
		},
		"system.role": {
			{Name: "query", Title: "查询", Permission: systemPermission(domain.ActionListSystemRoles), SortOffset: 1},
			{Name: "create", Title: "新增", Permission: systemPermission(domain.ActionCreateSystemRole), SortOffset: 2},
			{Name: "update", Title: "修改", Permission: systemPermission(domain.ActionUpdateSystemRole), SortOffset: 3},
			{Name: "delete", Title: "删除", Permission: systemPermission(domain.ActionDeleteSystemRole), SortOffset: 4},
			{Name: "assign-menus", Title: "分配权限", Permission: systemPermission(domain.ActionAssignSystemRoleMenus), SortOffset: 5},
		},
		"system.menu": {
			{Name: "query", Title: "查询", Permission: systemPermission(domain.ActionListSystemMenus), SortOffset: 1},
			{Name: "create", Title: "新增", Permission: systemPermission(domain.ActionCreateSystemMenu), SortOffset: 2},
			{Name: "update", Title: "修改", Permission: systemPermission(domain.ActionUpdateSystemMenu), SortOffset: 3},
			{Name: "delete", Title: "删除", Permission: systemPermission(domain.ActionDeleteSystemMenu), SortOffset: 4},
		},
		"system.dept": {
			{Name: "query", Title: "查询", Permission: systemPermission(domain.ActionListSystemDepts), SortOffset: 1},
			{Name: "create", Title: "新增", Permission: systemPermission(domain.ActionCreateSystemDept), SortOffset: 2},
			{Name: "update", Title: "修改", Permission: systemPermission(domain.ActionUpdateSystemDept), SortOffset: 3},
			{Name: "delete", Title: "删除", Permission: systemPermission(domain.ActionDeleteSystemDept), SortOffset: 4},
		},
		"system.login-log": {
			{Name: "query", Title: "查询", Permission: systemPermission(domain.ActionListLoginLogs), SortOffset: 1},
		},
		"system.operation-log": {
			{Name: "query", Title: "查询", Permission: systemPermission(domain.ActionListOperationLogs), SortOffset: 1},
		},
	}
	menuIDs := append(adminMenuIDs, root.ID)
	for i := range children {
		if err := upsertSystemMenuSeed(ctx, tx, &children[i]); err != nil {
			return err
		}
		menuIDs = append(menuIDs, children[i].ID)
		buttons, err := upsertSystemMenuButtonSeeds(ctx, tx, children[i], systemButtonSeeds[children[i].Name])
		if err != nil {
			return err
		}
		for _, button := range buttons {
			menuIDs = append(menuIDs, button.ID)
		}
	}
	var roles []po.Role
	if err := tx.WithContext(ctx).Where("key IN ?", []string{"admin", "superadmin"}).Find(&roles).Error; err != nil {
		return err
	}
	for _, role := range roles {
		if err := replaceRoleMenus(ctx, tx, role.ID, menuIDs); err != nil {
			return err
		}
		if err := replaceRoleSystemPolicies(ctx, tx, role, menuIDs); err != nil {
			return err
		}
	}
	var moderator po.Role
	res := tx.WithContext(ctx).Where("key = ?", "moderator").First(&moderator)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return res.Error
	}
	if res.Error == nil {
		moderatorMenuIDs = append([]int64{governanceRoot.ID}, moderatorMenuIDs...)
		if err := replaceRoleMenus(ctx, tx, moderator.ID, moderatorMenuIDs); err != nil {
			return err
		}
		if err := replaceRoleSystemPolicies(ctx, tx, moderator, moderatorMenuIDs); err != nil {
			return err
		}
	}
	return nil
}

func upsertSystemMenuSeed(ctx context.Context, tx *gorm.DB, seed *po.Menu) error {
	seed.UpdateTime = time.Now()
	if seed.CreateTime.IsZero() {
		seed.CreateTime = seed.UpdateTime
	}
	var existing po.Menu
	res := tx.WithContext(ctx).Where("name = ?", seed.Name).Find(&existing)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		if err := tx.WithContext(ctx).Create(seed).Error; err != nil {
			return err
		}
		return nil
	}
	updates := map[string]any{
		"title":       seed.Title,
		"icon":        seed.Icon,
		"path":        seed.Path,
		"paths":       seed.Paths,
		"type":        seed.Type,
		"permission":  seed.Permission,
		"parent_id":   seed.ParentId,
		"status":      seed.Status,
		"visible":     seed.Visible,
		"is_hide":     seed.IsHide,
		"component":   seed.Component,
		"sort":        seed.Sort,
		"remark":      seed.Remark,
		"update_time": seed.UpdateTime,
	}
	if err := tx.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
		return err
	}
	seed.ID = existing.ID
	return nil
}

func upsertSystemMenuButtonSeeds(ctx context.Context, tx *gorm.DB, parent po.Menu, seeds []systemMenuButtonSeed) ([]seededSystemMenuButton, error) {
	out := make([]seededSystemMenuButton, 0, len(seeds))
	for i, seed := range seeds {
		permission := strings.TrimSpace(seed.Permission)
		if permission == "" {
			continue
		}
		sortOffset := seed.SortOffset
		if sortOffset <= 0 {
			sortOffset = i + 1
		}
		button := po.Menu{
			Name:       parent.Name + "." + seed.Name,
			Title:      seed.Title,
			Icon:       "",
			Path:       "",
			Paths:      "",
			Type:       "F",
			Permission: permission,
			ParentId:   parent.ID,
			Status:     "0",
			Visible:    "1",
			IsHide:     "1",
			Component:  "",
			Sort:       parent.Sort + sortOffset,
			Remark:     "bootstrap " + parent.Name + " " + seed.Name + " button",
		}
		if err := upsertSystemMenuSeed(ctx, tx, &button); err != nil {
			return nil, err
		}
		out = append(out, seededSystemMenuButton{ID: button.ID, Permission: permission})
	}
	return out, nil
}
