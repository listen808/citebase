package text2sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// ExecutionPlanInfo 执行计划信息
type ExecutionPlanInfo struct {
	Driver     string                   // 数据库驱动
	Rows       []map[string]interface{} // EXPLAIN 原始行
	Warnings   []string                 // 潜在问题
	ScoreDelta float64                  // 评分调整（-1 ~ +1，负数表示扣分）
}

// ExecutionPlanAnalyzer 执行计划分析器
// P1-2 (§4.2.4): 用 EXPLAIN 评估候选 SQL 的性能，作为评分维度
type ExecutionPlanAnalyzer struct {
	executor *SQLExecutor
}

// NewExecutionPlanAnalyzer 创建执行计划分析器
func NewExecutionPlanAnalyzer(executor *SQLExecutor) *ExecutionPlanAnalyzer {
	return &ExecutionPlanAnalyzer{executor: executor}
}

// AnalyzeOnSpace 在指定空间的数据源上执行 EXPLAIN 并分析
func (a *ExecutionPlanAnalyzer) AnalyzeOnSpace(sqlStr string, spaceID uint64) *ExecutionPlanInfo {
	if a == nil || a.executor == nil || a.executor.dsRepo == nil {
		return nil
	}
	if !isReadOnlyQuery(sqlStr) {
		return &ExecutionPlanInfo{Warnings: []string{"非 SELECT 查询，跳过执行计划分析"}}
	}

	dsList, err := a.executor.dsRepo.FindActiveBySpace(spaceID)
	if err != nil || len(dsList) == 0 {
		return nil
	}
	return a.analyzeOnDatasource(sqlStr, &dsList[0])
}

// analyzeOnDatasource 在指定数据源上执行 EXPLAIN
func (a *ExecutionPlanAnalyzer) analyzeOnDatasource(sqlStr string, ds *modelai.DataSource) *ExecutionPlanInfo {
	driver := "mysql"
	if ds.DBType == "postgres" || ds.DBType == "postgresql" {
		driver = "postgres"
	} else if ds.DBType == "oracle" {
		driver = "oracle"
	}

	dsn := buildDSN(ds)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		logx.Infof("[ExecPlan] Open db failed: %v", err)
		return nil
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(15 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		logx.Infof("[ExecPlan] Ping failed: %v", err)
		return nil
	}

	// 构造 EXPLAIN 语句
	explainSQL := buildExplainSQL(driver, sqlStr)

	rows, err := db.QueryContext(ctx, explainSQL)
	if err != nil {
		logx.Infof("[ExecPlan] EXPLAIN failed: %v", err)
		return nil
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	if cols == nil {
		cols = []string{}
	}

	var resultRows []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range cols {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		resultRows = append(resultRows, row)
	}

	info := &ExecutionPlanInfo{
		Driver: driver,
		Rows:   resultRows,
	}

	// 分析执行计划
	a.analyzeMySQL(info, resultRows)
	return info
}

// buildExplainSQL 构造 EXPLAIN 语句
func buildExplainSQL(driver, sqlStr string) string {
	switch driver {
	case "postgres":
		// 用 EXPLAIN 而非 EXPLAIN ANALYZE：后者会真实执行查询（全表扫描/聚合），
		// 在评分链路上可能对无 LIMIT 的重查询做完整执行，造成性能放大。
		return "EXPLAIN " + sqlStr
	case "oracle":
		return "EXPLAIN PLAN FOR " + sqlStr
	default: // mysql
		return "EXPLAIN " + sqlStr
	}
}

// analyzeMySQL 分析 MySQL 执行计划
func (a *ExecutionPlanAnalyzer) analyzeMySQL(info *ExecutionPlanInfo, rows []map[string]interface{}) {
	var scoreDelta float64 = 0
	var warnings []string

	for _, row := range rows {
		// MySQL EXPLAIN 关键字段：type, rows, Extra, key
		accessType, _ := row["type"].(string)
		tableName, _ := row["table"].(string)
		key, _ := row["key"].(string)
		extra, _ := row["Extra"].(string)

		// 访问类型评分
		switch strings.ToLower(accessType) {
		case "all":
			// 全表扫描
			scoreDelta -= 0.2
			warnings = append(warnings, fmt.Sprintf("表 %s 全表扫描（type=ALL）", tableName))
		case "index":
			// 全索引扫描
			scoreDelta -= 0.05
		case "range", "ref", "eq_ref", "const", "system":
			// 使用索引
			if key != "" {
				scoreDelta += 0.1
			}
		}

		// 扫描行数评分
		if rowsVal, ok := row["rows"]; ok {
			var scannedRows int64
			switch v := rowsVal.(type) {
			case int64:
				scannedRows = v
			case int:
				scannedRows = int64(v)
			case float64:
				scannedRows = int64(v)
			}
			if scannedRows > 100000 {
				scoreDelta -= 0.15
				warnings = append(warnings, fmt.Sprintf("表 %s 预估扫描 %d 行", tableName, scannedRows))
			} else if scannedRows > 10000 {
				scoreDelta -= 0.1
				warnings = append(warnings, fmt.Sprintf("表 %s 预估扫描 %d 行", tableName, scannedRows))
			}
		}

		// Using filesort / temporary
		if strings.Contains(strings.ToLower(extra), "using filesort") {
			scoreDelta -= 0.05
			warnings = append(warnings, fmt.Sprintf("表 %s 使用文件排序（filesort）", tableName))
		}
		if strings.Contains(strings.ToLower(extra), "using temporary") {
			scoreDelta -= 0.05
			warnings = append(warnings, fmt.Sprintf("表 %s 使用临时表（temporary）", tableName))
		}
	}

	// 限制在 [-1, +1]
	if scoreDelta > 1 {
		scoreDelta = 1
	}
	if scoreDelta < -1 {
		scoreDelta = -1
	}

	info.ScoreDelta = scoreDelta
	info.Warnings = warnings
}

// ScoreForCandidate 为候选 SQL 计算执行计划评分
// 返回 (scoreDelta, warnings)
func (a *ExecutionPlanAnalyzer) ScoreForCandidate(sqlStr string, spaceID uint64) (float64, []string) {
	if a == nil {
		return 0, nil
	}
	info := a.AnalyzeOnSpace(sqlStr, spaceID)
	if info == nil {
		return 0, nil
	}
	return info.ScoreDelta, info.Warnings
}
