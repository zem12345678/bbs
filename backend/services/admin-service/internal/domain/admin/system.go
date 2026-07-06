package admin

import (
	"errors"
	"strings"
)

const (
	ResourceSystem = "system"

	ActionListSystemUsers       Action = "list_system_users"
	ActionCreateSystemUser      Action = "create_system_user"
	ActionUpdateSystemUser      Action = "update_system_user"
	ActionDeleteSystemUser      Action = "delete_system_user"
	ActionResetSystemUserPass   Action = "reset_system_user_password"
	ActionAssignSystemUserRoles Action = "assign_system_user_roles"
	ActionListSystemRoles       Action = "list_system_roles"
	ActionCreateSystemRole      Action = "create_system_role"
	ActionUpdateSystemRole      Action = "update_system_role"
	ActionDeleteSystemRole      Action = "delete_system_role"
	ActionAssignSystemRoleMenus Action = "assign_system_role_menus"
	ActionListSystemMenus       Action = "list_system_menus"
	ActionCreateSystemMenu      Action = "create_system_menu"
	ActionUpdateSystemMenu      Action = "update_system_menu"
	ActionDeleteSystemMenu      Action = "delete_system_menu"
	ActionListSystemDepts       Action = "list_system_depts"
	ActionCreateSystemDept      Action = "create_system_dept"
	ActionUpdateSystemDept      Action = "update_system_dept"
	ActionDeleteSystemDept      Action = "delete_system_dept"
)

var (
	ErrInvalidSystemUser   = errors.New("invalid system user")
	ErrInvalidSystemRole   = errors.New("invalid system role")
	ErrInvalidSystemMenu   = errors.New("invalid system menu")
	ErrInvalidSystemDept   = errors.New("invalid system dept")
	ErrProtectedSystemRole = errors.New("内置管理员角色不能修改")
)

func IsProtectedSystemRoleKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "admin", "superadmin":
		return true
	default:
		return false
	}
}

func ResourceForAction(action Action) string {
	switch action {
	case ActionListSystemUsers,
		ActionCreateSystemUser,
		ActionUpdateSystemUser,
		ActionDeleteSystemUser,
		ActionResetSystemUserPass,
		ActionAssignSystemUserRoles,
		ActionListSystemRoles,
		ActionCreateSystemRole,
		ActionUpdateSystemRole,
		ActionDeleteSystemRole,
		ActionAssignSystemRoleMenus,
		ActionListSystemMenus,
		ActionCreateSystemMenu,
		ActionUpdateSystemMenu,
		ActionDeleteSystemMenu,
		ActionListSystemDepts,
		ActionCreateSystemDept,
		ActionUpdateSystemDept,
		ActionDeleteSystemDept,
		ActionListLoginLogs,
		ActionListOperationLogs:
		return ResourceSystem
	default:
		return ResourceGovernance
	}
}

type SystemUser struct {
	ID         int64
	Username   string
	Nickname   string
	Email      string
	Phone      string
	AvatarURL  string
	Status     int32
	LockedFlag int32
	RoleID     int64
	DeptID     int64
	PostID     int64
	RoleIDs    []int64
	Roles      []string
	CreatedAt  int64
	UpdatedAt  int64
}

type SystemUserList struct {
	Items []SystemUser
	Total int64
}

type UpsertSystemUserCommand struct {
	ID        int64
	Username  string
	Nickname  string
	Email     string
	Phone     string
	Password  string
	AvatarURL string
	Status    int32
	DeptID    int64
	PostID    int64
	RoleIDs   []int64
}

type SystemRole struct {
	ID          int64
	Name        string
	Key         string
	Status      string
	Sort        int32
	Admin       bool
	DataScope   string
	Remark      string
	MenuIDs     []int64
	Permissions []string
}

type SystemRoleList struct {
	Items []SystemRole
	Total int64
}

type UpsertSystemRoleCommand struct {
	ID        int64
	Name      string
	Key       string
	Status    string
	Sort      int32
	Admin     bool
	DataScope string
	Remark    string
}

type SystemMenu struct {
	ID         int64
	ParentID   int64
	Name       string
	Title      string
	Icon       string
	Path       string
	Component  string
	Type       string
	Permission string
	Status     string
	Visible    string
	IsHide     string
	Sort       int32
	Remark     string
	Children   []SystemMenu
}

type SystemMenuList struct {
	Items []SystemMenu
	Total int64
}

type UpsertSystemMenuCommand struct {
	ID         int64
	ParentID   int64
	Name       string
	Title      string
	Icon       string
	Path       string
	Component  string
	Type       string
	Permission string
	Status     string
	Visible    string
	IsHide     string
	Sort       int32
	Remark     string
}

type SystemDept struct {
	ID        int64
	ParentID  int64
	Path      string
	Name      string
	Sort      int32
	Leader    string
	Phone     string
	Email     string
	Status    int32
	CreatedAt int64
	UpdatedAt int64
}

type SystemDeptList struct {
	Items []SystemDept
	Total int64
}

type UpsertSystemDeptCommand struct {
	ID       int64
	ParentID int64
	Name     string
	Sort     int32
	Leader   string
	Phone    string
	Email    string
	Status   int32
}
