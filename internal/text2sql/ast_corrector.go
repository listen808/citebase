package text2sql

import (
	"strings"
)

// ASTCorrector AST 修正器
type ASTCorrector struct {
	rules []*CorrectionRule
}

// CorrectionRule 修正规则
type CorrectionRule struct {
	Name         string
	ErrorPattern string
	Correct      func(sql, errMsg string, availableFields map[string][]string) *CorrectionResult
}

// CorrectionResult 修正结果
type CorrectionResult struct {
	OriginalSQL  string
	CorrectedSQL string
	Corrected    bool
	Suggestion   string
}

// NewASTCorrector 创建 AST 修正器
func NewASTCorrector() *ASTCorrector {
	c := &ASTCorrector{
		rules: make([]*CorrectionRule, 0),
	}
	c.registerBuiltinRules()
	return c
}

// registerBuiltinRules 注册内置修正规则
func (c *ASTCorrector) registerBuiltinRules() {
	// 规则1: Unknown column - 字段不存在
	c.rules = append(c.rules, &CorrectionRule{
		Name:         "unknown_column",
		ErrorPattern: "Unknown column",
		Correct: func(sql, errMsg string, availableFields map[string][]string) *CorrectionResult {
			// 提取错误中的字段名 (例如: "Unknown column 'e.salary'")
			fieldName := extractFieldName(errMsg)
			if fieldName == "" {
				return &CorrectionResult{
					OriginalSQL:  sql,
					CorrectedSQL: sql,
					Corrected:    false,
				}
			}

			// 在所有可用字段中模糊匹配
			for table, fields := range availableFields {
				match := fuzzyMatch(fieldName, fields)
				if match != "" {
					// 替换字段名
					correctedSQL := strings.ReplaceAll(sql, fieldName, match)
					return &CorrectionResult{
						OriginalSQL:  sql,
						CorrectedSQL: correctedSQL,
						Corrected:    true,
						Suggestion:   "字段 " + fieldName + " 不存在，已替换为 " + match + " (来自表 " + table + ")",
					}
				}
			}

			return &CorrectionResult{
				OriginalSQL:  sql,
				CorrectedSQL: sql,
				Corrected:    false,
				Suggestion:   "字段 " + fieldName + " 不存在，未找到相似字段",
			}
		},
	})

	// 规则2: Ambiguous column - 字段歧义
	c.rules = append(c.rules, &CorrectionRule{
		Name:         "ambiguous_column",
		ErrorPattern: "ambiguous",
		Correct: func(sql, errMsg string, availableFields map[string][]string) *CorrectionResult {
			// 提取错误中的字段名 (例如: "Column 'name' is ambiguous")
			fieldName := extractFieldName(errMsg)
			if fieldName == "" {
				return &CorrectionResult{
					OriginalSQL:  sql,
					CorrectedSQL: sql,
					Corrected:    false,
				}
			}

			// 找到第一个包含该字段的表，添加表前缀
			for table, fields := range availableFields {
				for _, f := range fields {
					if f == fieldName {
						// 添加表前缀
						correctedSQL := strings.ReplaceAll(sql, fieldName, table+"."+fieldName)
						return &CorrectionResult{
							OriginalSQL:  sql,
							CorrectedSQL: correctedSQL,
							Corrected:    true,
							Suggestion:   "字段 " + fieldName + " 有歧义，已添加表前缀 " + table + "." + fieldName,
						}
					}
				}
			}

			return &CorrectionResult{
				OriginalSQL:  sql,
				CorrectedSQL: sql,
				Corrected:    false,
				Suggestion:   "字段 " + fieldName + " 有歧义，未找到对应表",
			}
		},
	})

	// 规则3: Table doesn't exist - 表不存在
	c.rules = append(c.rules, &CorrectionRule{
		Name:         "unknown_table",
		ErrorPattern: "doesn't exist",
		Correct: func(sql, errMsg string, availableFields map[string][]string) *CorrectionResult {
			// 提取错误中的表名 (例如: "Table 'emp' doesn't exist")
			tableName := extractTableName(errMsg)
			if tableName == "" {
				return &CorrectionResult{
					OriginalSQL:  sql,
					CorrectedSQL: sql,
					Corrected:    false,
				}
			}

			// 在可用表中模糊匹配
			availableTables := make([]string, 0, len(availableFields))
			for t := range availableFields {
				availableTables = append(availableTables, t)
			}
			match := fuzzyMatch(tableName, availableTables)
			if match != "" {
				// 替换表名
				correctedSQL := strings.ReplaceAll(sql, tableName, match)
				return &CorrectionResult{
					OriginalSQL:  sql,
					CorrectedSQL: correctedSQL,
					Corrected:    true,
					Suggestion:   "表 " + tableName + " 不存在，已替换为 " + match,
				}
			}

			return &CorrectionResult{
				OriginalSQL:  sql,
				CorrectedSQL: sql,
				Corrected:    false,
				Suggestion:   "表 " + tableName + " 不存在，未找到相似表",
			}
		},
	})
}

// Correct 尝试修正 SQL
func (c *ASTCorrector) Correct(sql, errMsg string, availableFields map[string][]string) *CorrectionResult {
	if errMsg == "" {
		return &CorrectionResult{
			OriginalSQL:  sql,
			CorrectedSQL: sql,
			Corrected:    false,
		}
	}

	// 遍历规则，找到匹配的规则
	for _, rule := range c.rules {
		if strings.Contains(errMsg, rule.ErrorPattern) {
			return rule.Correct(sql, errMsg, availableFields)
		}
	}

	// 无匹配规则
	return &CorrectionResult{
		OriginalSQL:  sql,
		CorrectedSQL: sql,
		Corrected:    false,
		Suggestion:   "无匹配的修正规则",
	}
}

// fuzzyMatchField 在字段列表中模糊匹配
func (c *ASTCorrector) fuzzyMatchField(target string, fields []string) string {
	return fuzzyMatch(target, fields)
}

// fuzzyMatch 通用模糊匹配函数
func fuzzyMatch(target string, candidates []string) string {
	target = strings.ToLower(target)
	bestMatch := ""
	bestScore := 0.0

	for _, candidate := range candidates {
		candidateLower := strings.ToLower(candidate)

		// 完全匹配
		if candidateLower == target {
			return candidate
		}

		// 子串匹配（优先级最高）
		if strings.Contains(candidateLower, target) {
			// target 是 candidate 的子串
			score := float64(len(target)) / float64(len(candidateLower))
			if score > bestScore {
				bestScore = score
				bestMatch = candidate
			}
			continue
		}
		if strings.Contains(target, candidateLower) {
			// candidate 是 target 的子串
			score := float64(len(candidateLower)) / float64(len(target)) * 0.9
			if score > bestScore {
				bestScore = score
				bestMatch = candidate
			}
			continue
		}

		// 计算相似度
		score := calculateSimilarity(target, candidateLower)
		if score > bestScore && score > 0.6 {
			bestScore = score
			bestMatch = candidate
		}
	}

	if bestScore > 0.3 {
		return bestMatch
	}
	return ""
}

// calculateSimilarity 计算两个字符串的相似度 (0-1)
// 使用 Levenshtein 编辑距离归一化，比字符集 Jaccard 更准确
func calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}
	dist := levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	return 1.0 - float64(dist)/float64(maxLen)
}

// extractFieldName 从错误信息中提取字段名
func extractFieldName(errMsg string) string {
	// 匹配模式: "Unknown column 'xxx'" 或 "Column 'xxx' is ambiguous"
	start := strings.Index(errMsg, "'")
	if start == -1 {
		return ""
	}
	end := strings.Index(errMsg[start+1:], "'")
	if end == -1 {
		return ""
	}
	return errMsg[start+1 : start+1+end]
}

// extractTableName 从错误信息中提取表名
func extractTableName(errMsg string) string {
	// 匹配模式: "Table 'xxx' doesn't exist"
	start := strings.Index(errMsg, "'")
	if start == -1 {
		return ""
	}
	end := strings.Index(errMsg[start+1:], "'")
	if end == -1 {
		return ""
	}
	return errMsg[start+1 : start+1+end]
}
