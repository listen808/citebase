package trust

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// PolicyEngine SQL 执行安全策略引擎
type PolicyEngine struct {
	repo *airepo.SQLSecurityPolicyRepo
}

func NewPolicyEngine(repo *airepo.SQLSecurityPolicyRepo) *PolicyEngine {
	return &PolicyEngine{repo: repo}
}

// PolicyCheckResult 策略校验结果
type PolicyCheckResult struct {
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason,omitempty"`
	MaxRows    int    `json:"max_rows"`
	TimeoutSec int    `json:"timeout_sec"`
}

// Check 校验 SQL 是否符合空间安全策略
func (e *PolicyEngine) Check(ctx context.Context, spaceID uint64, sqlStr string, tokenUsed int) (*PolicyCheckResult, error) {
	policy, err := e.getEffectivePolicy(spaceID)
	if err != nil {
		return nil, err
	}

	result := &PolicyCheckResult{
		Allowed:    true,
		MaxRows:    policy.MaxRows,
		TimeoutSec: policy.TimeoutSec,
	}

	upper := strings.ToUpper(sqlStr)

	// 1. 语句类型检查
	if !policy.AllowDML && isDML(upper) {
		result.Allowed = false
		result.Reason = "安全策略禁止 DML 操作（INSERT/UPDATE/DELETE）"
		return result, nil
	}
	if !policy.AllowDDL && isDDL(upper) {
		result.Allowed = false
		result.Reason = "安全策略禁止 DDL 操作（CREATE/ALTER/DROP/TRUNCATE）"
		return result, nil
	}

	// 2. 黑名单检查：策略自带的规则 **加上** 底线规则。
	// 底线不参与"管理员清空 DenyPatterns 就失效"这条路径 ——
	// 那一栏是纯文本框，清空它不该等于把系统表与 OUTFILE 敞开着，
	// 因为 Text2SQL 拿的是主业务库连接。
	deny := policy.DenyPatterns
	if strings.TrimSpace(deny) == "" {
		deny = DefaultDenyPatterns
	} else {
		deny = deny + "\n" + DefaultDenyPatterns
	}
	if matched, pattern := e.matchPatterns(sqlStr, deny); matched {
		result.Allowed = false
		result.Reason = fmt.Sprintf("SQL 命中黑名单规则: %s", pattern)
		return result, nil
	}

	// 3. 白名单检查（若配置）
	if strings.TrimSpace(policy.AllowPatterns) != "" {
		if matched, _ := e.matchPatterns(sqlStr, policy.AllowPatterns); !matched {
			result.Allowed = false
			result.Reason = "SQL 未命中任何白名单规则"
			return result, nil
		}
	}

	// 4. Token 成本上限
	if policy.MaxCostTokens > 0 && tokenUsed > policy.MaxCostTokens {
		result.Allowed = false
		result.Reason = fmt.Sprintf("Token 消耗 %d 超过策略上限 %d", tokenUsed, policy.MaxCostTokens)
		return result, nil
	}

	return result, nil
}

// GetPolicy 获取空间生效策略（含全局默认回退）
func (e *PolicyEngine) GetPolicy(spaceID uint64) (*aimodel.SQLSecurityPolicy, error) {
	return e.getEffectivePolicy(spaceID)
}

func (e *PolicyEngine) getEffectivePolicy(spaceID uint64) (*aimodel.SQLSecurityPolicy, error) {
	if e.repo == nil {
		return defaultPolicy(), nil
	}
	policy, err := e.repo.GetEffectivePolicy(spaceID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return defaultPolicy(), nil
	}
	return policy, nil
}

// DefaultDenyPatterns 是没有策略行时也要成立的那一份底线。
//
// 它存在的理由：defaultPolicy() 此前只给 MaxRows/TimeoutSec，DenyPatterns 是空串，
// 而 matchPatterns 对空串一条都不匹配 —— 于是"这个空间还没配 SQL 安全策略"
// 实际含义是"没有任何黑名单"，information_schema、mysql.user、OUTFILE 全都能查。
// 策略行是可以由管理端删掉的，所以"没有行"这一档必须有底线，不能等于放行。
//
// 格式与策略表里的 DenyPatterns 一致：一行一条正则，由 matchPatterns 逐条编译。
var DefaultDenyPatterns = strings.Join([]string{
	`(?i)\binformation_schema\b`,
	`(?i)\bmysql\.user\b`,
	`(?i)\bperformance_schema\b`,
	`(?i)\bsys\.`,
	`(?i)\binto\s+(out|dump)file\b`,
	`(?i)\bload_file\s*\(`,
	`(?i)\bload\s+data\b`,
	`(?i)\b(sleep|benchmark)\s*\(`,
	`(?i)\bgrant\b|\brevoke\b`,
	`(?i)\bpg_shadow\b|\bpg_authid\b|\bpg_catalog\b`,
	`(?i)\bsqlite_master\b|\bsqlite_temp_master\b`,
}, "\n")

func defaultPolicy() *aimodel.SQLSecurityPolicy {
	return &aimodel.SQLSecurityPolicy{
		SpaceID:      0,
		MaxRows:      200,
		TimeoutSec:   15,
		DenyPatterns: DefaultDenyPatterns,
	}
}

func (e *PolicyEngine) matchPatterns(sqlStr, patterns string) (bool, string) {
	for _, line := range strings.Split(patterns, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		re, err := regexp.Compile(line)
		if err != nil {
			// 无效正则跳过但记录日志，避免防护静默丢失且管理员无感知
			logx.Infof("[PolicyEngine] 无效正则模式 %q 被跳过: %v", line, err)
			continue
		}
		if re.MatchString(sqlStr) {
			return true, line
		}
	}
	return false, ""
}

func isDML(upperSQL string) bool {
	return regexp.MustCompile(`\b(INSERT|UPDATE|DELETE|MERGE|REPLACE)\b`).MatchString(upperSQL)
}

func isDDL(upperSQL string) bool {
	return regexp.MustCompile(`\b(CREATE|ALTER|DROP|TRUNCATE|RENAME)\b`).MatchString(upperSQL)
}

// ApplyLimit 已删除（P1 首要任务 2 / 清单 129）：
// 旧实现用 strings.Contains(upper,"LIMIT") 判定"已有 LIMIT"，列名/字面量/注释里
// 出现 "limit" 子串即可绕过策略行数上限。全仓无调用者；行数钳制的唯一正确实现是
// text2sql.injectLimit（AST 改写、min(已有, 策略值)），不再保留一个可被绕过的
// 同构字符串实现，误导后续调用方。

// ToDuration 秒转 duration
func ToDuration(sec int) time.Duration {
	if sec <= 0 {
		return 15 * time.Second
	}
	return time.Duration(sec) * time.Second
}
