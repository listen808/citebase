package user

// ============================================================
// 角色与权限模型
// 对应表：aw_role / aw_privilege / aw_role_privilege
// ============================================================

// 系统角色常量（对应 data.sql 中预置的 3 个系统角色）
const (
	RoleTypeSystem = 1 // 系统角色
	RoleTypeCustom = 0 // 自定义角色
)

// Role 角色模型
type Role struct {
	ID          int    `gorm:"column:role_id;primaryKey" json:"role_id"`
	Name        string `gorm:"column:name;size:50" json:"name"`
	Type        int    `gorm:"column:type" json:"type"` // 0 自定义 1 系统
	Description string `gorm:"column:description;size:255" json:"description"`
	ListOrder   int    `gorm:"column:list_order" json:"list_order"`
	IsDelete    int    `gorm:"column:is_delete" json:"is_delete"`
	CreateTime  int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime  int    `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (Role) TableName() string {
	return "aw_role"
}

// Privilege 权限模型
type Privilege struct {
	ID         int    `gorm:"column:privilege_id;primaryKey" json:"id"`
	Name       string `gorm:"column:name;size:50" json:"name"`
	ParentID   int    `gorm:"column:parent_id" json:"parent_id"`
	Type       string `gorm:"column:type;size:20" json:"type"` // controller / menu
	Controller string `gorm:"column:controller;size:100" json:"controller"`
	Action     string `gorm:"column:action;size:100" json:"action"`
	Icon       string `gorm:"column:icon;size:100" json:"icon"`
	Target     string `gorm:"column:target;size:200" json:"target"`
	IsDisplay  int    `gorm:"column:is_display" json:"is_display"` // 0 不显示 1 显示
	IsMenu     int    `gorm:"column:is_menu" json:"is_menu"`
	Sequence   int    `gorm:"column:sequence" json:"sequence"`
	ListOrder  int    `gorm:"column:list_order" json:"list_order"`
	IsDelete   int    `gorm:"column:is_delete" json:"is_delete"`
	CreateTime int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime int    `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (Privilege) TableName() string {
	return "aw_privilege"
}

// RolePrivilege 角色权限关系模型
type RolePrivilege struct {
	ID          int `gorm:"column:role_privilege_id;primaryKey" json:"id"`
	RoleID      int `gorm:"column:role_id" json:"role_id"`
	PrivilegeID int `gorm:"column:privilege_id" json:"privilege_id"`
	CreateTime  int `gorm:"column:create_time" json:"create_time"`
	UpdateTime  int `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (RolePrivilege) TableName() string {
	return "aw_role_privilege"
}

// PrivilegeNode 权限树节点（用于前端渲染菜单）
type PrivilegeNode struct {
	ID         int              `json:"id"`
	Name       string           `json:"name"`
	ParentID   int              `json:"parent_id"`
	Type       string           `json:"type"`
	Controller string           `json:"controller"`
	Action     string           `json:"action"`
	Icon       string           `json:"icon"`
	IsDisplay  int              `json:"is_display"`
	Children   []*PrivilegeNode `json:"children,omitempty"`
}
