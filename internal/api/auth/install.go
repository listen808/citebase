package auth

import (
	"github.com/gin-gonic/gin"
	logx "github.com/listen808/citebase/internal/log"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	repos "github.com/listen808/citebase/internal/repository"
	"github.com/listen808/citebase/internal/response"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/version"
	"golang.org/x/crypto/bcrypt"
	"sync"
	"time"
)

// installMu 串行化安装写入，避免并发请求各自通过「是否已初始化」检查。
var installMu sync.Mutex

// InstallAPI 安装向导 API（公开，无需认证）
type InstallAPI struct{}

func NewInstallAPI() *InstallAPI {
	return &InstallAPI{}
}

// GetInstallState 获取安装状态
// @Summary 安装状态
// @Tags 安装向导
// @Produce json
// @Success 200
// @Router /api/v1/install/state [get]
func (api *InstallAPI) GetInstallState(c *gin.Context) {
	db := repos.DB

	state := &adminmodel.InstallState{
		DBConnected:   true,
		TablesExist:   true,
		AdminExists:   false,
		SystemVersion: version.Version,
	}

	// 测试数据库连接
	if err := db.Exec("SELECT 1").Error; err != nil {
		state.DBConnected = false
		response.Success(c, state)
		return
	}

	// 检查表是否存在
	var tableCount int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('aw_user','aw_space','aw_document')").Scan(&tableCount)
	if tableCount < 3 {
		state.TablesExist = false
		response.Success(c, state)
		return
	}

	// 检查是否已有管理员
	var adminCount int64
	db.Model(&usermodel.User{}).Where("role_id = ?", 1).Count(&adminCount)
	if adminCount > 0 {
		state.AdminExists = true
	}

	response.Success(c, state)
}

// InitDatabase 初始化数据库表结构
// @Summary 初始化数据库
// @Tags 安装向导
// @Produce json
// @Success 200
// @Router /api/v1/install/init-db [post]
func (api *InstallAPI) InitDatabase(c *gin.Context) {
	db := repos.DB

	// 检查是否已初始化
	var adminCount int64
	db.Model(&usermodel.User{}).Where("role_id = ?", 1).Count(&adminCount)
	if adminCount > 0 {
		response.ConflictKey(c, response.MsgAlreadyInitialized)
		return
	}

	// 创建表（从 table.sql 读取建表语句执行）
	sqls := []string{
		// aw_user 表
		`CREATE TABLE IF NOT EXISTS aw_user (
			user_id INT(11) NOT NULL AUTO_INCREMENT,
			username VARCHAR(100) NOT NULL,
			password VARCHAR(60) NOT NULL,
			given_name VARCHAR(50) DEFAULT '',
			mobile VARCHAR(13) DEFAULT '',
			phone VARCHAR(13) DEFAULT '',
			email VARCHAR(50) DEFAULT '',
			department VARCHAR(50) DEFAULT '',
			position VARCHAR(50) DEFAULT '',
			location VARCHAR(50) DEFAULT '',
			im VARCHAR(50) DEFAULT '',
			last_ip VARCHAR(15) DEFAULT '',
			last_time INT(11) DEFAULT 0,
			role_id INT(11) DEFAULT 3,
			is_forbidden INT(1) DEFAULT 0,
			is_delete INT(1) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (user_id),
			UNIQUE KEY uk_username (username)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_role 表
		`CREATE TABLE IF NOT EXISTS aw_role (
			role_id INT(11) NOT NULL AUTO_INCREMENT,
			name VARCHAR(50) NOT NULL,
			type INT(1) DEFAULT 0,
			role_order INT(3) DEFAULT 0,
			is_delete INT(1) DEFAULT 0,
			PRIMARY KEY (role_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_privilege 表
		`CREATE TABLE IF NOT EXISTS aw_privilege (
			privilege_id INT(11) NOT NULL AUTO_INCREMENT,
			privilege_id_value INT(11) DEFAULT 0,
			parent_id INT(11) DEFAULT 0,
			name VARCHAR(50) DEFAULT '',
			controller VARCHAR(50) DEFAULT '',
			action VARCHAR(50) DEFAULT '',
			type VARCHAR(20) DEFAULT '',
			is_display INT(1) DEFAULT 1,
			icon VARCHAR(50) DEFAULT '',
			sort INT(3) DEFAULT 0,
			is_delete INT(1) DEFAULT 0,
			PRIMARY KEY (privilege_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_role_privilege 表
		`CREATE TABLE IF NOT EXISTS aw_role_privilege (
			role_id INT(11) NOT NULL,
			privilege_id INT(11) NOT NULL,
			PRIMARY KEY (role_id,privilege_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_space 表
		`CREATE TABLE IF NOT EXISTS aw_space (
			space_id INT(11) NOT NULL AUTO_INCREMENT,
			name VARCHAR(50) NOT NULL,
			description TEXT,
			creator_id INT(11) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			update_time INT(11) DEFAULT 0,
			is_delete INT(1) DEFAULT 0,
			visit_level INT(1) DEFAULT 1,
			is_share INT(1) DEFAULT 1,
			is_export INT(1) DEFAULT 1,
			PRIMARY KEY (space_id),
			KEY idx_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_space_user 表
		`CREATE TABLE IF NOT EXISTS aw_space_user (
			space_user_id INT(11) NOT NULL AUTO_INCREMENT,
			user_id INT(11) NOT NULL,
			space_id INT(11) NOT NULL,
			privilege INT(1) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (space_user_id),
			UNIQUE KEY uk_user_space (user_id,space_id),
			KEY idx_user_id (user_id),
			KEY idx_space_id (space_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_document 表
		`CREATE TABLE IF NOT EXISTS aw_document (
			document_id INT(11) NOT NULL AUTO_INCREMENT,
			space_id INT(11) NOT NULL,
			parent_id INT(11) DEFAULT 0,
			name VARCHAR(100) NOT NULL,
			document_key VARCHAR(50) DEFAULT '',
			creator_id INT(11) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			edit_user_id INT(11) DEFAULT 0,
			update_time INT(11) DEFAULT 0,
			type INT(1) DEFAULT 1,
			is_delete INT(1) DEFAULT 0,
			path VARCHAR(255) DEFAULT '',
			PRIMARY KEY (document_id),
			KEY idx_space_id (space_id),
			KEY idx_parent_id (parent_id),
			FULLTEXT KEY ft_name_content (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_document_version 表
		`CREATE TABLE IF NOT EXISTS aw_document_version (
			id INT(11) NOT NULL AUTO_INCREMENT,
			document_id INT(11) NOT NULL,
			version INT(11) NOT NULL DEFAULT 1,
			content LONGTEXT,
			change_log VARCHAR(255) DEFAULT '',
			create_user_id INT(11) NOT NULL,
			create_time INT(11) NOT NULL,
			PRIMARY KEY (id),
			KEY idx_doc_version (document_id,version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_attachment 表
		`CREATE TABLE IF NOT EXISTS aw_attachment (
			attachment_id INT(11) NOT NULL AUTO_INCREMENT,
			user_id INT(11) DEFAULT 0,
			document_id INT(11) DEFAULT 0,
			path VARCHAR(255) NOT NULL,
			source INT(1) DEFAULT 0,
			name VARCHAR(255) DEFAULT '',
			size BIGINT(20) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (attachment_id),
			KEY idx_document_id (document_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_collection 表
		`CREATE TABLE IF NOT EXISTS aw_collection (
			collection_id INT(11) NOT NULL AUTO_INCREMENT,
			user_id INT(11) NOT NULL,
			type INT(1) DEFAULT 1,
			resource_id INT(11) NOT NULL,
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (collection_id),
			UNIQUE KEY uk_user_resource (user_id,type,resource_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_follow 表
		`CREATE TABLE IF NOT EXISTS aw_follow (
			id INT(11) NOT NULL AUTO_INCREMENT,
			user_id INT(11) NOT NULL,
			type INT(1) NOT NULL,
			object_id INT(11) NOT NULL,
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (id),
			UNIQUE KEY uk_user_object (user_id,type,object_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_log_document 表
		`CREATE TABLE IF NOT EXISTS aw_log_document (
			log_document_id INT(11) NOT NULL AUTO_INCREMENT,
			document_id INT(11) DEFAULT 0,
			space_id INT(11) DEFAULT 0,
			action INT(1) DEFAULT 0,
			user_id INT(11) DEFAULT 0,
			change_log VARCHAR(255) DEFAULT '',
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (log_document_id),
			KEY idx_document_id (document_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_log 表
		`CREATE TABLE IF NOT EXISTS aw_log (
			log_id INT(11) NOT NULL AUTO_INCREMENT,
			username VARCHAR(50) DEFAULT '',
			path VARCHAR(255) DEFAULT '',
			method VARCHAR(10) DEFAULT '',
			get TEXT,
			post TEXT,
			ip VARCHAR(50) DEFAULT '',
			user_agent VARCHAR(255) DEFAULT '',
			level VARCHAR(20) DEFAULT '',
			create_time INT(11) DEFAULT 0,
			PRIMARY KEY (log_id),
			KEY idx_create_time (create_time),
			KEY idx_level (level)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_config 表
		`CREATE TABLE IF NOT EXISTS aw_config (
			config_id INT(11) NOT NULL AUTO_INCREMENT,
			config_key VARCHAR(50) NOT NULL,
			value TEXT,
			label VARCHAR(100) DEFAULT '',
			group_key VARCHAR(50) DEFAULT '',
			type VARCHAR(20) DEFAULT 'text',
			options TEXT,
			sort INT(3) DEFAULT 0,
			is_private INT(1) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			update_time INT(11) DEFAULT 0,
			PRIMARY KEY (config_id),
			UNIQUE KEY uk_key (config_key)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_email 表
		`CREATE TABLE IF NOT EXISTS aw_email (
			email_id INT(11) NOT NULL AUTO_INCREMENT,
			name VARCHAR(50) NOT NULL,
			sender_address VARCHAR(100) NOT NULL,
			sender_name VARCHAR(50) DEFAULT '',
			sender_title_prefix VARCHAR(50) DEFAULT '',
			host VARCHAR(100) NOT NULL,
			port VARCHAR(10) DEFAULT '25',
			username VARCHAR(100) NOT NULL,
			password VARCHAR(100) NOT NULL,
			is_ssl INT(1) DEFAULT 0,
			is_used INT(1) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			update_time INT(11) DEFAULT 0,
			PRIMARY KEY (email_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// aw_login_auth 表
		`CREATE TABLE IF NOT EXISTS aw_login_auth (
			login_auth_id INT(11) NOT NULL AUTO_INCREMENT,
			name VARCHAR(50) NOT NULL,
			url VARCHAR(255) NOT NULL,
			username_prefix VARCHAR(20) NOT NULL,
			ext_data TEXT,
			is_used INT(1) DEFAULT 0,
			create_time INT(11) DEFAULT 0,
			update_time INT(11) DEFAULT 0,
			PRIMARY KEY (login_auth_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, sql := range sqls {
		if err := db.Exec(sql).Error; err != nil {
			response.ServerErrorSafe(c, err)
			return
		}
	}

	response.SuccessWithMsg(c, nil, "数据库初始化成功")
}

// InitSystem 初始化系统（创建管理员等基础数据）
// @Summary 初始化系统
// @Tags 安装向导
// @Accept json
// @Produce json
// @Success 201
// @Router /api/v1/install/init-system [post]
func (api *InstallAPI) InitSystem(c *gin.Context) {
	var req adminmodel.InitSystemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := utils.ValidatePasswordStrength(req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	db := repos.DB

	// 安装接口是公开路由：必须串行化并在提交前二次确认，否则并发请求可以抢在
	// 真正的管理员之前创建超级管理员账号。
	installMu.Lock()
	defer installMu.Unlock()

	var adminCount int64
	if err := db.Model(&usermodel.User{}).Where("role_id = ?", 1).Count(&adminCount).Error; err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if adminCount > 0 {
		response.ConflictKey(c, response.MsgAlreadyInitialized)
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		response.ServerErrorSafe(c, tx.Error)
		return
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.Rollback().Error; err != nil {
				logx.Warnf("[install] 回滚安装事务失败: %v", err)
			}
		}
	}()

	// 创建角色（超级管理员、管理员、普通用户）
	roles := []usermodel.Role{
		{ID: 1, Name: "超级管理员", Type: 1, IsDelete: 0},
		{ID: 2, Name: "管理员", Type: 1, IsDelete: 0},
		{ID: 3, Name: "普通用户", Type: 1, IsDelete: 0},
	}
	for _, role := range roles {
		var existingRole usermodel.Role
		if err := tx.Where(usermodel.Role{ID: role.ID}).FirstOrCreate(&existingRole).Error; err != nil {
			response.ServerErrorSafe(c, err)
			return
		}
		if existingRole.Name == "" {
			existingRole.Name = role.Name
			existingRole.Type = role.Type
			existingRole.IsDelete = role.IsDelete
			if err := tx.Save(&existingRole).Error; err != nil {
				response.ServerErrorSafe(c, err)
				return
			}
		}
	}

	// 创建超级管理员
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	admin := &usermodel.User{
		Username:    req.Username,
		Password:    string(hashedPassword),
		GivenName:   req.GivenName,
		Email:       req.Email,
		RoleID:      1,
		IsDelete:    0,
		IsForbidden: 0,
	}
	if err := tx.Create(admin).Error; err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 初始化默认配置。清单只在 repository.DefaultConfigSeeds 一处维护 ——
	// 这里原先另抄了一份，两份的值、标签、排序号各自漂移过（main_title 一份写
	// "Citebase"、一份写 "Citebase 知识库"），于是"用安装向导"和"跑 table.sql+data.sql"
	// 装出来的库在后台显示不一样。漂移由 tools/repocheck/brand_source_test.go 拦住。
	now := int(time.Now().Unix())
	for _, seed := range repos.DefaultConfigSeeds() {
		var existing adminmodel.Config
		result := tx.Where(adminmodel.Config{Key: seed.Key}).Limit(1).Find(&existing)
		if result.Error != nil {
			response.ServerErrorSafe(c, result.Error)
			return
		}
		isNew := result.RowsAffected == 0
		row := mergeConfigSeed(existing, seed, isNew, now)
		write := tx.Create(&row)
		if !isNew {
			// 已存在的行按主键更新（existing.ID 由 Find 填好），
			// 不再走 FirstOrCreate —— 那条路径要依赖"GORM 创建后是否回填主键"，
			// 一旦回填不上就是往同一个 uniqueIndex:uk_key 再插一行。
			write = tx.Save(&row)
		}
		if write.Error != nil {
			response.ServerErrorSafe(c, write.Error)
			return
		}
	}

	// 提交前再确认一次：只允许本次创建的这一个超级管理员存在
	var finalCount int64
	if err := tx.Model(&usermodel.User{}).Where("role_id = ?", 1).Count(&finalCount).Error; err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if finalCount > 1 {
		response.ConflictKey(c, response.MsgConcurrentInit)
		return
	}
	if err := tx.Commit().Error; err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	committed = true

	response.Created(c, gin.H{
		"username": req.Username,
		"user_id":  admin.ID,
	})
}

// mergeConfigSeed 决定"这一行是新建的还是本来就有的"，两种情况各写哪些字段。
//
// 为什么要有这台纯函数：安装向导的配置写入原先是 FirstOrCreate 之后**无条件**覆盖全部字段，
// 于是 init-system 被再点一次（或有人拿向导去"修复"一个装好的库）就会把管理员自己填的值
// 冲回种子默认 —— 站点名现在真的生效了，被冲掉的表现是全站名字一夜回到 Citebase。
// 本仓没有可用的数据库测试工装（全仓 hermetic 单测都不碰库），所以把判据提成纯函数来测：
// GORM 只负责告诉我这一行是不是刚建的（RowsAffected），怎么写由这里定义一次。
//
// 规则：
//   - Value / CreateTime 只在新建行时写 —— 用户的数据不是这份清单的属性；
//   - Label / GroupKey / Type / Sort / Options 每次对齐 —— 它们是清单的结构属性，
//     不对齐的话清单改了也永远修不到现存的库上（Options 尤其明显：
//     select 类型的行没有选项，管理端那个下拉就是空的）。
func mergeConfigSeed(existing, seed adminmodel.Config, isNew bool, now int) adminmodel.Config {
	merged := existing
	// 键名取自清单 —— 我们本来就是按它匹配的，写回来一次就不必依赖
	// "GORM 会不会把 Where 条件回填到目标对象"这种实现细节（那条路径变了会插出空 key 的行）。
	merged.Key = seed.Key
	merged.Label = seed.Label
	merged.GroupKey = seed.GroupKey
	merged.Type = seed.Type
	merged.Sort = seed.Sort
	merged.Options = seed.Options
	merged.UpdateTime = now
	if isNew {
		merged.Value = seed.Value
		merged.CreateTime = now
	}
	return merged
}
