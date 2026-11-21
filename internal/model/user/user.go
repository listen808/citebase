package user

// User 用户模型 (对应 aw_user 表)
type User struct {
	ID          int    `gorm:"column:user_id;primaryKey" json:"id"`
	Username    string `gorm:"column:username;size:100;not null;uniqueIndex:uk_username" json:"username"`
	Password    string `gorm:"column:password;size:60;not null" json:"-"` // bcrypt 哈希 60 字符
	GivenName   string `gorm:"column:given_name;size:50" json:"given_name"`
	Mobile      string `gorm:"column:mobile;size:13" json:"mobile"`
	Phone       string `gorm:"column:phone;size:13" json:"phone"`
	Email       string `gorm:"column:email;size:50" json:"email"`
	Department  string `gorm:"column:department;size:50" json:"department"`
	Position    string `gorm:"column:position;size:50" json:"position"`
	Location    string `gorm:"column:location;size:50" json:"location"`
	IM          string `gorm:"column:im;size:50" json:"im"`
	LastIP      string `gorm:"column:last_ip;size:15" json:"last_ip"`
	LastTime    int    `gorm:"column:last_time" json:"last_time"`
	RoleID      int    `gorm:"column:role_id" json:"role_id"`
	IsForbidden int    `gorm:"column:is_forbidden" json:"is_forbidden"`
	IsDelete    int    `gorm:"column:is_delete" json:"is_delete"`
	CreateTime  int    `gorm:"column:create_time" json:"create_time"` // Unix 时间戳
	UpdateTime  int    `gorm:"column:update_time" json:"update_time"` // Unix 时间戳
	// Preferences 是个性化偏好（外观 × 材质 × 透明变体）校验后的 JSON 文本，见 preferences.go。
	// **json:"-"**：全仓有 4 处把 *usermodel.User 直接 response.Success 发出去
	// （GetUserByID、GetUsersForImport、UpdateUser、CreateUser），其中管理员看的用户列表
	// 会把这一列一起带出去 —— 偏好属于个人设置，跟着列表走等于把别人的界面配置暴露给管理员。
	// 自服务面另走 UserInfo.Preferences。
	Preferences string `gorm:"column:preferences;type:text" json:"-"`
}

// TableName 指定表名
func (User) TableName() string {
	return "aw_user"
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username   string `json:"username" binding:"required,min=3,max=50"`
	Password   string `json:"password" binding:"required,min=8,max=60"`
	GivenName  string `json:"given_name"`
	Email      string `json:"email" binding:"omitempty,email"`
	Mobile     string `json:"mobile"`
	Department string `json:"department"`
	Position   string `json:"position"`
	RoleID     int    `json:"role_id"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	GivenName  string `json:"given_name"`
	Mobile     string `json:"mobile"`
	Phone      string `json:"phone"`
	Email      string `json:"email" binding:"omitempty,email"`
	Department string `json:"department"`
	Position   string `json:"position"`
	Location   string `json:"location"`
	IM         string `json:"im"`
	RoleID     int    `json:"role_id"`
}

// UserResponse 用户响应 (隐藏敏感字段)
type UserResponse struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	GivenName   string `json:"given_name"`
	Mobile      string `json:"mobile"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Department  string `json:"department"`
	Position    string `json:"position"`
	Location    string `json:"location"`
	IM          string `json:"im"`
	Avatar      string `json:"avatar,omitempty"` // P1 修复：对齐前端 User.avatar（数据库无此列，留空字符串，前端按 ApAvatar fallback 渲染）
	LastIP      string `json:"last_ip"`
	LastTime    int    `json:"last_time"`
	RoleID      int    `json:"role_id"`
	RoleName    string `json:"role_name,omitempty"` // 角色名称
	IsForbidden int    `json:"is_forbidden"`
	IsDelete    int    `json:"is_delete,omitempty"`
	CreateTime  int    `json:"create_time"`
	UpdateTime  int    `json:"update_time"`
	// 关注统计（可选填充）
	Following   int  `json:"following,omitempty"`
	Followers   int  `json:"followers,omitempty"`
	IsFollowing bool `json:"is_following,omitempty"` // 当前用户是否已关注
}

// ToResponse 转换为响应对象
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		GivenName:   u.GivenName,
		Mobile:      u.Mobile,
		Phone:       u.Phone,
		Email:       u.Email,
		Department:  u.Department,
		Position:    u.Position,
		Location:    u.Location,
		IM:          u.IM,
		Avatar:      "", // P1 修复：数据库无 avatar 列，统一返回空字符串
		LastIP:      u.LastIP,
		LastTime:    u.LastTime,
		RoleID:      u.RoleID,
		IsForbidden: u.IsForbidden,
		IsDelete:    u.IsDelete,
		CreateTime:  u.CreateTime,
		UpdateTime:  u.UpdateTime,
	}
}

// ToResponseWithRole 转换为响应对象（带角色名称）
func (u *User) ToResponseWithRole(roleName string) UserResponse {
	resp := u.ToResponse()
	resp.RoleName = roleName
	return resp
}
