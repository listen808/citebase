package text2sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/log"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"gorm.io/gorm"
)

// SchemaStatsCollector 列级统计采集器
// 旧实现把数据源缓存在 c.ds 字段且从不失效：本采集器是 SchemaService 上的长生命周期单例，
// 第一次为空间 A 采集后，空间 B/C 的统计会被拿到 A 的数据源上执行，并以 B 的 SpaceID 落库
// （跨空间读到别的库的表统计/样本值，且归属记错）；该字段读写也无同步，并发采集构成数据竞争。
// 现按 spaceID 每次解析数据源，作为局部变量沿调用链传递，不再缓存在结构体上。
type SchemaStatsCollector struct {
	db        *gorm.DB
	statsRepo *airepo.ColumnStatsRepo
	dsRepo    *airepo.DataSourceRepo
}

// NewSchemaStatsCollector 创建统计采集器
func NewSchemaStatsCollector(db *gorm.DB, statsRepo *airepo.ColumnStatsRepo, dsRepo *airepo.DataSourceRepo) *SchemaStatsCollector {
	return &SchemaStatsCollector{
		db:        db,
		statsRepo: statsRepo,
		dsRepo:    dsRepo,
	}
}

// resolveDataSource 按空间解析当前生效的数据源（不缓存，保证空间隔离与配置即时生效）
func (c *SchemaStatsCollector) resolveDataSource(spaceID uint64) (*aimodel.DataSource, error) {
	dsList, err := c.dsRepo.FindActiveBySpace(spaceID)
	if err != nil || len(dsList) == 0 {
		return nil, fmt.Errorf("no datasource found for space %d", spaceID)
	}
	return &dsList[0], nil
}

// ident 给表名/列名加引号，并把"根本不该当标识符用"的输入挡在前面。
//
// 这三条统计 SQL 都是 fmt.Sprintf 拼出来的（行数、列分布、TOP 样本），
// 而 tableName / col.Name 是从外部进来的：表名来自扫描结果（用户可在管理页改名、
// 也可能来自被导入的 DDL），列名同理。原写法把它们裸拼进去，
// 一个带反引号的"表名"就能把这条 SELECT 变成任意 SQL —— 而且它是**用数据源配的
// 那套凭据**执行的，等于从统计采集这一步拿到对端库的执行权。
//
// 引号规则取自 MySQL 方言（双写反引号），和 dialect.MySQLDialect 一致；
// 控制字符与空名不是"引号能包住的东西"，直接拒。
func ident(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("标识符为空")
	}
	if strings.ContainsRune(trimmed, 0) || strings.ContainsAny(trimmed, "\n\r\t") {
		return "", fmt.Errorf("标识符 %q 含控制字符，拒绝执行统计查询", name)
	}
	return "`" + strings.ReplaceAll(trimmed, "`", "``") + "`", nil
}

// CollectForTable 采集指定表的列统计
func (c *SchemaStatsCollector) CollectForTable(spaceID uint64, tableName string, columns []ColumnInfo) error {
	ds, err := c.resolveDataSource(spaceID)
	if err != nil {
		return err
	}

	fullTableIdent, err := ident(tableName)
	if err != nil {
		return err
	}
	fullTableName := fullTableIdent

	// 采集行数
	var rowCount int64
	countResult := c.executeSQL(ds, fmt.Sprintf("SELECT COUNT(*) AS cnt FROM %s", fullTableName))
	if countResult != nil && len(countResult) > 0 {
		if v, ok := countResult[0]["cnt"].(int64); ok {
			rowCount = v
		} else if v2, ok := countResult[0]["cnt"].(float64); ok {
			rowCount = int64(v2)
		}
	}

	for _, col := range columns {
		colIdent, err := ident(col.Name)
		if err != nil {
			return err
		}
		stats := &aimodel.ColumnStats{
			SpaceID:    spaceID,
			TableNam:   tableName,
			ColumnName: col.Name,
			DataType:   col.Type,
			RowCount:   rowCount,
		}

		// 采集数值/日期列的统计
		if !isTextType(col.Type) && !isEnumType(col.Type) {
			statsQuery := fmt.Sprintf(
				"SELECT COUNT(*) AS total, COUNT(DISTINCT %s) AS dist, "+
					"MIN(%s) AS min_val, MAX(%s) AS max_val, AVG(%s) AS avg_val "+
					"FROM %s",
				colIdent, colIdent, colIdent, colIdent, fullTableName)
			rows := c.executeSQL(ds, statsQuery)
			if len(rows) > 0 {
				if v, ok := rows[0]["total"].(int64); ok {
					stats.NullCount = rowCount - v
				} else if v2, ok := rows[0]["total"].(float64); ok {
					stats.NullCount = rowCount - int64(v2)
				}
				if v, ok := rows[0]["dist"].(int64); ok {
					stats.DistinctCount = v
				} else if v2, ok := rows[0]["dist"].(float64); ok {
					stats.DistinctCount = int64(v2)
				}
				if v, ok := rows[0]["min_val"].(string); ok {
					stats.MinValue = v
				}
				if v, ok := rows[0]["max_val"].(string); ok {
					stats.MaxValue = v
				}
				if v, ok := rows[0]["avg_val"].(string); ok {
					stats.AvgValue = v
				}
			}
		}

		// 采集枚举/字符串列的高频值
		if isEnumType(col.Type) || isTextType(col.Type) {
			samples, _ := c.collectTopSamples(ds, fullTableName, col.Name, 10)
			if len(samples) > 0 {
				b, _ := json.Marshal(samples)
				stats.SampleValues = string(b)
			}
		}

		if err := c.statsRepo.Upsert(stats); err != nil {
			log.Errorf("[SchemaStatsCollector] failed to upsert stats for %s.%s: %v\n", tableName, col.Name, err)
		}
	}

	return nil
}

// executeSQL 在指定数据源上执行 SQL 并返回行列表（ds 由调用方按空间解析后传入，不再走结构体字段）
func (c *SchemaStatsCollector) executeSQL(ds *aimodel.DataSource, query string) []map[string]interface{} {
	if ds == nil {
		return nil
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
		log.Errorf("[SchemaStatsCollector] sql.Open failed: %v", err)
		return nil
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		log.Errorf("[SchemaStatsCollector] query failed: %v", err)
		return nil
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	if cols == nil {
		cols = []string{}
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range cols {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			log.Errorf("[SchemaStatsCollector] scan failed: %v", err)
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
	}
	if err := rows.Err(); err != nil {
		log.Errorf("[SchemaStatsCollector] rows iteration error: %v", err)
	}

	return results
}

func (c *SchemaStatsCollector) collectTopSamples(ds *aimodel.DataSource, table, column string, limit int) ([]string, error) {
	colIdent, err := ident(column)
	if err != nil {
		return nil, err
	}
	tableIdent, err := ident(table)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	query := fmt.Sprintf(
		"SELECT %s AS val, COUNT(*) AS cnt FROM %s WHERE %s IS NOT NULL AND %s != '' "+
			"GROUP BY %s ORDER BY cnt DESC LIMIT %d",
		colIdent, tableIdent, colIdent, colIdent, colIdent, limit)
	rows := c.executeSQL(ds, query)
	if len(rows) == 0 {
		return nil, fmt.Errorf("no samples")
	}
	var samples []string
	for _, r := range rows {
		if v, ok := r["val"].(string); ok {
			samples = append(samples, v)
		}
	}
	return samples, nil
}

func isTextType(t string) bool {
	t = strings.ToLower(t)
	return strings.Contains(t, "text") || strings.Contains(t, "varchar") ||
		strings.Contains(t, "char") || strings.Contains(t, "json")
}

func isEnumType(t string) bool {
	t = strings.ToLower(t)
	return strings.Contains(t, "enum") || strings.Contains(t, "set")
}
