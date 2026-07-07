package po

import "time"

type AdminUserProfile struct {
	UserID      int64     `json:"userId" gorm:"column:user_id;primaryKey"`
	DisplayName string    `json:"displayName" gorm:"column:display_name;type:varchar(80);comment:显示昵称"`
	AvatarURL   string    `json:"avatarUrl" gorm:"column:avatar_url;type:varchar(512);comment:头像地址"`
	Bio         string    `json:"bio" gorm:"column:bio;type:varchar(500);comment:个人简介"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (AdminUserProfile) TableName() string {
	return "sys_user_profile"
}
