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

type AnnouncementRead struct {
	UserID                int64     `json:"userId" gorm:"column:user_id;primaryKey"`
	AnnouncementID        string    `json:"announcementId" gorm:"column:announcement_id;type:varchar(64);primaryKey"`
	AnnouncementUpdatedAt int64     `json:"announcementUpdatedAt" gorm:"column:announcement_updated_at;not null"`
	ReadAt                time.Time `json:"readAt" gorm:"column:read_at;not null"`
}

func (AnnouncementRead) TableName() string {
	return "admin_announcement_read"
}

func (MenuApiRule) TableName() string {
	return "sys_menu_api_rule"
}

func Models() []any {
	return []any{
		&Ad{},
		&Api{},
		&Badge{},
		&Emoji{},
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
		&AdminUserProfile{},
		&UserToken{},
		&UserRole{},
		&RoleMenu{},
		&RoleDept{},
		&Task{},
		&MenuApiRule{},
		&AnnouncementRead{},
	}
}
