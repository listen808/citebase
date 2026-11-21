package middleware

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	"github.com/listen808/citebase/internal/response"
)

// ============================================================
// RBAC 权限中间件
// 基于 aw_role / aw_privilege / aw_role_privilege 三表
// 超级管理员(role_id=1) 拥有所有权限，自动放行
// ============================================================

var roleRepo = userrepo.NewRoleRepository()

// rbacCache RBAC 权限缓存（30秒TTL）
type rbacCacheEntry struct {
	value     interface{}
	createdAt time.Time
}

var (
	rbacCache    sync.Map
	rbacCacheTTL = 30 * time.Second
)

func getFromCache(key string) (interface{}, bool) {
	if v, ok := rbacCache.Load(key); ok {
		entry := v.(rbacCacheEntry)
		if time.Since(entry.createdAt) < rbacCacheTTL {
			return entry.value, true
		}
		rbacCache.Delete(key)
	}
	return nil, false
}

func setCache(key string, value interface{}) {
	rbacCache.Store(key, rbacCacheEntry{value: value, createdAt: time.Now()})
}

// InvalidateUserRBACCache 主动失效指定用户的 RBAC 缓存
// P0-9: 在角色/权限变更（角色分配、权限授予/撤销、用户角色变更）时调用，
// 避免 30s TTL 窗口内仍命中旧权限导致越权或权限残留。
// 由于 rbac:priv:<uid>:<ctrl>:<action> 的 key 是动态拼接的，
// 这里通过遍历 sync.Map 删除该 uid 的所有缓存项。
func InvalidateUserRBACCache(userID int) {
	if userID <= 0 {
		return
	}
	uidStr := strconv.Itoa(userID)
	prefixSuper := "rbac:super:" + uidStr
	prefixAdmin := "rbac:admin:" + uidStr
	prefixPriv := "rbac:priv:" + uidStr + ":"
	rbacCache.Range(func(key, _ interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if k == prefixSuper || k == prefixAdmin || strings.HasPrefix(k, prefixPriv) {
			rbacCache.Delete(k)
		}
		return true
	})
}

// InvalidateAllRBACCache 主动失效全部 RBAC 缓存
// 在大规模权限调整（如批量修改角色、删除角色）时调用
func InvalidateAllRBACCache() {
	rbacCache.Range(func(key, _ interface{}) bool {
		rbacCache.Delete(key)
		return true
	})
}

// RequirePrivilege 返回一个检查指定 controller.action 权限的中间件
// 用法: router.GET("/users", middleware.AuthMiddleware(), middleware.RequirePrivilege("user", "list"), userAPI.GetUserList)
func RequirePrivilege(controller, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}

		uid, ok := userID.(int)
		if !ok {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}
		hasPriv, err := roleRepo.HasPrivilege(uid, controller, action)
		if err != nil {
			response.ServerError(c, "failed to check privilege")
			c.Abort()
			return
		}

		if !hasPriv {
			response.Forbidden(c, "you do not have permission to perform this action")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdmin 超级管理员或管理员专用
// 对应 data.sql 预置角色：超级管理员(role_id=1) 和 管理员(role_id=2)
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}

		uid, ok := userID.(int)
		if !ok {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}
		isAdmin, err := roleRepo.IsAdmin(uid)
		if err != nil {
			response.ServerError(c, "failed to check role")
			c.Abort()
			return
		}

		if !isAdmin {
			response.Forbidden(c, "admin privileges required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSuperAdmin 仅超级管理员可用
// 对应 data.sql 预置角色：超级管理员(role_id=1)
func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}

		uid, ok := userID.(int)
		if !ok {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}
		isSuper, err := roleRepo.IsSuperAdmin(uid)
		if err != nil {
			response.ServerError(c, "failed to check role")
			c.Abort()
			return
		}

		if !isSuper {
			response.Forbidden(c, "super admin privileges required")
			c.Abort()
			return
		}

		c.Next()
	}
}

// PrivilegeMap 权限路由映射表
// key: HTTP_METHOD:PATH  (使用前缀匹配)
// value: required role minimum (1=super, 2=admin, 3=any)
//
// 使用场景示例:
//   - 用户管理 CRUD  → 超级管理员
//   - 空间管理       → 管理员
//   - 文档操作       → 空间成员（由 spaceService 控制，不走 RBAC 中间件）
var PrivilegeMap = map[string]struct {
	Controller string
	Action     string
	MinRole    int // 1=super admin, 2=admin, 3=normal user
}{
	// 用户管理（超级管理员）
	"GET:/api/v1/users":               {Controller: "user", Action: "list", MinRole: 1},
	"GET:/api/v1/users/export":        {Controller: "user", Action: "export", MinRole: 1},
	"POST:/api/v1/users":              {Controller: "user", Action: "save", MinRole: 1},
	"PUT:/api/v1/users/:id":           {Controller: "user", Action: "modify", MinRole: 1},
	"DELETE:/api/v1/users/:id":        {Controller: "user", Action: "delete", MinRole: 1},
	"GET:/api/v1/users/:id":           {Controller: "user", Action: "detail", MinRole: 1},
	"PUT:/api/v1/users/:id/password":  {Controller: "user", Action: "password", MinRole: 1},
	"PUT:/api/v1/users/:id/forbidden": {Controller: "user", Action: "forbidden", MinRole: 1},
	"GET:/api/v1/users/for-import":    {Controller: "user", Action: "for_import", MinRole: 1},

	// 角色管理（超级管理员）
	// 这一组必须与 api/auth/routes.go 里的角色/权限路由逐条对齐：
	// /api/v1/roles、/api/v1/users、/api/v1/privileges 都不在 adminPathPrefixes 里，
	// 而 AutoRBAC 对「既没命中规则、又不在管理前缀」的请求是直接 c.Next() 放行，
	// 所以漏登记一条 = 任何登录用户都能调。原先漏的四条里，
	// POST /roles/:id/users 可以让普通用户把自己设成 role 2（IsAdmin 认 1 和 2），
	// 一次调用即拿到全站管理员权限。由 TestPrivilegedRouteGroupsAreAllRegistered 钉住。
	"GET:/api/v1/roles":                  {Controller: "role", Action: "list", MinRole: 1},
	"POST:/api/v1/roles":                 {Controller: "role", Action: "save", MinRole: 1},
	"PUT:/api/v1/roles/:id":              {Controller: "role", Action: "modify", MinRole: 1},
	"DELETE:/api/v1/roles/:id":           {Controller: "role", Action: "delete", MinRole: 1},
	"GET:/api/v1/roles/:id/privileges":   {Controller: "role", Action: "privileges", MinRole: 1},
	"POST:/api/v1/roles/:id/privileges":  {Controller: "role", Action: "assign_privileges", MinRole: 1},
	"GET:/api/v1/roles/:id/users":        {Controller: "role", Action: "users", MinRole: 1},
	"POST:/api/v1/roles/:id/users":       {Controller: "role", Action: "add_user", MinRole: 1},
	"POST:/api/v1/roles/:id/reset-users": {Controller: "role", Action: "reset_users", MinRole: 1},

	// 权限管理（超级管理员）
	"GET:/api/v1/privileges":        {Controller: "privilege", Action: "list", MinRole: 1},
	"POST:/api/v1/privileges":       {Controller: "privilege", Action: "save", MinRole: 1},
	"PUT:/api/v1/privileges/:id":    {Controller: "privilege", Action: "modify", MinRole: 1},
	"DELETE:/api/v1/privileges/:id": {Controller: "privilege", Action: "delete", MinRole: 1},

	// 空间管理（管理员）
	"POST:/api/v1/spaces":                      {Controller: "space", Action: "save", MinRole: 2},
	"PUT:/api/v1/spaces/:id":                   {Controller: "space", Action: "modify", MinRole: 2},
	"DELETE:/api/v1/spaces/:id":                {Controller: "space", Action: "delete", MinRole: 2},
	"GET:/api/v1/spaces/:id/members":           {Controller: "space", Action: "members", MinRole: 2},
	"POST:/api/v1/spaces/:id/members":          {Controller: "space", Action: "add_member", MinRole: 2},
	"DELETE:/api/v1/spaces/:id/members":        {Controller: "space", Action: "remove_member", MinRole: 2},
	"PUT:/api/v1/spaces/:id/members/privilege": {Controller: "space", Action: "update_member_priv", MinRole: 2},
	"GET:/api/v1/spaces/:id/download":          {Controller: "space", Action: "download", MinRole: 2},

	// 搜索重建索引（管理员）
	// /api/v1/search 整段不在 adminPathPrefixes 里（公开检索走 OptionalAuth），
	// 所以这一组里的"诊断"类路由必须逐条登记，漏一条就是任何登录用户可调。
	"POST:/api/v1/search/reindex":       {Controller: "search", Action: "reindex", MinRole: 2},
	"GET:/api/v1/search/reindex/status": {Controller: "search", Action: "reindex_status", MinRole: 2},
	"GET:/api/v1/search/init":           {Controller: "search", Action: "init_index", MinRole: 2},
	"POST:/api/v1/search/index/:id":     {Controller: "search", Action: "index_doc", MinRole: 2},
	"GET:/api/v1/search/monitor":        {Controller: "search", Action: "monitor", MinRole: 2},
	"GET:/api/v1/search/debug/:id":      {Controller: "search", Action: "debug_doc", MinRole: 2},

	// 邮件管理（管理员）
	"GET:/api/v1/emails":             {Controller: "email", Action: "list", MinRole: 2},
	"POST:/api/v1/emails":            {Controller: "email", Action: "save", MinRole: 2},
	"GET:/api/v1/emails/:id":         {Controller: "email", Action: "detail", MinRole: 2},
	"PUT:/api/v1/emails/:id":         {Controller: "email", Action: "modify", MinRole: 2},
	"DELETE:/api/v1/emails/:id":      {Controller: "email", Action: "delete", MinRole: 2},
	"PUT:/api/v1/emails/:id/default": {Controller: "email", Action: "set_default", MinRole: 2},
	"POST:/api/v1/emails/test":       {Controller: "email", Action: "test", MinRole: 2},

	// 系统配置（管理员）
	"GET:/api/v1/configs":       {Controller: "config", Action: "list", MinRole: 2},
	"PUT:/api/v1/configs":       {Controller: "config", Action: "modify", MinRole: 2},
	"GET:/api/v1/configs/value": {Controller: "config", Action: "value", MinRole: 2},

	// SSO 统一认证配置（管理员）
	"GET:/api/v1/auth-sso":             {Controller: "sso", Action: "list", MinRole: 2},
	"GET:/api/v1/auth-sso/:id":         {Controller: "sso", Action: "detail", MinRole: 2},
	"POST:/api/v1/auth-sso":            {Controller: "sso", Action: "save", MinRole: 2},
	"PUT:/api/v1/auth-sso/:id":         {Controller: "sso", Action: "modify", MinRole: 2},
	"DELETE:/api/v1/auth-sso/:id":      {Controller: "sso", Action: "delete", MinRole: 2},
	"PUT:/api/v1/auth-sso/:id/default": {Controller: "sso", Action: "set_default", MinRole: 2},

	// 链接管理（管理员）
	"GET:/api/v1/links":        {Controller: "link", Action: "list", MinRole: 2},
	"GET:/api/v1/links/all":    {Controller: "link", Action: "list_all", MinRole: 2},
	"POST:/api/v1/links":       {Controller: "link", Action: "save", MinRole: 2},
	"PUT:/api/v1/links/:id":    {Controller: "link", Action: "modify", MinRole: 2},
	"DELETE:/api/v1/links/:id": {Controller: "link", Action: "delete", MinRole: 2},
	// 打开计数走 MinRole 3（任何登录用户）：/api/v1/links 整段在 adminPathPrefixes 里，
	// 不显式登记这条就会默认要管理员，用户端将来点不到也计不上。
	"POST:/api/v1/links/:id/click": {Controller: "link", Action: "click", MinRole: 3},

	// 插件管理（管理员）
	"GET:/api/v1/plugins":            {Controller: "plugin", Action: "list", MinRole: 2},
	"POST:/api/v1/plugins":           {Controller: "plugin", Action: "save", MinRole: 2},
	"GET:/api/v1/plugins/:id":        {Controller: "plugin", Action: "detail", MinRole: 2},
	"PUT:/api/v1/plugins/:id":        {Controller: "plugin", Action: "modify", MinRole: 2},
	"PUT:/api/v1/plugins/:id/config": {Controller: "plugin", Action: "update_config", MinRole: 2},
	"PUT:/api/v1/plugins/:id/toggle": {Controller: "plugin", Action: "toggle", MinRole: 2},
	"DELETE:/api/v1/plugins/:id":     {Controller: "plugin", Action: "delete", MinRole: 2},

	// 联系人管理（管理员）
	"GET:/api/v1/contacts":               {Controller: "contact", Action: "list", MinRole: 2},
	"GET:/api/v1/contacts/all":           {Controller: "contact", Action: "list_all", MinRole: 2},
	"POST:/api/v1/contacts":              {Controller: "contact", Action: "save", MinRole: 2},
	"PUT:/api/v1/contacts/:id":           {Controller: "contact", Action: "modify", MinRole: 2},
	"DELETE:/api/v1/contacts/:id":        {Controller: "contact", Action: "delete", MinRole: 2},
	"GET:/api/v1/contacts/:id":           {Controller: "contact", Action: "detail", MinRole: 2},
	"POST:/api/v1/contacts/import-users": {Controller: "contact", Action: "import_users", MinRole: 2},

	// 系统日志（管理员）
	"GET:/api/v1/logs/system":             {Controller: "log", Action: "system", MinRole: 2},
	"GET:/api/v1/logs/recent-errors":      {Controller: "log", Action: "recent_errors", MinRole: 2},
	"GET:/api/v1/logs/users/:id/activity": {Controller: "log", Action: "user_activity", MinRole: 2},

	// AI 配置管理（管理员）
	"GET:/api/v1/ai/config":                      {Controller: "ai", Action: "config_read", MinRole: 2},
	"POST:/api/v1/ai/config/update":              {Controller: "ai", Action: "config_update", MinRole: 2},
	"POST:/api/v1/ai/config/test":                {Controller: "ai", Action: "config_test", MinRole: 2},
	"POST:/api/v1/ai/config/schedule":            {Controller: "ai", Action: "schedule_update", MinRole: 2},
	"GET:/api/v1/ai/config/schedule":             {Controller: "ai", Action: "schedule_read", MinRole: 2},
	"PUT:/api/v1/ai/data-query/prompts/:id":      {Controller: "ai", Action: "prompt_update", MinRole: 2},
	"POST:/api/v1/ai/cache/clear":                {Controller: "ai", Action: "cache_clear", MinRole: 2},
	"POST:/api/v1/ai/analysis/start":             {Controller: "ai", Action: "analysis_start", MinRole: 2},
	"POST:/api/v1/ai/analysis/pause/:id":         {Controller: "ai", Action: "analysis_pause", MinRole: 2},
	"POST:/api/v1/ai/analysis/resume/:id":        {Controller: "ai", Action: "analysis_resume", MinRole: 2},
	"POST:/api/v1/ai/analysis/detect-duplicates": {Controller: "ai", Action: "detect_duplicates", MinRole: 2},
	"GET:/api/v1/ai/analysis/clusters":           {Controller: "ai", Action: "clusters_read", MinRole: 2},
	"POST:/api/v1/ai/analysis/merge/:id":         {Controller: "ai", Action: "cluster_merge", MinRole: 2},
	"POST:/api/v1/ai/vector/sync":                {Controller: "ai", Action: "vector_sync", MinRole: 2},
	"POST:/api/v1/ai/doc/review":                 {Controller: "ai", Action: "doc_review", MinRole: 2},
	"POST:/api/v1/ai/import":                     {Controller: "ai", Action: "import_doc", MinRole: 2},
	"POST:/api/v1/ai/ragas/evaluate":             {Controller: "ai", Action: "ragas_evaluate", MinRole: 2},
	"DELETE:/api/v1/ai/ragas/results/:id":        {Controller: "ai", Action: "ragas_delete", MinRole: 2},
	"POST:/api/v1/ai/auto-tag":                   {Controller: "ai", Action: "auto_tag", MinRole: 2},
	"POST:/api/v1/ai/entities/extract":           {Controller: "ai", Action: "entities_extract", MinRole: 2},
	// 错误看板：聚合的是全库 SQL 执行失败记录（含别人的查询原文），注册时挂在
	// 逐条带 AutoRBAC 的 aiDataQuery 组上，但 adminPathPrefixes 里当时写的是
	// /api/v1/ai/data-query/error-dashboard —— 那个路径从来没有路由，
	// 于是这两条真路由既没规则也没前缀，落到 c.Next()。
	"GET:/api/v1/ai/data-query/error-summary":   {Controller: "ai", Action: "error_summary", MinRole: 2},
	"GET:/api/v1/ai/data-query/recent-failures": {Controller: "ai", Action: "recent_failures", MinRole: 2},

	// P0 补口：以下路由组此前完全没有 RBAC 覆盖（既不在 PrivilegeMap 也不在管理前缀里），
	// 任意登录用户即可调用可修改全局数据源、Schema 知识库与导出全库结构的能力。
	"POST:/api/v1/schema-spaces":                       {Controller: "schema", Action: "space_create", MinRole: 2},
	"DELETE:/api/v1/schema-spaces/:id":                 {Controller: "schema", Action: "space_delete", MinRole: 2},
	"POST:/api/v1/schema-spaces/:id/scan":              {Controller: "schema", Action: "space_scan", MinRole: 2},
	"POST:/api/v1/schema-spaces/:id/refresh-knowledge": {Controller: "schema", Action: "space_refresh_knowledge", MinRole: 2},
	"POST:/api/v1/schema-spaces/:id/rules":             {Controller: "schema", Action: "rule_create", MinRole: 2},
	"DELETE:/api/v1/schema-spaces/:id/rules/:rule_id":  {Controller: "schema", Action: "rule_delete", MinRole: 2},
	"POST:/api/v1/schema-export-csv":                   {Controller: "schema", Action: "export_csv", MinRole: 2},
	"DELETE:/api/v1/schema-scan-tasks/:task_id":        {Controller: "schema", Action: "scan_task_delete", MinRole: 2},
	// 同批补全：这一族当时只登记了写侧，读侧全部漏空 —— 而 Schema 空间描述的是
	// 已配置数据源的库结构（表名/列名/采样值缓存），并能直接跑 text2sql，
	// 属于「没登记就等于任何登录用户可读」那条静默通道上的路由。
	// 消费者只有 /ai/settings 下的 Schema 管理与 SQL 历史两个子页，
	// 而那条路由的 meta.roles 本来就是 ['admin','super_admin']，抬到管理员不影响任何用户侧功能。
	"GET:/api/v1/schema-spaces":                        {Controller: "schema", Action: "space_list", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id":                    {Controller: "schema", Action: "space_detail", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/tables":             {Controller: "schema", Action: "space_tables", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/tables/:table_name": {Controller: "schema", Action: "space_table_detail", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/stats":              {Controller: "schema", Action: "space_stats", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/rules":              {Controller: "schema", Action: "rule_list", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/table-cache":        {Controller: "schema", Action: "space_table_cache", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/debug-scan":         {Controller: "schema", Action: "space_debug_scan", MinRole: 2},
	"GET:/api/v1/schema-spaces/:id/scan-tasks":         {Controller: "schema", Action: "space_scan_tasks", MinRole: 2},
	"DELETE:/api/v1/schema-spaces/:id/scan-tasks":      {Controller: "schema", Action: "space_scan_tasks_delete", MinRole: 2},
	"POST:/api/v1/schema-spaces/:id/ask":               {Controller: "schema", Action: "space_ask", MinRole: 2},
	"GET:/api/v1/schema-scan-tasks/:task_id":           {Controller: "schema", Action: "scan_task_detail", MinRole: 2},
	"POST:/api/v1/schema-scan-tasks/:task_id/pause":    {Controller: "schema", Action: "scan_task_pause", MinRole: 2},
	"POST:/api/v1/schema-scan-tasks/:task_id/resume":   {Controller: "schema", Action: "scan_task_resume", MinRole: 2},
	"POST:/api/v1/schema-scan-tasks/:task_id/cancel":   {Controller: "schema", Action: "scan_task_cancel", MinRole: 2},
	// Git 同步：可写远程地址与凭据、并在服务端真的执行 pull（往磁盘上改文件）
	"GET:/api/v1/git-sync/config":    {Controller: "git_sync", Action: "config_read", MinRole: 2},
	"PUT:/api/v1/git-sync/config":    {Controller: "git_sync", Action: "config_update", MinRole: 2},
	"DELETE:/api/v1/git-sync/config": {Controller: "git_sync", Action: "config_delete", MinRole: 2},
	"POST:/api/v1/git-sync/pull":     {Controller: "git_sync", Action: "pull", MinRole: 2},
	"GET:/api/v1/git-sync/logs":      {Controller: "git_sync", Action: "logs", MinRole: 2},
	// Webhook：URL 由服务端主动发起请求，配上「任何登录用户都能建」就是现成的内网探针
	"GET:/api/v1/webhooks":                {Controller: "webhook", Action: "list", MinRole: 2},
	"GET:/api/v1/webhooks/events":         {Controller: "webhook", Action: "events", MinRole: 2},
	"GET:/api/v1/webhooks/:id":            {Controller: "webhook", Action: "detail", MinRole: 2},
	"POST:/api/v1/webhooks":               {Controller: "webhook", Action: "save", MinRole: 2},
	"PUT:/api/v1/webhooks/:id":            {Controller: "webhook", Action: "modify", MinRole: 2},
	"DELETE:/api/v1/webhooks/:id":         {Controller: "webhook", Action: "delete", MinRole: 2},
	"GET:/api/v1/webhooks/:id/deliveries": {Controller: "webhook", Action: "deliveries", MinRole: 2},
	// 数据源与连接测试：能改 Host/口令并让服务端主动连库，必须管理员
	"GET:/api/v1/datasources":                  {Controller: "datasource", Action: "list", MinRole: 2},
	"POST:/api/v1/datasources":                 {Controller: "datasource", Action: "create", MinRole: 2},
	"POST:/api/v1/datasources/test-connection": {Controller: "datasource", Action: "test_connection", MinRole: 2},
	"PUT:/api/v1/datasources/:id":              {Controller: "datasource", Action: "update", MinRole: 2},
	"DELETE:/api/v1/datasources/:id":           {Controller: "datasource", Action: "delete", MinRole: 2},
	"POST:/api/v1/datasources/:id/sync":        {Controller: "datasource", Action: "sync", MinRole: 2},
	// SQL 历史是「用户自己的」数据：任何登录用户可访问，归属由 Service 层强制校验
	"GET:/api/v1/sql-history":              {Controller: "sql_history", Action: "list", MinRole: 3},
	"GET:/api/v1/sql-history/favorites":    {Controller: "sql_history", Action: "list_favorites", MinRole: 3},
	"GET:/api/v1/sql-history/:id":          {Controller: "sql_history", Action: "detail", MinRole: 3},
	"POST:/api/v1/sql-history/save":        {Controller: "sql_history", Action: "save", MinRole: 3},
	"PUT:/api/v1/sql-history/:id/favorite": {Controller: "sql_history", Action: "favorite", MinRole: 3},
	"PUT:/api/v1/sql-history/:id/rate":     {Controller: "sql_history", Action: "rate", MinRole: 3},
	"DELETE:/api/v1/sql-history/:id":       {Controller: "sql_history", Action: "delete", MinRole: 3},
}

// privilegeRule 是 PrivilegeMap 的预编译形式：按特异性从高到低排序后线性匹配，
// 保证同一请求永远命中同一条规则。
type privilegeRule struct {
	method  string
	pattern string
	priv    struct {
		Controller string
		Action     string
		MinRole    int
	}
}

var (
	privilegeRulesOnce sync.Once
	privilegeRules     []privilegeRule
)

// routeSpecificity 参数段越少越具体；段数相同时路径越长越具体。
func routeSpecificity(pattern string) int {
	parts := strings.Split(pattern, "/")
	params := 0
	for _, p := range parts {
		if strings.HasPrefix(p, ":") {
			params++
		}
	}
	return (len(parts)-params)*100000 - params*1000 + len(pattern)
}

func compiledPrivilegeRules() []privilegeRule {
	privilegeRulesOnce.Do(func() {
		rules := make([]privilegeRule, 0, len(PrivilegeMap))
		for key, priv := range PrivilegeMap {
			method, pattern, ok := strings.Cut(key, ":")
			if !ok {
				continue
			}
			r := privilegeRule{method: method, pattern: pattern}
			r.priv.Controller = priv.Controller
			r.priv.Action = priv.Action
			r.priv.MinRole = priv.MinRole
			rules = append(rules, r)
		}
		sort.Slice(rules, func(i, j int) bool {
			si, sj := routeSpecificity(rules[i].pattern), routeSpecificity(rules[j].pattern)
			if si != sj {
				return si > sj
			}
			return rules[i].pattern < rules[j].pattern
		})
		privilegeRules = rules
	})
	return privilegeRules
}

// matchRoutePattern 检查实际请求路径是否匹配路由模式
// pattern: /api/v1/users/:id  actual: /api/v1/users/123
func matchRoutePattern(pattern, actual string) bool {
	patternParts := strings.Split(pattern, "/")
	actualParts := strings.Split(actual, "/")

	if len(patternParts) != len(actualParts) {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		// 如果是参数占位符（:id, :space_id 等），匹配任意非空值
		if strings.HasPrefix(patternParts[i], ":") {
			if actualParts[i] == "" {
				return false
			}
			continue
		}
		// 否则必须精确匹配
		if patternParts[i] != actualParts[i] {
			return false
		}
	}
	return true
}

// adminPathPrefixes 需要管理员权限的路径前缀（用于 PrivilegeMap 未覆盖的管理路由默认拒绝）
var adminPathPrefixes = []string{
	"/api/v1/emails",
	"/api/v1/configs",
	"/api/v1/auth-sso",
	"/api/v1/links",
	"/api/v1/plugins",
	"/api/v1/contacts",
	// P0 修复：管理类路由必须纳入 RBAC，避免任意登录用户越权
	"/api/v1/stats",
	"/api/v1/backups",
	"/api/v1/logs",
	"/api/v1/datasources",
	// SQL 历史按用户归属控制（PrivilegeMap 中显式定义为 MinRole 3），不当作管理员前缀
	"/api/v1/auth/oidc/admin", // OIDC 提供方管理：可自建 IdP 并指定默认角色，必须管理员
	"/api/v1/open/api-keys",   // API Key 管理（创建/列表/吊销）需管理员；/open/me 与 /open/export 走 ApiKeyAuth 不受影响
	"/api/v1/ai/config",
	"/api/v1/ai/config/schedule",
	"/api/v1/ai/cache",
	"/api/v1/ai/analysis",
	"/api/v1/ai/vector",
	"/api/v1/ai/doc",
	"/api/v1/ai/import",
	"/api/v1/ai/ragas",
	"/api/v1/ai/auto-tag",
	// 曾有 "/api/v1/ai/merge" —— 匹配不到任何注册路由（真实路径是 /ai/analysis/merge/:id，
	// 已被上面的 /api/v1/ai/analysis 覆盖）。留着只会让人以为那一段还在被兜住。
	// AI 管理子模块（与 routes.go 中 AutoRBAC() 路由对齐）
	"/api/v1/ai/data-query/rules",
	"/api/v1/ai/data-query/relations",
	"/api/v1/ai/data-query/refresh",
	"/api/v1/ai/data-query/spaces", // POST scan/scan-rag 需要 admin；GET tables 子路由由 Service 层权限控制
	"/api/v1/ai/data-query/prompts",
	"/api/v1/ai/data-query/examples", // POST/PUT/DELETE 需 admin；GET 子路由由 Service 层放行
	"/api/v1/ai/data-query/traces",
	"/api/v1/ai/data-query/prompt-versions",
	"/api/v1/ai/data-query/ab-experiments",
	"/api/v1/ai/workflows",
	"/api/v1/ai/mcp",
	"/api/v1/ai/rag",
	"/api/v1/ai/ragas",
	"/api/v1/ai/analyzer",
	"/api/v1/ai/semantic",
	"/api/v1/ai/eval",
	"/api/v1/ai/sql-audit",
	"/api/v1/ai/sql-security-policy",
	"/api/v1/ai/data-sensitivity",
}

// AutoRBAC 自动 RBAC 中间件
// 策略：对 PrivilegeMap 中定义的路径做模式匹配鉴权；
// 对管理路由前缀但未在 PrivilegeMap 中定义的路径默认拒绝；
// 其余放行（由 Service 层空间权限控制）
func AutoRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// 模式匹配：按特异性排序后取第一条命中规则。
		// 旧实现直接遍历 map 并 break，Go 的 map 迭代顺序每轮随机，
		// 当两条模式能匹配同一路径（如 /ai/analysis/clusters 与 /ai/analysis/:id）时
		// 权限判定会在「需要管理员」和「任意登录用户」之间随机漂移。
		var priv struct {
			Controller string
			Action     string
			MinRole    int
		}
		var defined bool

		for _, rule := range compiledPrivilegeRules() {
			if rule.method != method {
				continue
			}
			if matchRoutePattern(rule.pattern, path) {
				priv = rule.priv
				defined = true
				break
			}
		}

		// 未定义路由：检查是否属于管理路径前缀
		// 若匹配管理前缀，视为需要管理员权限（MinRole=2），走正常身份校验；
		// 否则放行（由 Service 层空间权限控制）
		if !defined {
			for _, prefix := range adminPathPrefixes {
				if strings.HasPrefix(path, prefix) {
					priv.MinRole = 2
					defined = true
					break
				}
			}
			if !defined {
				c.Next()
				return
			}
		}

		userID, exists := c.Get("user_id")
		if !exists {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}
		uid, ok := userID.(int)
		if !ok {
			response.Unauthorized(c, "unauthorized, please login first")
			c.Abort()
			return
		}

		// 超级管理员拥有所有权限（使用缓存）
		cacheKey := "rbac:super:" + strconv.Itoa(uid)
		var isSuper bool
		if cached, ok := getFromCache(cacheKey); ok {
			isSuper = cached.(bool)
		} else {
			var err error
			isSuper, err = roleRepo.IsSuperAdmin(uid)
			if err != nil {
				response.ServerError(c, "failed to check role")
				c.Abort()
				return
			}
			setCache(cacheKey, isSuper)
		}
		if isSuper {
			c.Next()
			return
		}

		// 角色等级校验
		if priv.MinRole == 1 {
			response.Forbidden(c, "super admin privileges required")
			c.Abort()
			return
		}
		if priv.MinRole == 2 {
			adminCacheKey := "rbac:admin:" + strconv.Itoa(uid)
			var isAdmin bool
			if cached, ok := getFromCache(adminCacheKey); ok {
				isAdmin = cached.(bool)
			} else {
				var err error
				isAdmin, err = roleRepo.IsAdmin(uid)
				if err != nil {
					response.ServerError(c, "failed to check role")
					c.Abort()
					return
				}
				setCache(adminCacheKey, isAdmin)
			}
			if !isAdmin {
				response.Forbidden(c, "admin privileges required for this action")
				c.Abort()
				return
			}
		}

		// 具体权限校验：仅当 Controller/Action 已定义时（管理路径前缀路由跳过）
		if priv.Controller == "" || priv.Action == "" {
			c.Next()
			return
		}

		privCacheKey := "rbac:priv:" + strconv.Itoa(uid) + ":" + priv.Controller + ":" + priv.Action
		var hasPriv bool
		if cached, ok := getFromCache(privCacheKey); ok {
			hasPriv = cached.(bool)
		} else {
			var err error
			hasPriv, err = roleRepo.HasPrivilege(uid, priv.Controller, priv.Action)
			if err != nil || !hasPriv {
				setCache(privCacheKey, false)
				response.Forbidden(c, "you do not have permission to perform this action")
				c.Abort()
				return
			}
			setCache(privCacheKey, true)
		}
		if !hasPriv {
			response.Forbidden(c, "you do not have permission to perform this action")
			c.Abort()
			return
		}

		c.Next()
	}
}
