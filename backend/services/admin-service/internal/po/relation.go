package po

import "time"

type UserRole struct {
	UserID    int64     `json:"userId" gorm:"column:user_id;primaryKey"`
	RoleID    int64     `json:"roleId" gorm:"column:role_id;primaryKey"`
	CreatedAt time.Time `json:"createdAt" gorm:"column:created_at"`
}

func (UserRole) TableName() string {
	return "sys_user_role"
}

type RoleMenu struct {
	RoleID int64 `json:"roleId" gorm:"column:role_id;primaryKey"`
	MenuID int64 `json:"menuId" gorm:"column:menu_id;primaryKey"`
}

func (RoleMenu) TableName() string {
	return "sys_role_menu"
}

type RoleDept struct {
	RoleID int64 `json:"roleId" gorm:"column:role_id;primaryKey"`
	DeptID int64 `json:"deptId" gorm:"column:dept_id;primaryKey"`
}

func (RoleDept) TableName() string {
	return "sys_role_dept"
}

type MenuApiRule struct {
	MenuID int64 `json:"menuId" gorm:"column:menu_id;primaryKey"`
	ApiID  int64 `json:"apiId" gorm:"column:api_id;primaryKey"`
}

func (MenuApiRule) TableName() string {
	return "sys_menu_api_rule"
}

func Models() []any {
	return []any{
		&Api{},
		&Badge{},
		&CasbinRule{},
		&Dept{},
		&DictType{},
		&DictData{},
		&EmailLog{},
		&ForbiddenWord{},
		&LoginLog{},
		&Level{},
		&Link{},
		&Menu{},
		&MenuParam{},
		&MenuButton{},
		&SysOperaLog{},
		&Post{},
		&Role{},
		&SiteSetting{},
		&User{},
		&UserToken{},
		&UserRole{},
		&RoleMenu{},
		&RoleDept{},
		&Task{},
		&MenuApiRule{},
	}
}
