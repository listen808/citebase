package admin

// Email 邮件服务器配置（对应 aw_email 表）
type Email struct {
	ID                int    `gorm:"column:email_id;primaryKey" json:"id"`
	Name              string `gorm:"column:name;size:50;not null" json:"name"`
	SenderAddress     string `gorm:"column:sender_address;size:100;not null" json:"sender_address"`
	SenderName        string `gorm:"column:sender_name;size:50" json:"sender_name"`
	SenderTitlePrefix string `gorm:"column:sender_title_prefix;size:50" json:"sender_title_prefix"`
	Host              string `gorm:"column:host;size:100;not null" json:"host"`
	Port              string `gorm:"column:port;size:10;not null;default:25" json:"port"`
	Username          string `gorm:"column:username;size:100;not null" json:"username"`
	Password          string `gorm:"column:password;size:100;not null" json:"-"` // 不返回给前端
	IsSSL             int    `gorm:"column:is_ssl;default:0" json:"is_ssl"`
	IsUsed            int    `gorm:"column:is_used;default:0" json:"is_used"`
	CreateTime        int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime        int    `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (Email) TableName() string {
	return "aw_email"
}

// EmailResponse 邮件服务器响应（隐藏密码）
type EmailResponse struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	SenderAddress     string `json:"sender_address"`
	SenderName        string `json:"sender_name"`
	SenderTitlePrefix string `json:"sender_title_prefix"`
	Host              string `json:"host"`
	Port              string `json:"port"`
	Username          string `json:"username"`
	IsSSL             int    `json:"is_ssl"`
	IsUsed            int    `json:"is_used"`
	CreateTime        int    `json:"create_time"`
	UpdateTime        int    `json:"update_time"`
}

// ToResponse 转换为响应对象
func (e *Email) ToResponse() EmailResponse {
	return EmailResponse{
		ID:                e.ID,
		Name:              e.Name,
		SenderAddress:     e.SenderAddress,
		SenderName:        e.SenderName,
		SenderTitlePrefix: e.SenderTitlePrefix,
		Host:              e.Host,
		Port:              e.Port,
		Username:          e.Username,
		IsSSL:             e.IsSSL,
		IsUsed:            e.IsUsed,
		CreateTime:        e.CreateTime,
		UpdateTime:        e.UpdateTime,
	}
}

// CreateEmailRequest 创建邮件服务器请求
type CreateEmailRequest struct {
	Name              string `json:"name" binding:"required,min=1,max=50"`
	SenderAddress     string `json:"sender_address" binding:"required,email"`
	SenderName        string `json:"sender_name"`
	SenderTitlePrefix string `json:"sender_title_prefix"`
	Host              string `json:"host" binding:"required"`
	Port              string `json:"port" binding:"required"`
	Username          string `json:"username" binding:"required"`
	Password          string `json:"password" binding:"required"`
	IsSSL             int    `json:"is_ssl"`
}

// UpdateEmailRequest 更新邮件服务器请求
type UpdateEmailRequest struct {
	Name              string `json:"name" binding:"required,min=1,max=50"`
	SenderAddress     string `json:"sender_address" binding:"required,email"`
	SenderName        string `json:"sender_name"`
	SenderTitlePrefix string `json:"sender_title_prefix"`
	Host              string `json:"host" binding:"required"`
	Port              string `json:"port" binding:"required"`
	Username          string `json:"username" binding:"required"`
	Password          string `json:"password"` // 可选，不传则不更新密码
	IsSSL             int    `json:"is_ssl"`
}

// TestEmailRequest 测试邮件发送请求
type TestEmailRequest struct {
	Name              string `json:"name" binding:"required"`
	SenderAddress     string `json:"sender_address" binding:"required,email"`
	SenderName        string `json:"sender_name"`
	SenderTitlePrefix string `json:"sender_title_prefix"`
	Host              string `json:"host" binding:"required"`
	Port              string `json:"port" binding:"required"`
	Username          string `json:"username" binding:"required"`
	Password          string `json:"password" binding:"required"`
	IsSSL             int    `json:"is_ssl"`
	ToEmails          string `json:"to_emails" binding:"required"` // 多个用分号分隔
}

// ConfigKey 系统配置键名常量
const (
	ConfigKeyAutoFollowDoc   = "auto_follow_doc"
	ConfigKeyMainTitle       = "main_title"
	ConfigKeyMainDescription = "main_description"
)

// Config 系统配置（对应 aw_config 表）
type Config struct {
	ID         int    `gorm:"column:config_id;primaryKey" json:"id"`
	Key        string `gorm:"column:key;size:50;not null;uniqueIndex:uk_key" json:"key"`
	Value      string `gorm:"column:value;type:text" json:"value"`
	Label      string `gorm:"column:label;size:100" json:"label"`
	GroupKey   string `gorm:"column:group_key;size:50" json:"group_key"`
	Type       string `gorm:"column:type;size:20;default:'text'" json:"type"` // text/textarea/select/switch
	Options    string `gorm:"column:options;type:text" json:"options"`        // select 的选项，JSON 格式
	Sort       int    `gorm:"column:sort;default:0" json:"sort"`
	IsPrivate  int    `gorm:"column:is_private;default:0" json:"is_private"` // 私密配置不返回 value
	CreateTime int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime int    `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (Config) TableName() string {
	return "aw_config"
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	ID         int    `json:"id"`
	Key        string `json:"key"`
	Value      string `json:"value,omitempty"`
	Label      string `json:"label"`
	GroupKey   string `json:"group_key"`
	Type       string `json:"type"`
	Options    string `json:"options,omitempty"`
	Sort       int    `json:"sort"`
	IsPrivate  int    `json:"is_private,omitempty"`
	CreateTime int    `json:"create_time"`
	UpdateTime int    `json:"update_time"`
}

// ToResponse 转换为响应对象
func (c *Config) ToResponse() ConfigResponse {
	resp := ConfigResponse{
		ID:         c.ID,
		Key:        c.Key,
		Label:      c.Label,
		GroupKey:   c.GroupKey,
		Type:       c.Type,
		Options:    c.Options,
		Sort:       c.Sort,
		IsPrivate:  c.IsPrivate,
		CreateTime: c.CreateTime,
		UpdateTime: c.UpdateTime,
	}
	if c.IsPrivate == 0 {
		resp.Value = c.Value
	}
	return resp
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Value string `json:"value"`
}

// AuthSSO SSO 认证配置（对应 aw_login_auth 表）
type AuthSSO struct {
	ID             int    `gorm:"column:login_auth_id;primaryKey" json:"id"`
	Name           string `gorm:"column:name;size:50;not null" json:"name"`
	URL            string `gorm:"column:url;size:255;not null" json:"url"`
	UsernamePrefix string `gorm:"column:username_prefix;size:20;not null" json:"username_prefix"`
	ExtData        string `gorm:"column:ext_data;type:text" json:"ext_data"`
	IsUsed         int    `gorm:"column:is_used;default:0" json:"is_used"`
	CreateTime     int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime     int    `gorm:"column:update_time" json:"update_time"`
}

// TableName 指定表名
func (AuthSSO) TableName() string {
	return "aw_login_auth"
}

// AuthSSOResponse SSO 配置响应
type AuthSSOResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	UsernamePrefix string `json:"username_prefix"`
	ExtData        string `json:"ext_data"`
	IsUsed         int    `json:"is_used"`
	CreateTime     int    `json:"create_time"`
	UpdateTime     int    `json:"update_time"`
}

// ToResponse 转换为响应对象
func (a *AuthSSO) ToResponse() AuthSSOResponse {
	return AuthSSOResponse{
		ID:             a.ID,
		Name:           a.Name,
		URL:            a.URL,
		UsernamePrefix: a.UsernamePrefix,
		ExtData:        a.ExtData,
		IsUsed:         a.IsUsed,
		CreateTime:     a.CreateTime,
		UpdateTime:     a.UpdateTime,
	}
}

// CreateAuthSSORequest 创建 SSO 配置请求
type CreateAuthSSORequest struct {
	Name           string `json:"name" binding:"required,min=1,max=50"`
	URL            string `json:"url" binding:"required,url"`
	UsernamePrefix string `json:"username_prefix" binding:"required,min=1,max=20,alphanum"`
	ExtData        string `json:"ext_data"`
}

// UpdateAuthSSORequest 更新 SSO 配置请求
type UpdateAuthSSORequest struct {
	Name           string `json:"name" binding:"required,min=1,max=50"`
	URL            string `json:"url" binding:"required,url"`
	UsernamePrefix string `json:"username_prefix" binding:"required,min=1,max=20,alphanum"`
	ExtData        string `json:"ext_data"`
}

// HotDocument 热门文档
type HotDocument struct {
	DocumentID int    `json:"document_id"`
	Name       string `json:"name"`
	SpaceName  string `json:"space_name"`
	Count      int    `json:"count"` // 浏览/编辑次数
}

// DashboardStats 仪表盘统计数据
type DashboardStats struct {
	TotalUsers         int64           `json:"total_users"`
	ActiveUsers        int64           `json:"active_users"` // 今天登录用户数
	ForbiddenUsers     int64           `json:"forbidden_users"`
	TotalSpaces        int64           `json:"total_spaces"`
	TotalDocuments     int64           `json:"total_documents"`
	TodayDocuments     int64           `json:"today_documents"`
	IndexedDocuments   int64           `json:"indexed_documents"` // 已索引文档数
	IndexStatus        string          `json:"index_status"`      // 索引状态
	TopCreator         *UserStatItem   `json:"top_creator,omitempty"`
	TopEditor          *UserStatItem   `json:"top_editor,omitempty"`
	TopCollector       *UserStatItem   `json:"top_collector,omitempty"`
	TopFollower        *UserStatItem   `json:"top_follower,omitempty"`
	ActiveContributors []UserStatItem  `json:"active_contributors"` // 活跃贡献者列表（综合活跃度（活跃
	SpaceRank          []SpaceDocCount `json:"space_rank"`
	DocumentTrend      []DocCountItem  `json:"document_trend"`
	HotDocuments       []HotDocument   `json:"hot_documents"` // 热门文档 TOP10
}

// UserStatItem 用户统计项
type UserStatItem struct {
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	GivenName   string `json:"given_name,omitempty"`
	Count       int    `json:"count"`
	LastTime    int    `json:"last_time,omitempty"`
	IsForbidden int    `json:"is_forbidden,omitempty"`
	IsDelete    int    `json:"is_delete,omitempty"`
}

// SpaceDocCount 空间文档数量
type SpaceDocCount struct {
	SpaceID   int    `json:"space_id"`
	SpaceName string `json:"space_name"`
	Total     int    `json:"total"`
}

// DocCountItem 文档数量趋势
type DocCountItem struct {
	Date  string `gorm:"column:date" json:"date"`
	Count int64  `gorm:"column:count" json:"count"`
}

// ServerStatus 服务器状态
type ServerStatus struct {
	LocalIP        string  `json:"local_ip"`
	Hostname       string  `json:"hostname"`
	OS             string  `json:"os"`
	Platform       string  `json:"platform"`
	PlatformFamily string  `json:"platform_family"`
	Uptime         int64   `json:"uptime"` // 秒
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryPercent  float64 `json:"memory_percent"`
	MemoryUsed     uint64  `json:"memory_used"`  // bytes
	MemoryTotal    uint64  `json:"memory_total"` // bytes
	DiskPercent    float64 `json:"disk_percent"`
	DiskUsed       uint64  `json:"disk_used"`       // bytes
	DiskTotal      uint64  `json:"disk_total"`      // bytes
	NetworkPercent float64 `json:"network_percent"` // 网络带宽使用率（模拟）
	ErrorLogCount  int64   `json:"error_log_count"`
	// 构建元数据只在管理端 diagnostics 里出现，公开的 /health 只给版本号。
	// 放在这里而不是另开一个端点：这页本来就是"这台机器上跑的是什么"。
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
	// SystemStatusError 采集失败原因（磁盘等指标采集不到时非空）。
	// 旧实现把所有采集错误吞掉恒返回 nil，Linux 部署下磁盘使用率长期显示 0 且无人察觉。
	SystemStatusError string `json:"system_status_error,omitempty"`
}

// ServerTime 服务器时间
type ServerTime struct {
	ServerTime int64 `json:"server_time"`
	RunTime    int64 `json:"run_time"`
}

// InstallState 安装状态
type InstallState struct {
	DBConnected   bool   `json:"db_connected"`
	TablesExist   bool   `json:"tables_exist"`
	AdminExists   bool   `json:"admin_exists"`
	SystemVersion string `json:"system_version"`
}

// InitSystemRequest 初始化系统请求
type InitSystemRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Password  string `json:"password" binding:"required,min=8,max=60"`
	GivenName string `json:"given_name"`
	Email     string `json:"email" binding:"omitempty,email"`
}
