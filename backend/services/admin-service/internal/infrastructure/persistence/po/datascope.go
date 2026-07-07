package po

import (
	"admin/pkg/tools/text_color"
	"fmt"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"strconv"
)

type DataPermission struct {
	DataScope string
	UserId    int64
	DeptId    int64
	RoleId    int64
	Enable    bool
}

func (e *DataPermission) GetDataScope(tableName string, db *gorm.DB) (*gorm.DB, error) {

	if !e.Enable {
		usageStr := fmt.Sprintf("%s\n", `数据权限已经为您`+text_color.Green(`关闭`)+`，如需开启请参考配置文件字段说明`)
		fmt.Print(usageStr)
		return db, nil
	}
	user := new(User)
	role := new(Role)
	err := db.Find(user, e.UserId).Error
	if err != nil {
		return nil, errors.New("获取用户数据出错 msg:" + err.Error())
	}
	err = db.Find(role, user.RoleId).Error
	if err != nil {
		return nil, errors.New("获取用户数据出错 msg:" + err.Error())
	}
	if role.DataScope == "2" {
		db = db.Where(tableName+".create_by in (select sys_user.user_id from sys_role_dept left join sys_user on sys_user.dept_id=sys_role_dept.dept_id where sys_role_dept.role_id = ?)", user.RoleId)
	}
	if role.DataScope == "3" {
		db = db.Where(tableName+".create_by in (SELECT user_id from sys_user where dept_id = ? )", user.DeptId)
	}
	if role.DataScope == "4" {
		db = db.Where(tableName+".create_by in (SELECT user_id from sys_user where sys_user.dept_id in(select dept_id from sys_dept where dept_path like ? ))", "%"+strconv.Itoa(int(user.DeptId))+"%")
	}
	if role.DataScope == "5" || role.DataScope == "" {
		db = db.Where(tableName+".create_by = ?", e.UserId)
	}

	return db, nil
}
