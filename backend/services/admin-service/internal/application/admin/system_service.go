package admin

import (
	"context"
	"strings"

	domain "admin/internal/domain/admin"
)

func (s *Service) ListSystemUsers(ctx context.Context, actor domain.Actor, query string, status int32, page int32, pageSize int32) (domain.SystemUserList, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionListSystemUsers); err != nil {
		return domain.SystemUserList{}, err
	}
	return s.system.ListSystemUsers(ctx, query, status, page, pageSize)
}

func (s *Service) CreateSystemUser(ctx context.Context, actor domain.Actor, command domain.UpsertSystemUserCommand) (domain.SystemUser, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionCreateSystemUser); err != nil {
		return domain.SystemUser{}, err
	}
	if strings.TrimSpace(command.Username) == "" {
		return domain.SystemUser{}, domain.ErrInvalidSystemUser
	}
	if err := validatePasswordPolicy(command.Password); err != nil {
		return domain.SystemUser{}, err
	}
	passwordHash, err := s.hasher.Hash(command.Password)
	if err != nil {
		return domain.SystemUser{}, err
	}
	command.ID = 0
	return s.system.CreateSystemUser(ctx, command, passwordHash)
}

func (s *Service) UpdateSystemUser(ctx context.Context, actor domain.Actor, command domain.UpsertSystemUserCommand) (domain.SystemUser, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionUpdateSystemUser); err != nil {
		return domain.SystemUser{}, err
	}
	if command.ID <= 0 || strings.TrimSpace(command.Username) == "" {
		return domain.SystemUser{}, domain.ErrInvalidSystemUser
	}
	return s.system.UpdateSystemUser(ctx, command)
}

func (s *Service) DeleteSystemUser(ctx context.Context, actor domain.Actor, id int64) error {
	if err := s.authorizeSystem(ctx, actor, domain.ActionDeleteSystemUser); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidAdminUserID
	}
	return s.system.DeleteSystemUser(ctx, id)
}

func (s *Service) ResetSystemUserPassword(ctx context.Context, actor domain.Actor, id int64, password string) (domain.SystemUser, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionResetSystemUserPass); err != nil {
		return domain.SystemUser{}, err
	}
	if id <= 0 {
		return domain.SystemUser{}, domain.ErrInvalidAdminUserID
	}
	if err := validatePasswordPolicy(password); err != nil {
		return domain.SystemUser{}, err
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return domain.SystemUser{}, err
	}
	return s.system.ResetSystemUserPassword(ctx, id, passwordHash)
}

func (s *Service) AssignSystemUserRoles(ctx context.Context, actor domain.Actor, userID int64, roleIDs []int64) (domain.SystemUser, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionAssignSystemUserRoles); err != nil {
		return domain.SystemUser{}, err
	}
	if userID <= 0 {
		return domain.SystemUser{}, domain.ErrInvalidAdminUserID
	}
	if len(roleIDs) == 0 {
		return domain.SystemUser{}, domain.ErrInvalidRoleKeys
	}
	return s.system.AssignSystemUserRoles(ctx, userID, roleIDs)
}

func (s *Service) ListSystemRoles(ctx context.Context, actor domain.Actor, query string, status string, page int32, pageSize int32) (domain.SystemRoleList, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionListSystemRoles); err != nil {
		return domain.SystemRoleList{}, err
	}
	return s.system.ListSystemRoles(ctx, query, status, page, pageSize)
}

func (s *Service) GetSystemRole(ctx context.Context, actor domain.Actor, id int64) (domain.SystemRole, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionListSystemRoles); err != nil {
		return domain.SystemRole{}, err
	}
	if id <= 0 {
		return domain.SystemRole{}, domain.ErrInvalidSystemRole
	}
	return s.system.GetSystemRole(ctx, id)
}

func (s *Service) CreateSystemRole(ctx context.Context, actor domain.Actor, command domain.UpsertSystemRoleCommand) (domain.SystemRole, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionCreateSystemRole); err != nil {
		return domain.SystemRole{}, err
	}
	if strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Key) == "" {
		return domain.SystemRole{}, domain.ErrInvalidSystemRole
	}
	command.ID = 0
	return s.system.CreateSystemRole(ctx, command)
}

func (s *Service) UpdateSystemRole(ctx context.Context, actor domain.Actor, command domain.UpsertSystemRoleCommand) (domain.SystemRole, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionUpdateSystemRole); err != nil {
		return domain.SystemRole{}, err
	}
	if command.ID <= 0 || strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Key) == "" {
		return domain.SystemRole{}, domain.ErrInvalidSystemRole
	}
	role, err := s.system.UpdateSystemRole(ctx, command)
	if err != nil {
		return domain.SystemRole{}, err
	}
	if err := s.auth.Reload(ctx); err != nil {
		return domain.SystemRole{}, err
	}
	return role, nil
}

func (s *Service) DeleteSystemRole(ctx context.Context, actor domain.Actor, id int64) error {
	if err := s.authorizeSystem(ctx, actor, domain.ActionDeleteSystemRole); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidSystemRole
	}
	if err := s.system.DeleteSystemRole(ctx, id); err != nil {
		return err
	}
	return s.auth.Reload(ctx)
}

func (s *Service) AssignSystemRoleMenus(ctx context.Context, actor domain.Actor, roleID int64, menuIDs []int64) (domain.SystemRole, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionAssignSystemRoleMenus); err != nil {
		return domain.SystemRole{}, err
	}
	if roleID <= 0 {
		return domain.SystemRole{}, domain.ErrInvalidSystemRole
	}
	role, err := s.system.AssignSystemRoleMenus(ctx, roleID, menuIDs)
	if err != nil {
		return domain.SystemRole{}, err
	}
	if err := s.auth.Reload(ctx); err != nil {
		return domain.SystemRole{}, err
	}
	return role, nil
}

func (s *Service) ListSystemMenus(ctx context.Context, actor domain.Actor, query string, status string) (domain.SystemMenuList, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionListSystemMenus); err != nil {
		return domain.SystemMenuList{}, err
	}
	return s.system.ListSystemMenus(ctx, query, status)
}

func (s *Service) ListCurrentSystemMenus(ctx context.Context, actor domain.Actor) (domain.SystemMenuList, error) {
	if err := actor.Validate(); err != nil {
		return domain.SystemMenuList{}, err
	}
	return s.system.ListCurrentSystemMenus(ctx, actor.ID)
}

func (s *Service) CreateSystemMenu(ctx context.Context, actor domain.Actor, command domain.UpsertSystemMenuCommand) (domain.SystemMenu, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionCreateSystemMenu); err != nil {
		return domain.SystemMenu{}, err
	}
	if strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Title) == "" {
		return domain.SystemMenu{}, domain.ErrInvalidSystemMenu
	}
	command.ID = 0
	return s.system.CreateSystemMenu(ctx, command)
}

func (s *Service) UpdateSystemMenu(ctx context.Context, actor domain.Actor, command domain.UpsertSystemMenuCommand) (domain.SystemMenu, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionUpdateSystemMenu); err != nil {
		return domain.SystemMenu{}, err
	}
	if command.ID <= 0 || strings.TrimSpace(command.Name) == "" || strings.TrimSpace(command.Title) == "" {
		return domain.SystemMenu{}, domain.ErrInvalidSystemMenu
	}
	menu, err := s.system.UpdateSystemMenu(ctx, command)
	if err != nil {
		return domain.SystemMenu{}, err
	}
	if err := s.auth.Reload(ctx); err != nil {
		return domain.SystemMenu{}, err
	}
	return menu, nil
}

func (s *Service) DeleteSystemMenu(ctx context.Context, actor domain.Actor, id int64) error {
	if err := s.authorizeSystem(ctx, actor, domain.ActionDeleteSystemMenu); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidSystemMenu
	}
	if err := s.system.DeleteSystemMenu(ctx, id); err != nil {
		return err
	}
	return s.auth.Reload(ctx)
}

func (s *Service) ListSystemDepts(ctx context.Context, actor domain.Actor, query string, status int32) (domain.SystemDeptList, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionListSystemDepts); err != nil {
		return domain.SystemDeptList{}, err
	}
	return s.system.ListSystemDepts(ctx, query, status)
}

func (s *Service) CreateSystemDept(ctx context.Context, actor domain.Actor, command domain.UpsertSystemDeptCommand) (domain.SystemDept, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionCreateSystemDept); err != nil {
		return domain.SystemDept{}, err
	}
	if strings.TrimSpace(command.Name) == "" {
		return domain.SystemDept{}, domain.ErrInvalidSystemDept
	}
	command.ID = 0
	return s.system.CreateSystemDept(ctx, command)
}

func (s *Service) UpdateSystemDept(ctx context.Context, actor domain.Actor, command domain.UpsertSystemDeptCommand) (domain.SystemDept, error) {
	if err := s.authorizeSystem(ctx, actor, domain.ActionUpdateSystemDept); err != nil {
		return domain.SystemDept{}, err
	}
	if command.ID <= 0 || strings.TrimSpace(command.Name) == "" {
		return domain.SystemDept{}, domain.ErrInvalidSystemDept
	}
	return s.system.UpdateSystemDept(ctx, command)
}

func (s *Service) DeleteSystemDept(ctx context.Context, actor domain.Actor, id int64) error {
	if err := s.authorizeSystem(ctx, actor, domain.ActionDeleteSystemDept); err != nil {
		return err
	}
	if id <= 0 {
		return domain.ErrInvalidSystemDept
	}
	return s.system.DeleteSystemDept(ctx, id)
}

func (s *Service) authorizeSystem(ctx context.Context, actor domain.Actor, action domain.Action) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	return s.auth.Authorize(ctx, actor, action)
}
