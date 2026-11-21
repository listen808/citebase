package dialect

import (
	"strconv"
	"strings"
)

// Dialect 数据库方言接口
type Dialect interface {
	Name() string
	QuoteIdentifier(name string) string
	QuoteString(s string) string
	Paginate(sql string, offset, limit int) string
	LimitOnly(sql string, limit int) string
	TypeMapping(goType string) string
	Functions() map[string]string
	SupportsReturning() bool
	Placeholder(index int) string
}

// ===== MySQL =====

// MySQLDialect MySQL 方言
type MySQLDialect struct{}

func (d *MySQLDialect) Name() string { return "mysql" }

func (d *MySQLDialect) QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func (d *MySQLDialect) QuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (d *MySQLDialect) Paginate(sql string, offset, limit int) string {
	return sql + " LIMIT " + strconv.Itoa(offset) + ", " + strconv.Itoa(limit)
}

func (d *MySQLDialect) LimitOnly(sql string, limit int) string {
	return sql + " LIMIT " + strconv.Itoa(limit)
}

func (d *MySQLDialect) TypeMapping(goType string) string {
	switch goType {
	case "string":
		return "VARCHAR(255)"
	case "int":
		return "INT"
	case "int64":
		return "BIGINT"
	case "float64":
		return "DOUBLE"
	case "bool":
		return "TINYINT(1)"
	case "time.Time":
		return "DATETIME"
	default:
		return "TEXT"
	}
}

func (d *MySQLDialect) Functions() map[string]string {
	return map[string]string{
		"now":          "NOW()",
		"current_date": "CURDATE()",
		"concat":       "CONCAT",
		"length":       "LENGTH",
		"lower":        "LOWER",
		"upper":        "UPPER",
	}
}

func (d *MySQLDialect) SupportsReturning() bool { return false }

func (d *MySQLDialect) Placeholder(index int) string { return "?" }

// ===== PostgreSQL =====

// PostgresDialect PostgreSQL 方言
type PostgresDialect struct{}

func (d *PostgresDialect) Name() string { return "postgres" }

func (d *PostgresDialect) QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (d *PostgresDialect) QuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (d *PostgresDialect) Paginate(sql string, offset, limit int) string {
	return sql + " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

func (d *PostgresDialect) LimitOnly(sql string, limit int) string {
	return sql + " LIMIT " + strconv.Itoa(limit)
}

func (d *PostgresDialect) TypeMapping(goType string) string {
	switch goType {
	case "string":
		return "VARCHAR(255)"
	case "int":
		return "INTEGER"
	case "int64":
		return "BIGINT"
	case "float64":
		return "DOUBLE PRECISION"
	case "bool":
		return "BOOLEAN"
	case "time.Time":
		return "TIMESTAMP"
	default:
		return "TEXT"
	}
}

func (d *PostgresDialect) Functions() map[string]string {
	return map[string]string{
		"now":          "NOW()",
		"current_date": "CURRENT_DATE",
		"concat":       "CONCAT",
		"length":       "LENGTH",
		"lower":        "LOWER",
		"upper":        "UPPER",
	}
}

func (d *PostgresDialect) SupportsReturning() bool { return true }

func (d *PostgresDialect) Placeholder(index int) string {
	return "$" + strconv.Itoa(index)
}

// ===== SQLite =====

// SQLiteDialect SQLite 方言
type SQLiteDialect struct{}

func (d *SQLiteDialect) Name() string { return "sqlite" }

func (d *SQLiteDialect) QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (d *SQLiteDialect) QuoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (d *SQLiteDialect) Paginate(sql string, offset, limit int) string {
	return sql + " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

func (d *SQLiteDialect) LimitOnly(sql string, limit int) string {
	return sql + " LIMIT " + strconv.Itoa(limit)
}

func (d *SQLiteDialect) TypeMapping(goType string) string {
	switch goType {
	case "string":
		return "TEXT"
	case "int", "int64":
		return "INTEGER"
	case "float64":
		return "REAL"
	case "bool":
		return "INTEGER"
	case "time.Time":
		return "TEXT"
	default:
		return "TEXT"
	}
}

func (d *SQLiteDialect) Functions() map[string]string {
	return map[string]string{
		"now":          "datetime('now')",
		"current_date": "date('now')",
		"concat":       "",
		"length":       "LENGTH",
		"lower":        "LOWER",
		"upper":        "UPPER",
	}
}

func (d *SQLiteDialect) SupportsReturning() bool { return true }

func (d *SQLiteDialect) Placeholder(index int) string { return "?" }

// ===== Registry =====

// DialectRegistry 方言注册表
type DialectRegistry struct {
	dialects map[string]Dialect
}

// NewDialectRegistry 创建方言注册表
func NewDialectRegistry() *DialectRegistry {
	r := &DialectRegistry{dialects: make(map[string]Dialect)}
	r.Register(&MySQLDialect{})
	r.Register(&PostgresDialect{})
	r.Register(&SQLiteDialect{})
	return r
}

// Register 注册方言
func (r *DialectRegistry) Register(d Dialect) {
	r.dialects[d.Name()] = d
}

// Get 获取方言
func (r *DialectRegistry) Get(name string) (Dialect, bool) {
	d, ok := r.dialects[strings.ToLower(name)]
	return d, ok
}

// List 列出所有方言名
func (r *DialectRegistry) List() []string {
	names := make([]string, 0, len(r.dialects))
	for name := range r.dialects {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry 默认注册表
var DefaultRegistry = NewDialectRegistry()

// GetDialect 根据名称获取方言（默认 mysql）
func GetDialect(name string) Dialect {
	if d, ok := DefaultRegistry.Get(name); ok {
		return d
	}
	return &MySQLDialect{}
}
