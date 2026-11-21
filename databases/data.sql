-- ============================================================
-- Citebase 初始数据 DML
-- 注意: 系统启动时 GORM AutoMigrate + seedDefaults() 会自动补全
-- 本文件仅用于手动初始化数据库的参考
-- 默认管理员: 用户名 admin，初始密码由 seedDefaults() 随机生成并只在首次启动日志中打印一次
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. 系统角色
-- ============================================================
INSERT INTO `aw_role` (`role_id`, `name`, `type`, `description`, `list_order`, `is_delete`, `create_time`, `update_time`) VALUES
(1, '超级管理员', 1, '系统内置超级管理员，拥有所有权限', 0, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
(2, '管理员', 1, '系统内置管理员，拥有大部分管理权限', 0, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
(3, '普通用户', 1, '系统内置普通用户，仅有个人中心权限', 0, 0, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());

-- ============================================================
-- 2. 系统权限（93项）
-- 注意: 完整权限列表较长，首次启动时由 seedPrivileges() 自动插入
-- 以下仅列出顶层菜单，完整数据由应用自动补全
-- ============================================================

-- ============================================================
-- 3. 角色权限关系
-- 注意: 超级管理员(role_id=1)默认拥有全部权限
-- 管理员(role_id=2)和普通用户(role_id=3)的权限由应用自动分配
-- ============================================================

-- ============================================================
-- 4. 系统配置默认值
-- 以下配置项与 seedDefaults() 的 defaultConfigs 逐行对齐
-- （改 db.go 后请同步本文件，或直接用 AutoMigrate+seedDefaults 初始化）:
-- ============================================================
INSERT INTO `aw_config` (`key`, `value`, `label`, `group_key`, `type`, `options`, `is_private`, `sort`, `create_time`, `update_time`) VALUES
('main_title', 'Citebase', '主页标题', 'basic', 'text', NULL, 0, 4, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('main_description', '', '主页描述', 'basic', 'textarea', NULL, 0, 5, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('system_name', 'Citebase', '系统名称', 'basic', 'text', NULL, 0, 6, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('system_version', '2.0.0', '系统版本', 'system', 'text', NULL, 0, 10, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('fulltext_search_open', '1', '开启全文搜索', 'search', 'switch', NULL, 0, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('auto_follow_doc_open', '0', '自动关注文档', 'basic', 'switch', NULL, 0, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('auto_follow_doc', '', '自动关注文档配置', 'basic', 'text', NULL, 0, 7, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('send_email_open', '0', '发送邮件通知', 'basic', 'switch', NULL, 0, 2, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('sso_open', '0', '统一登录(SSO)', 'basic', 'switch', NULL, 0, 3, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
('wiki.multiagent.enabled', '0', '多智能体深度研究', 'wiki', 'switch', NULL, 0, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());
SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- 重要提示:
-- 1. 首次启动应用时，系统会自动:
--    - 执行 GORM AutoMigrate 创建/更新所有表结构
--    - 调用 ensureAllIndexes() 补全缺失索引
--    - 调用 ensureMissingColumns() 补全缺失字段
--    - 调用 seedDefaults() 创建默认管理员(随机初始密码)、
--      完整权限列表(93项)、角色权限关系、完整系统配置
-- 2. 如无特殊需求，无需手动执行本文件
-- 3. 从旧版本升级时，系统会自动检测并补全缺失的表、字段、索引和数据
-- ============================================================
