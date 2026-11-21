package text2sql

import (
	"context"
	"database/sql"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/listen808/citebase/internal/ai/rag"
	modelai "github.com/listen808/citebase/internal/model/ai"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/text2sql/trust"
	_ "github.com/sijms/go-ora/v2"
	"github.com/xwb1989/sqlparser"
)

type SQLExecutor struct {
	dsRepo    *airepo.DataSourceRepo
	masking   *MaskingEngine
	policy    *trust.PolicyEngine
	auditor   *trust.SQLAuditRecorder
	rls       *trust.RowLevelSecurity
	spaceRepo spaceAccessRepo
	maxRows   int
	timeout   time.Duration
}

// spaceAccessRepo 执行链归属校验所需的最小空间仓储能力。
// 这里刻意使用 repository 层而不是 internal/service/workspace：
// service 层依赖链更重且部分模块反向依赖 text2sql，容易引入 import 循环。
type spaceAccessRepo interface {
	FindByIDCached(id int) (*workspacemodel.Space, error)
	FindSpaceUserCached(spaceID, userID int) (*workspacemodel.SpaceUser, error)
}

type ExecResult struct {
	Success  bool                     `json:"success"`
	Columns  []string                 `json:"columns,omitempty"`
	Rows     []map[string]interface{} `json:"rows,omitempty"`
	Error    string                   `json:"error,omitempty"`
	RowCount int                      `json:"row_count"`
	Duration int                      `json:"duration_ms"`
}

func NewSQLExecutor(dsRepo *airepo.DataSourceRepo) *SQLExecutor {
	cfg := GetText2SQLConfig()
	return &SQLExecutor{
		dsRepo:  dsRepo,
		maxRows: cfg.ExecutionMaxRows,
		timeout: time.Duration(cfg.ExecutionTimeoutSec) * time.Second,
	}
}

func (e *SQLExecutor) SetMaskingEngine(m *MaskingEngine) {
	e.masking = m
}

func (e *SQLExecutor) SetPolicyEngine(p *trust.PolicyEngine) {
	e.policy = p
}

func (e *SQLExecutor) SetAuditor(a *trust.SQLAuditRecorder) {
	e.auditor = a
}

// SetRLS 注入行级安全过滤器（P0-10）
// 启用后，调用 ExecuteOnSpaceWithUser 的 SQL 会被注入 WHERE 条件，
// 限制用户只能访问有权限的空间数据
func (e *SQLExecutor) SetRLS(r *trust.RowLevelSecurity) {
	e.rls = r
}

// SetSpaceRepo 注入空间归属校验仓储（P0 安全修复）
// 未注入时按需回退到 workspacerepo.NewSpaceRepository()
func (e *SQLExecutor) SetSpaceRepo(r spaceAccessRepo) {
	e.spaceRepo = r
}

// ExecuteOnSpace 不带调用者身份的执行入口。
//
// P0 安全修复：该入口已无法执行任何 SQL —— 执行链必须携带真实 userID，
// 否则无法做归属校验、RLS 与审计。请改用 ExecuteOnSpaceWithUser，
// 或在持有请求 ctx 时改用 ExecuteWithContext（身份沿 ctx 透传）。
func (e *SQLExecutor) ExecuteOnSpace(sqlStr string, spaceID uint64) *ExecResult {
	return e.ExecuteOnSpaceWithUser(sqlStr, spaceID, 0)
}

// ExecuteWithContext 从请求上下文解析调用者身份后执行（P1 首要任务 1）。
//
// 生成链上的自动试执行/自纠错/候选验证过去以 userID=0 调 ExecuteOnSpace，
// 被 P0 的 fail-closed 守卫拒绝后退化为“生成期不验证”。本入口沿 ctx 取真实
// userID（provider.ContextWithUserID / WithRequestScope 两条写入路径都兼容），
// 恢复验证能力的同时不把身份存放在共享单例字段上（避免跨请求借用身份）。
// 上下文无身份时仍然拒绝执行（fail-closed 语义不变）。
func (e *SQLExecutor) ExecuteWithContext(ctx context.Context, sqlStr string, spaceID uint64) *ExecResult {
	return e.ExecuteOnSpaceWithUser(sqlStr, spaceID, UserIDFromContext(ctx))
}

// AuthorizeSpaceAccess 暴露执行链的归属校验给请求入口（如 StreamHandler）：
// 在加载空间知识库/生成之前先判定调用者是否可见该空间，失败即拒绝。
func (e *SQLExecutor) AuthorizeSpaceAccess(spaceID uint64, userID int) error {
	return e.authorizeSpaceAccess(spaceID, userID)
}

// ExecuteOnSpaceWithUser 在指定空间执行 SQL。
//
// P0 安全修复：
//  1. userID <= 0（无调用者身份）直接拒绝，不再“匿名但放行”；
//  2. 校验 userID 是否真的属于 spaceID（旧实现完全信任调用方传入的 spaceID，
//     任意登录用户换个 spaceID 就能读别人空间的数据源）；
//  3. 校验失败绝不执行；
//  4. userID 有效时才注入 RLS 行级过滤。
func (e *SQLExecutor) ExecuteOnSpaceWithUser(sqlStr string, spaceID uint64, userID int) *ExecResult {
	if err := e.authorizeSpaceAccess(spaceID, userID); err != nil {
		return &ExecResult{Error: err.Error()}
	}

	if !isReadOnlyQuery(sqlStr) {
		return &ExecResult{Error: "安全限制：仅允许 SELECT 查询"}
	}

	policyResult, err := e.checkPolicy(spaceID, sqlStr, 0)
	if err != nil {
		return &ExecResult{Error: fmt.Sprintf("策略检查失败: %v", err)}
	}
	if !policyResult.Allowed {
		return &ExecResult{Error: fmt.Sprintf("SQL 被安全策略拦截: %s", policyResult.Reason)}
	}

	maxRows := e.effectiveMaxRows(policyResult.MaxRows)
	limited, err := injectLimit(sqlStr, maxRows)
	if err != nil {
		return &ExecResult{Error: fmt.Sprintf("行数上限校验失败: %v", err)}
	}
	sqlStr = limited
	timeout := e.effectiveTimeout(trust.ToDuration(policyResult.TimeoutSec))

	dsList, err := e.dsRepo.FindActiveBySpace(spaceID)
	if err != nil || len(dsList) == 0 {
		return &ExecResult{Error: "该空间未配置数据源连接"}
	}

	// RLS 注入：用户无权限空间的数据被过滤掉（authorizeSpaceAccess 已保证 userID > 0）
	if e.rls != nil {
		filtered, rlsErr := e.rls.InjectRowLevelFilter(context.Background(), sqlStr, userID, int(spaceID), nil)
		if rlsErr != nil {
			return &ExecResult{Error: fmt.Sprintf("RLS 注入失败: %v", rlsErr)}
		}
		sqlStr = filtered
	}

	return e.executeOnDatasourceWithTimeout(sqlStr, &dsList[0], maxRows, timeout)
}

// authorizeSpaceAccess 执行链上的调用者归属校验（两个执行入口共用）
//
// 旧实现里执行链上一个归属校验都没有：facade 只绑 req.SpaceID/req.SQL，路由只挂
// AuthMiddleware，任何登录用户都能带任意 spaceID 去执行 SQL（横向越权），
// 项目里已有的 CheckSpacePrivilege / GetUserSpacePrivilege 能力在执行路径上零调用。
func (e *SQLExecutor) authorizeSpaceAccess(spaceID uint64, userID int) error {
	if userID <= 0 {
		return fmt.Errorf("缺少调用者身份，拒绝执行：Text2SQL 执行链必须携带真实 userID")
	}
	if spaceID == 0 {
		return fmt.Errorf("缺少目标空间 ID，拒绝执行：无法校验数据归属")
	}
	repo := e.spaceRepoOrNil()
	if repo == nil {
		return fmt.Errorf("空间权限仓储未初始化，无法校验调用者归属，拒绝执行")
	}

	space, err := repo.FindByIDCached(int(spaceID))
	if err != nil || space == nil {
		return fmt.Errorf("空间 %d 不存在或不可用，拒绝执行", spaceID)
	}

	// 与 internal/service/workspace 的 CheckSpaceAccess 保持一致的可见性语义：
	// visit_level 0/public = 所有人可见，1/login = 登录用户可见，其余 = 仅空间成员。
	if space.IsPublic() || space.IsLoggedInVisible() {
		return nil
	}

	if _, err := repo.FindSpaceUserCached(int(spaceID), userID); err != nil {
		// 与 internal/service/workspace 的 CheckSpaceAccess 一致：管理员对所有空间可见
		if e.isAdminUser(userID) {
			return nil
		}
		return fmt.Errorf("用户 %d 不属于空间 %d，拒绝执行越权查询", userID, spaceID)
	}
	return nil
}

// isAdminUser 判断是否平台管理员/超管（仅在成员校验失败后作为兜旁路；查询失败按非管理员处理）
func (e *SQLExecutor) isAdminUser(userID int) bool {
	if repos.DB == nil || userID <= 0 {
		return false
	}
	isAdmin, err := userrepo.NewRoleRepository().IsAdmin(userID)
	return err == nil && isAdmin
}

// spaceRepoOrNil 延迟获取空间仓储；DB 尚未初始化时返回 nil（调用方 fail-closed）
func (e *SQLExecutor) spaceRepoOrNil() spaceAccessRepo {
	if e.spaceRepo != nil {
		return e.spaceRepo
	}
	if repos.DB == nil {
		return nil
	}
	return workspacerepo.NewSpaceRepository()
}

// effectiveMaxRows 把策略行数上限钳制到全局配置硬上限
func (e *SQLExecutor) effectiveMaxRows(policyMaxRows int) int {
	hard := e.maxRows
	if hard <= 0 {
		hard = GetText2SQLConfig().ExecutionMaxRows
	}
	if hard <= 0 {
		hard = 200
	}
	if policyMaxRows <= 0 || policyMaxRows > hard {
		return hard
	}
	return policyMaxRows
}

// effectiveTimeout 把策略超时钳制到全局配置上限，并保留兜底超时
func (e *SQLExecutor) effectiveTimeout(policyTimeout time.Duration) time.Duration {
	if e.timeout > 0 && (policyTimeout <= 0 || policyTimeout > e.timeout) {
		return e.timeout
	}
	if policyTimeout <= 0 {
		return 15 * time.Second
	}
	return policyTimeout
}

func (e *SQLExecutor) ExecuteOnDatasource(sqlStr string, dsID uint64) *ExecResult {
	// P0 安全修复：本入口没有调用者身份参数，无法做归属校验，一律拒绝执行；
	// 需要按数据源执行的调用方必须改为携带 userID 的 ExecuteOnSpaceWithUser。
	if err := e.authorizeSpaceAccess(0, 0); err != nil {
		return &ExecResult{Error: err.Error()}
	}

	if !isReadOnlyQuery(sqlStr) {
		return &ExecResult{Error: "安全限制：仅允许 SELECT 查询"}
	}

	ds, err := e.dsRepo.FindByID(dsID)
	if err != nil {
		return &ExecResult{Error: fmt.Sprintf("数据源不存在: %v", err)}
	}

	policyResult, err := e.checkPolicy(ds.SpaceID, sqlStr, 0)
	if err != nil {
		return &ExecResult{Error: fmt.Sprintf("策略检查失败: %v", err)}
	}
	if !policyResult.Allowed {
		return &ExecResult{Error: fmt.Sprintf("SQL 被安全策略拦截: %s", policyResult.Reason)}
	}

	maxRows := e.effectiveMaxRows(policyResult.MaxRows)
	limited, err := injectLimit(sqlStr, maxRows)
	if err != nil {
		return &ExecResult{Error: fmt.Sprintf("行数上限校验失败: %v", err)}
	}
	sqlStr = limited
	timeout := e.effectiveTimeout(trust.ToDuration(policyResult.TimeoutSec))

	return e.executeOnDatasourceWithTimeout(sqlStr, ds, maxRows, timeout)
}

func (e *SQLExecutor) checkPolicy(spaceID uint64, sqlStr string, tokenUsed int) (*trust.PolicyCheckResult, error) {
	if e.policy == nil {
		return &trust.PolicyCheckResult{Allowed: true, MaxRows: e.maxRows, TimeoutSec: int(e.timeout.Seconds())}, nil
	}
	return e.policy.Check(context.Background(), spaceID, sqlStr, tokenUsed)
}

func (e *SQLExecutor) executeOnDatasource(sqlStr string, ds *modelai.DataSource) *ExecResult {
	return e.executeOnDatasourceWithTimeout(sqlStr, ds, e.effectiveMaxRows(0), e.timeout)
}

// executeOnDatasourceWithTimeout 真正下发 SQL 的地方。
// maxRows 为本次执行的硬行数上限（已经过 effectiveMaxRows 钳制到配置值）。
func (e *SQLExecutor) executeOnDatasourceWithTimeout(sqlStr string, ds *modelai.DataSource, maxRows int, timeout time.Duration) *ExecResult {
	if maxRows <= 0 {
		maxRows = e.effectiveMaxRows(0)
	}
	dsn := buildDSN(ds)
	driver := "mysql"
	if ds.DBType == "postgres" || ds.DBType == "postgresql" {
		driver = "postgres"
	} else if ds.DBType == "oracle" {
		driver = "oracle"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return &ExecResult{Error: fmt.Sprintf("创建连接失败: %v", err)}
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Second)

	// 驱动层超时兜底：QueryContext 取消后 rows 迭代同样会中断
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return &ExecResult{Error: fmt.Sprintf("连接数据库失败: %v", err)}
	}

	var rows *sql.Rows
	start := time.Now()
	tx, txErr := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if txErr != nil {
		// MySQL驱动可能不支持ReadOnly，fallback到普通查询
		// P1：降级不再是静默的——只读事务这层兜底未生效必须留痕，
		// 运维可通过驱动/账号配置（只读账号）消除该告警
		logx.Infof("[SQLExecutor] readonly transaction unsupported (%v), fallback to plain query; recommend configuring a read-only DB account", txErr)
		rows, err = db.QueryContext(ctx, sqlStr)
	} else {
		defer func() { _ = tx.Rollback() }()
		rows, err = tx.QueryContext(ctx, sqlStr)
	}
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return &ExecResult{
			Error:    formatSQLError(err),
			Duration: int(duration),
		}
	}
	defer rows.Close()

	// P0 安全修复：列信息读取失败必须报错，不能像旧实现那样吞掉 error 继续往下走，
	// 否则会把“截断/失败的结果集”伪装成 Success=true 返回给调用方。
	cols, err := rows.Columns()
	if err != nil {
		return &ExecResult{
			Error:    fmt.Sprintf("读取结果列信息失败: %v", err),
			Duration: int(duration),
		}
	}
	if cols == nil {
		cols = []string{}
	}

	var results []map[string]interface{}
	var scanErrs []string
	scanErrCount := 0
	scanCount := 0
	for rows.Next() {
		if scanCount >= maxRows {
			break
		}
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range cols {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			// 单行 Scan 失败不再静默 continue：计数并在最终结果里如实反馈
			scanErrCount++
			if len(scanErrs) < 5 {
				scanErrs = append(scanErrs, formatSQLError(err))
			}
			scanCount++
			continue
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
		scanCount++
	}

	// 迭代本身被中断（超时/连接断开）同样不能算成功
	if err := rows.Err(); err != nil {
		return &ExecResult{
			Error:    fmt.Sprintf("结果集读取中断: %v", err),
			Duration: int(duration),
		}
	}

	result := &ExecResult{
		Success:  scanErrCount == 0,
		Columns:  cols,
		Rows:     results,
		RowCount: len(results),
		Duration: int(duration),
	}
	if scanErrCount > 0 {
		// 允许返回已读到的部分结果，但必须标记失败并带上错误信息
		result.Error = fmt.Sprintf("结果集有 %d 行读取失败: %s", scanErrCount, strings.Join(scanErrs, "; "))
		return result
	}

	// 应用字段级脱敏
	// P0 安全修复：脱敏失败/规则不可得时拒绝返回结果集（旧实现只打日志后放行，fail-open）
	if e.masking != nil && ds != nil {
		if err := e.masking.MaskResult(ds.SpaceID, result, sqlStr); err != nil {
			logx.Infof("[SQLExecutor] masking failed, result rejected: %v", err)
			return &ExecResult{
				Error:    fmt.Sprintf("字段脱敏处理失败，已拒绝返回结果集: %v", err),
				Duration: int(duration),
			}
		}
	}

	return result
}

func buildDSN(ds *modelai.DataSource) string {
	switch ds.DBType {
	case "postgres", "postgresql":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=10",
			ds.Host, ds.Port, ds.Username, ds.Password, ds.DBName)
	case "oracle":
		return fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
			ds.Username, ds.Password, ds.Host, ds.Port, ds.DBName)
	default:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&timeout=10s",
			ds.Username, ds.Password, ds.Host, ds.Port, ds.DBName)
	}
}

func isReadOnlyQuery(sqlStr string) bool {
	return rag.ValidateSQLAST(sqlStr) == nil
}

// injectLimit 保证顶层 SELECT 带上不超过 maxRows 的 LIMIT。
//
// 旧实现用 `strings.Contains(upper, "LIMIT")` 判断“已有 LIMIT”，于是：
//   - 模型自带 `LIMIT 100000` 直接被原样放行（绕过行数上限）；
//   - 字符串字面量里出现 "limit"（如 `WHERE name LIKE '%limit%'`）会被误判为已有 LIMIT。
//
// 现在用 sqlparser 解析顶层语句（能走到这里的 SQL 已通过 rag.ValidateSQLAST 同一套解析器）：
//   - 顶层无 LIMIT        -> 追加 `LIMIT maxRows`；
//   - 顶层 LIMIT 未超上限 -> 原样保留；
//   - 顶层 LIMIT 超过上限 -> 通过 AST 把 Rowcount 钳制为 maxRows 后重新渲染；
//   - 解析失败 / 结构或 LIMIT 值无法可靠判定 -> 返回 error，调用方拒绝执行（fail-closed）。
func injectLimit(sqlStr string, maxRows int) (string, error) {
	if maxRows <= 0 {
		return "", fmt.Errorf("行数上限无效（%d），拒绝执行", maxRows)
	}

	trimmed := strings.TrimSpace(sqlStr)
	trailingSemi := strings.HasSuffix(trimmed, ";")
	body := strings.TrimSuffix(trimmed, ";")

	stmt, err := sqlparser.Parse(body)
	if err != nil {
		return "", fmt.Errorf("无法解析 SQL 以判定 LIMIT 上限: %w", err)
	}
	for {
		ps, ok := stmt.(*sqlparser.ParenSelect)
		if !ok {
			break
		}
		stmt = ps.Select
	}

	var limit *sqlparser.Limit
	switch sel := stmt.(type) {
	case *sqlparser.Select:
		limit = sel.Limit
	case *sqlparser.Union:
		limit = sel.Limit
	default:
		return "", fmt.Errorf("不支持的顶层语句结构 %T，无法判定行数上限", stmt)
	}

	if limit == nil {
		out := body + fmt.Sprintf(" LIMIT %d", maxRows)
		if trailingSemi {
			out += ";"
		}
		return out, nil
	}

	current, err := limitRowCount(limit.Rowcount)
	if err != nil {
		return "", fmt.Errorf("已有 LIMIT 的行数无法判定: %w", err)
	}
	if current <= maxRows {
		return sqlStr, nil
	}
	limit.Rowcount = sqlparser.NewIntVal([]byte(strconv.Itoa(maxRows)))
	return sqlparser.String(stmt), nil
}

// limitRowCount 读取 LIMIT 子句的行数（仅接受数字字面量，绑定参数等一律视为不可判定）
func limitRowCount(expr sqlparser.Expr) (int, error) {
	val, ok := expr.(*sqlparser.SQLVal)
	if !ok || val == nil || val.Type != sqlparser.IntVal {
		return 0, fmt.Errorf("LIMIT 值不是数字字面量（%T）", expr)
	}
	n, err := strconv.Atoi(string(val.Val))
	if err != nil {
		return 0, fmt.Errorf("LIMIT 值无法解析为行数: %w", err)
	}
	if n < 0 {
		return 0, fmt.Errorf("LIMIT 值为负数: %d", n)
	}
	return n, nil
}

func formatSQLError(err error) string {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return msg
}
