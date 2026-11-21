package text2sql

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	"github.com/xwb1989/sqlparser"
)

// MaskingEngine 字段级脱敏引擎
type MaskingEngine struct {
	repo DataSensitivityRepoInterface
}

type DataSensitivityRepoInterface interface {
	ListBySpace(spaceID uint64) ([]*aimodel.DataSensitivityRule, error)
}

func NewMaskingEngine(repo DataSensitivityRepoInterface) *MaskingEngine {
	return &MaskingEngine{repo: repo}
}

// MaskResult 对 SQL 执行结果按敏感度规则脱敏。
//
// sqlStr 为本次实际下发给数据源的 SQL（可能已注入 LIMIT / RLS 条件），用于把结果集的
// 输出列名还原成 SELECT 列表里的原始表达式，避免以下两类绕过：
//   - 列别名：`SELECT phone AS p` 只按输出列名匹配 → 旧实现漏脱敏；
//   - 函数/CAST 包裹：`SELECT CONCAT(phone, 常量)` / `CAST(phone AS char)` → 旧实现漏脱敏。
//
// P0 安全修复（fail-closed）：规则仓储未配置、规则加载失败、SQL 无法解析出 SELECT 列表时
// 一律返回错误，由执行器拒绝返回结果集；旧实现把错误吞掉后继续放行明文数据。
func (m *MaskingEngine) MaskResult(spaceID uint64, result *ExecResult, sqlStr string) error {
	if result == nil || len(result.Rows) == 0 {
		return nil
	}
	if m.repo == nil {
		return fmt.Errorf("脱敏规则仓储未配置，无法确认敏感字段")
	}

	rules, err := m.repo.ListBySpace(spaceID)
	if err != nil {
		return fmt.Errorf("加载脱敏规则失败: %w", err)
	}
	if len(rules) == 0 {
		// 规则加载成功且该空间确实未声明敏感字段：无脱敏义务
		return nil
	}

	// 输出列名 -> SELECT 列表中的原始表达式（小写）
	exprs, err := selectColumnExpressions(sqlStr)
	if err != nil {
		return err
	}

	// 按 "表.字段" 或 "字段" 建立规则索引
	ruleMap := make(map[string]*aimodel.DataSensitivityRule)
	for _, r := range rules {
		if r.Table != "" {
			ruleMap[strings.ToLower(r.Table+"."+r.Field)] = r
		}
		ruleMap[strings.ToLower(r.Field)] = r
	}
	// 表达式内出现受保护列名即需脱敏（覆盖函数/CAST 包裹）
	type fieldRule struct {
		re   *regexp.Regexp
		rule *aimodel.DataSensitivityRule
	}
	var fieldRules []fieldRule
	for _, r := range rules {
		field := strings.TrimSpace(r.Field)
		if field == "" {
			continue
		}
		re, cerr := regexp.Compile(`(?i)(^|[^a-z0-9_$])` + regexp.QuoteMeta(field) + `([^a-z0-9_$]|$)`)
		if cerr != nil {
			return fmt.Errorf("脱敏规则字段 %s 无法编译: %w", field, cerr)
		}
		fieldRules = append(fieldRules, fieldRule{re: re, rule: r})
	}

	// 对每一列，找到匹配规则并脱敏
	for colIdx, col := range result.Columns {
		colKey := strings.ToLower(strings.TrimSpace(col))
		exprKey := colKey
		if e, ok := exprs[colKey]; ok {
			exprKey = e
		}
		rule, ok := ruleMap[exprKey]
		if !ok {
			rule, ok = ruleMap[colKey]
		}
		if !ok {
			// 表达式中出现受保护字段（CONCAT(phone,'')、d.phone 等）即脱敏
			for _, fr := range fieldRules {
				if fr.re.MatchString(exprKey) {
					rule, ok = fr.rule, true
					break
				}
			}
		}
		if !ok {
			continue
		}
		for _, row := range result.Rows {
			if val, exists := row[col]; exists {
				row[col] = applyMask(val, rule)
			} else if val, exists := row[result.Columns[colIdx]]; exists {
				// 处理大小写差异
				row[col] = applyMask(val, rule)
			}
		}
	}

	return nil
}

// selectColumnExpressions 解析顶层 SELECT 列表，返回 输出列名(小写) -> 原始表达式(小写)。
// `SELECT *` 没有可还原的表达式，结果集列名本身就是一列名，按列名匹配即可。
// 解析失败/结构不是 SELECT 时返回错误（fail-closed）。
func selectColumnExpressions(sqlStr string) (map[string]string, error) {
	trimmed := strings.TrimSpace(sqlStr)
	if trimmed == "" {
		return nil, fmt.Errorf("SQL 为空，无法还原输出列的原始表达式")
	}
	stmt, err := sqlparser.Parse(strings.TrimSuffix(trimmed, ";"))
	if err != nil {
		return nil, fmt.Errorf("无法解析 SQL 以还原列表达式: %w", err)
	}
	var sel *sqlparser.Select
	switch s := stmt.(type) {
	case *sqlparser.Select:
		sel = s
	case *sqlparser.Union:
		if left, ok := s.Left.(*sqlparser.Select); ok {
			sel = left
		}
	case *sqlparser.ParenSelect:
		if inner, ok := s.Select.(*sqlparser.Select); ok {
			sel = inner
		}
	}
	if sel == nil {
		return nil, fmt.Errorf("SQL 顶层结构不是 SELECT，无法还原输出列表达式")
	}

	exprs := make(map[string]string)
	for _, se := range sel.SelectExprs {
		item, ok := se.(*sqlparser.AliasedExpr)
		if !ok {
			continue // StarExpr：输出列名即真实列名
		}
		exprText := strings.ToLower(sqlparser.String(item.Expr))
		name := strings.ToLower(item.As.Lowered())
		if name == "" {
			name = exprText
		}
		exprs[name] = exprText
	}
	return exprs, nil
}

// DetectPII 简单 PII 检测：手机号、身份证、邮箱、银行卡
func DetectPII(text string) []string {
	patterns := []struct {
		name    string
		pattern string
	}{
		{"mobile", `1[3-9]\d{9}`},
		{"id_card", `\d{17}[\dXx]|\d{15}`},
		{"email", `[\w.+-]+@[\w-]+\.[\w.-]+`},
		{"bank_card", `\d{16,19}`},
	}
	found := []string{}
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		matches := re.FindAllString(text, -1)
		for _, m := range matches {
			// 简单过滤：身份证需要 15/18 位，银行卡需要 16-19 位
			if p.name == "id_card" && !(len(m) == 15 || len(m) == 18) {
				continue
			}
			if p.name == "bank_card" && (len(m) < 16 || len(m) > 19) {
				continue
			}
			found = append(found, fmt.Sprintf("%s:%s", p.name, maskPartial(m)))
		}
	}
	return found
}

func applyMask(val interface{}, rule *aimodel.DataSensitivityRule) interface{} {
	s, ok := val.(string)
	if !ok {
		return val
	}

	switch rule.MaskType {
	case "full":
		return strings.Repeat("*", utf8.RuneCountInString(s))
	case "partial":
		return maskPartial(s)
	case "hash":
		h := sha256.Sum256([]byte(s))
		return fmt.Sprintf("sha256:%x", h[:8])
	case "regex":
		if rule.MaskRule != "" {
			re, err := regexp.Compile(rule.MaskRule)
			if err == nil {
				return re.ReplaceAllString(s, "***")
			}
		}
		return maskPartial(s)
	default:
		// 默认根据级别脱敏
		switch rule.Level {
		case aimodel.SensitivityCritical, aimodel.SensitivityHigh:
			return strings.Repeat("*", utf8.RuneCountInString(s))
		case aimodel.SensitivityMedium:
			return maskPartial(s)
		default:
			return s
		}
	}
}

// maskPartial 保留首尾，中间用 * 替代
func maskPartial(s string) string {
	runes := []rune(s)
	n := len(runes)
	if n <= 2 {
		return strings.Repeat("*", n)
	}
	if n <= 6 {
		return string(runes[0]) + strings.Repeat("*", n-2) + string(runes[n-1])
	}
	show := n / 4
	if show < 1 {
		show = 1
	}
	return string(runes[:show]) + strings.Repeat("*", n-show*2) + string(runes[n-show:])
}
