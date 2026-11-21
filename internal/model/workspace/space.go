package workspace

// Space 空间模型 (对应 aw_space 表)
type Space struct {
	ID            int    `gorm:"column:space_id;primaryKey" json:"id"`
	Name          string `gorm:"column:name;size:50;not null" json:"name"`
	Description   string `gorm:"column:description;size:500" json:"description"`
	Tags          string `gorm:"column:tags;size:255" json:"tags"`
	Status        int    `gorm:"column:status" json:"status"`                          // 0 关闭 1 开启
	VisitLevel    string `gorm:"column:visit_level;size:20" json:"visit_level"`        // 0 所有 1 登录 2 空间成员
	IsShare       int    `gorm:"column:is_share" json:"is_share"`                      // 是否允许分享：0否 1是
	IsExport      int    `gorm:"column:is_export" json:"is_export"`                    // 是否允许导出：0否 1是
	SharePassword string `gorm:"column:share_password;size:100" json:"share_password"` // 分享密码（bcrypt hash，空=无密码）
	CreatedBy     int    `gorm:"column:created_by" json:"created_by"`
	CreateUserID  int    `gorm:"column:create_user_id" json:"create_user_id"`
	EditUserID    int    `gorm:"column:edit_user_id" json:"edit_user_id"`
	IsDelete      int    `gorm:"column:is_delete" json:"is_delete"`
	Sequence      int    `gorm:"column:sequence" json:"sequence"`
	CreateTime    int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime    int    `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (Space) TableName() string {
	return "aw_space"
}

// visit_level 历史上混用两套编码（数字与英文），判定必须走下面的方法，不要直接比较字符串。
const (
	VisitLevelPublic     = "0" // 所有人可见（历史值 "public"）
	VisitLevelLoggedIn   = "1" // 登录用户可见（历史值 "login"）
	VisitLevelMemberOnly = "2" // 仅空间成员可见（历史值 "member"/"private"）
)

// IsPublic 是否所有人可见
func (s *Space) IsPublic() bool {
	return s.VisitLevel == "0" || s.VisitLevel == "public"
}

// IsLoggedInVisible 是否登录用户即可见（不含仅成员可见的级别）
func (s *Space) IsLoggedInVisible() bool {
	return s.VisitLevel == "1" || s.VisitLevel == "login"
}

// VisitLevelAliases 返回同一级别的所有历史写法，供 SQL 筛选使用
func VisitLevelAliases(level string) []string {
	switch level {
	case "0", "public":
		return []string{"0", "public"}
	case "1", "login":
		return []string{"1", "login"}
	case "2", "member", "private":
		return []string{"2", "member", "private"}
	}
	return []string{level}
}

// SpaceUser 空间成员模型 (对应 aw_space_user 表)
type SpaceUser struct {
	SpaceUserID int `gorm:"column:space_user_id;primaryKey;autoIncrement" json:"id"`
	UserID      int `gorm:"column:user_id;uniqueIndex:idx_user_space" json:"user_id"`
	SpaceID     int `gorm:"column:space_id;uniqueIndex:idx_user_space" json:"space_id"`
	Privilege   int `gorm:"column:privilege" json:"privilege"` // 0 浏览者 1 编辑者 2 管理员
	CreateTime  int `gorm:"column:create_time" json:"create_time"`
	UpdateTime  int `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (SpaceUser) TableName() string {
	return "aw_space_user"
}

// CreateSpaceRequest 创建空间请求
type CreateSpaceRequest struct {
	Name          string `json:"name" binding:"required,min=1,max=50"`
	Description   string `json:"description"`
	Tags          string `json:"tags"`
	VisitLevel    string `json:"visit_level"`    // public/private
	IsShare       *bool  `json:"is_share"`       // 可选，默认 true
	IsExport      *bool  `json:"is_export"`      // 可选，默认 true
	SharePassword string `json:"share_password"` // 可选，分享密码
}

// UpdateSpaceRequest 更新空间请求
type UpdateSpaceRequest struct {
	Name          string `json:"name" binding:"omitempty,min=1,max=50"`
	Description   string `json:"description"`
	Tags          string `json:"tags"`
	VisitLevel    string `json:"visit_level"`
	IsShare       *bool  `json:"is_share"`
	IsExport      *bool  `json:"is_export"`
	SharePassword string `json:"share_password"`
	Sequence      *int   `json:"sequence"`
}

// SortSpacesRequest 批量排序空间请求
type SortSpacesRequest struct {
	Orders []SpaceOrder `json:"orders" binding:"required"`
}

// SpaceOrder 单个空间排序项
type SpaceOrder struct {
	ID       int `json:"id" binding:"required"`
	Sequence int `json:"sequence"`
}

// SpaceResponse 空间响应
type SpaceResponse struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	VisitLevel  string `json:"visit_level"`
	IsShare     int    `json:"is_share"`
	IsExport    int    `json:"is_export"`
	HasPassword bool   `json:"has_password"`
	CreatedBy   int    `json:"created_by"`
	IsDelete    int    `json:"is_delete"`
	Sequence    int    `json:"sequence"`
	CreateTime  int    `json:"create_time"`
	UpdateTime  int    `json:"update_time"`
	MemberCount int64  `json:"member_count,omitempty"`
	DocCount    int64  `json:"doc_count,omitempty"`
	Privilege   int    `json:"privilege,omitempty"`
	CreatorID   int    `json:"creator_id,omitempty"`   // 创建者ID
	CreatorName string `json:"creator_name,omitempty"` // 创建者姓名
}

// ToResponse 转换为响应对象
func (s *Space) ToResponse() SpaceResponse {
	return SpaceResponse{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		Status:      s.Status,
		VisitLevel:  s.VisitLevel,
		IsShare:     s.IsShare,
		IsExport:    s.IsExport,
		HasPassword: s.SharePassword != "",
		CreatedBy:   s.CreatedBy,
		IsDelete:    s.IsDelete,
		Sequence:    s.Sequence,
		CreateTime:  s.CreateTime,
		UpdateTime:  s.UpdateTime,
	}
}

// AddSpaceUserRequest 添加空间成员请求
type AddSpaceUserRequest struct {
	UserID    int `json:"user_id" binding:"required"`
	Privilege int `json:"privilege"` // 0 浏览者 1 编辑者 2 管理员
}

// UpdateSpaceUserPrivilegeRequest 更新成员权限请求
type UpdateSpaceUserPrivilegeRequest struct {
	UserID    int `json:"user_id"`
	Privilege int `json:"privilege" binding:"required,oneof=0 1 2"`
}

// SpaceUserResponse 空间成员响应
type SpaceUserResponse struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	SpaceID    int    `json:"space_id"`
	Privilege  int    `json:"privilege"`
	Username   string `json:"username,omitempty"`
	GivenName  string `json:"given_name,omitempty"`
	Email      string `json:"email,omitempty"`
	Department string `json:"department,omitempty"`
	CreateTime int    `json:"create_time"`
	UpdateTime int    `json:"update_time"`
}
