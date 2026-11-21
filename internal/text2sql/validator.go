package text2sql

import (
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ai/rag"
)

type SQLValidator struct {
	kb *KnowledgeBase
}

func NewSQLValidator(kb *KnowledgeBase) *SQLValidator {
	return &SQLValidator{kb: kb}
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func (v *SQLValidator) Validate(sql string, input *RecognizedInput) *ValidationResult {
	result := &ValidationResult{Valid: true}

	v.checkSyntax(sql, result)
	v.checkTables(input.Tables, result)
	v.checkFields(input, result)
	v.checkPerformance(sql, result)
	v.checkPermissions(sql, result)

	return result
}

func (v *SQLValidator) checkSyntax(sql string, result *ValidationResult) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "SQL语句为空")
		return
	}

	// 用 AST 校验替代字符串包含检查：旧实现用 strings.Contains 判定危险关键字，
	// 表名/列名含 DROP/ALTER 等子串（如 alter_log、is_delete）即被误拒；
	// 括号计数也不区分字符串字面量，WHERE v='(' 会被误判不匹配。
	if err := rag.ValidateSQLAST(sql); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("SQL语法校验失败: %v", err))
	}
}

func (v *SQLValidator) checkTables(tables []string, result *ValidationResult) {
	for _, table := range tables {
		if v.kb.GetTable(table) == nil {
			// 大小写不敏感二次检查
			lowerTable := strings.ToLower(table)
			found := false
			for name := range v.kb.GetAllTables() {
				if strings.ToLower(name) == lowerTable {
					found = true
					break
				}
			}
			if !found {
				result.Warnings = append(result.Warnings, fmt.Sprintf("表 %s 不在知识库中，请先通过「Schema导入」功能导入该表的DDL，或调用 /api/v1/text2sql/scan-docs 扫描文档中的DDL", table))
			}
		}
	}
}

func (v *SQLValidator) checkFields(input *RecognizedInput, result *ValidationResult) {
	for table, fields := range input.Fields {
		t := v.kb.GetTable(table)
		if t == nil {
			continue
		}

		validFields := make(map[string]bool)
		for _, col := range t.ColumnsJSON {
			validFields[col.Name] = true
		}

		for _, f := range fields {
			if !validFields[f] {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("字段 %s 可能不属于表 %s", f, table))
			}
		}
	}
}

func (v *SQLValidator) checkPerformance(sql string, result *ValidationResult) {
	upperSQL := strings.ToUpper(sql)

	if strings.Contains(upperSQL, "SELECT *") {
		result.Warnings = append(result.Warnings, "建议明确指定查询字段，避免使用 SELECT *")
	}

	if strings.Contains(upperSQL, "WHERE") && !strings.Contains(upperSQL, "LIMIT") {
		result.Warnings = append(result.Warnings, "建议添加 LIMIT 限制返回行数")
	}
}

func (v *SQLValidator) checkPermissions(sql string, result *ValidationResult) {
	upperSQL := strings.ToUpper(sql)

	writeKeywords := []string{"INSERT", "UPDATE", "DELETE"}
	for _, kw := range writeKeywords {
		if strings.HasPrefix(strings.TrimSpace(upperSQL), kw) {
			result.Warnings = append(result.Warnings,
				"这是写操作，请确认有相应权限")
			break
		}
	}
}
