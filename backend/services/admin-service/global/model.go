package global

import "time"

type ModelTime struct {
	CreateTime time.Time `json:"createTime" gorm:"column:create_time"`
	UpdateTime time.Time `json:"updateTime" gorm:"column:update_time"`
}

type OperateBy struct {
	CreateBy int64 `json:"createBy" gorm:"column:create_by"`
	UpdateBy int64 `json:"updateBy" gorm:"column:update_by"`
}
