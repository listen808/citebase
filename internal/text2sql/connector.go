package text2sql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	logx "github.com/listen808/citebase/internal/log"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

func isValidIdentifier(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return matched
}

type DBConnector interface {
	Connect() error
	Close() error
	FetchAllDDL() ([]*modelai.TableCache, error)
	FetchTableDDL(tableName string) (string, error)
	Ping() error
}

type DBConfig struct {
	DBType   string
	Host     string
	Port     int
	DBName   string
	Username string
	Password string
	Timeout  time.Duration
}

func NewDBConnector(cfg DBConfig) (DBConnector, error) {
	switch cfg.DBType {
	case "mysql":
		return newMySQLConnector(cfg), nil
	case "postgres", "postgresql":
		return newPostgresConnector(cfg), nil
	case "oracle":
		return newOracleConnector(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported db_type: %s", cfg.DBType)
	}
}

type mysqlConnector struct {
	cfg    DBConfig
	db     *sql.DB
	parser *DDLParser
}

func newMySQLConnector(cfg DBConfig) *mysqlConnector {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &mysqlConnector{cfg: cfg, parser: NewDDLParser()}
}

func (c *mysqlConnector) dsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&timeout=%s",
		c.cfg.Username, c.cfg.Password, c.cfg.Host, c.cfg.Port, c.cfg.DBName, c.cfg.Timeout)
}

func (c *mysqlConnector) Connect() error {
	var err error
	c.db, err = sql.Open("mysql", c.dsn())
	if err != nil {
		return fmt.Errorf("mysql open failed: %w", err)
	}
	c.db.SetMaxOpenConns(5)
	c.db.SetMaxIdleConns(2)
	c.db.SetConnMaxLifetime(5 * time.Minute)
	return nil
}

func (c *mysqlConnector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *mysqlConnector) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.db.PingContext(ctx)
}

func (c *mysqlConnector) FetchAllDDL() ([]*modelai.TableCache, error) {
	query := `SELECT TABLE_NAME FROM information_schema.TABLES 
              WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`

	rows, err := c.db.Query(query, c.cfg.DBName)
	if err != nil {
		return nil, fmt.Errorf("query tables failed: %w", err)
	}
	defer rows.Close()

	var tables []*modelai.TableCache
	var skipped []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			skipped = append(skipped, fmt.Sprintf("scan error: %v", err))
			continue
		}
		ddl, err := c.FetchTableDDL(tableName)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", tableName, err))
			continue
		}
		cache, err := c.parser.ParseDDL(ddl)
		if err != nil || cache.TableNam == "" {
			skipped = append(skipped, fmt.Sprintf("%s: parse DDL failed", tableName))
			continue
		}
		cache.DBType = "mysql"
		tables = append(tables, cache)
	}
	if err := rows.Err(); err != nil {
		return tables, fmt.Errorf("rows iteration error: %w", err)
	}
	if len(skipped) > 0 {
		logx.Infof("[mysqlConnector] FetchAllDDL skipped %d tables: %s", len(skipped), strings.Join(skipped, "; "))
	}
	return tables, nil
}

func (c *mysqlConnector) FetchTableDDL(tableName string) (string, error) {
	var createSQL, tableComment string

	safeName := strings.ReplaceAll(tableName, "`", "")
	safeName = strings.ReplaceAll(safeName, ";", "")
	safeName = strings.ReplaceAll(safeName, "'", "")

	showQuery := fmt.Sprintf("SHOW CREATE TABLE `%s`", safeName)
	row := c.db.QueryRow(showQuery)
	var dummy string
	if err := row.Scan(&dummy, &createSQL); err != nil {
		return "", fmt.Errorf("show create table %s failed: %w", tableName, err)
	}

	commentQuery := `SELECT TABLE_COMMENT FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`
	row2 := c.db.QueryRow(commentQuery, c.cfg.DBName, tableName)
	_ = row2.Scan(&tableComment)

	return createSQL, nil
}

type postgresConnector struct {
	cfg    DBConfig
	db     *sql.DB
	parser *DDLParser
}

func newPostgresConnector(cfg DBConfig) *postgresConnector {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &postgresConnector{cfg: cfg, parser: NewDDLParser()}
}

func (c *postgresConnector) dsn() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=%d",
		c.cfg.Host, c.cfg.Port, c.cfg.Username, c.cfg.Password, c.cfg.DBName, int(c.cfg.Timeout.Seconds()))
}

func (c *postgresConnector) Connect() error {
	var err error
	c.db, err = sql.Open("postgres", c.dsn())
	if err != nil {
		return fmt.Errorf("postgres open failed: %w", err)
	}
	c.db.SetMaxOpenConns(5)
	c.db.SetMaxIdleConns(2)
	return nil
}

func (c *postgresConnector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *postgresConnector) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.db.PingContext(ctx)
}

func (c *postgresConnector) FetchAllDDL() ([]*modelai.TableCache, error) {
	query := `SELECT tablename FROM pg_tables 
              WHERE schemaname = 'public' ORDER BY tablename`

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query tables failed: %w", err)
	}
	defer rows.Close()

	var tables []*modelai.TableCache
	var skipped []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			skipped = append(skipped, fmt.Sprintf("scan error: %v", err))
			continue
		}
		ddl, err := c.FetchTableDDL(tableName)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", tableName, err))
			continue
		}
		cache, err := c.parser.ParseDDL(ddl)
		if err != nil || cache.TableNam == "" {
			skipped = append(skipped, fmt.Sprintf("%s: parse DDL failed", tableName))
			continue
		}
		cache.DBType = "postgres"
		tables = append(tables, cache)
	}
	if err := rows.Err(); err != nil {
		return tables, fmt.Errorf("rows iteration error: %w", err)
	}
	if len(skipped) > 0 {
		logx.Infof("[postgresConnector] FetchAllDDL skipped %d tables: %s", len(skipped), strings.Join(skipped, "; "))
	}
	return tables, nil
}

func (c *postgresConnector) FetchTableDDL(tableName string) (string, error) {
	if !isValidIdentifier(tableName) {
		return "", fmt.Errorf("invalid table name: %s", tableName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", tableName))
	colQuery := `SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, c.character_maximum_length,
                 c.numeric_precision, c.numeric_scale, c.udt_name,
                 (SELECT pg_catalog.col_description(c.table_oid::regclass, c.ordinal_position) LIMIT 1) as comment
                 FROM information_schema.columns c
                 WHERE c.table_schema = 'public' AND c.table_name = $1 ORDER BY c.ordinal_position`

	rows, err := c.db.Query(colQuery, tableName)
	if err != nil {
		return "", fmt.Errorf("query columns for %s failed: %w", tableName, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var colName, dataType, isNullable, defaultVal, maxLen, udtName, comment string
		var numPrec, numScale *float64
		if err := rows.Scan(&colName, &dataType, &isNullable, &defaultVal, &maxLen, &numPrec, &numScale, &udtName, &comment); err != nil {
			continue
		}
		colDef := fmt.Sprintf("  %s %s", colName, dataType)
		if defaultVal != "" && defaultVal != "NULL" {
			colDef += fmt.Sprintf(" DEFAULT %s", defaultVal)
		}
		if isNullable == "NO" {
			colDef += " NOT NULL"
		}
		cols = append(cols, colDef)
	}

	pkQuery := `SELECT kcu.column_name FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
                WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'`
	pkRows, _ := c.db.Query(pkQuery, tableName)
	var pkCols []string
	if pkRows != nil {
		defer pkRows.Close()
		for pkRows.Next() {
			var pkCol string
			if pkRows.Scan(&pkCol) == nil {
				pkCols = append(pkCols, pkCol)
			}
		}
	}
	if len(pkCols) > 0 {
		cols = append(cols, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ",")))
	}

	sb.WriteString(strings.Join(cols, ",\n"))
	sb.WriteString("\n);")

	return sb.String(), nil
}

type oracleConnector struct {
	cfg    DBConfig
	db     *sql.DB
	parser *DDLParser
}

func newOracleConnector(cfg DBConfig) *oracleConnector {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &oracleConnector{cfg: cfg, parser: NewDDLParser()}
}

func (c *oracleConnector) Connect() error {
	dsn := fmt.Sprintf("%s/%s@%s:%d/%s",
		c.cfg.Username, c.cfg.Password, c.cfg.Host, c.cfg.Port, c.cfg.DBName)
	var err error
	c.db, err = sql.Open("oracle", dsn)
	if err != nil {
		return fmt.Errorf("oracle open failed: %w", err)
	}
	c.db.SetMaxOpenConns(5)
	c.db.SetMaxIdleConns(2)
	return nil
}

func (c *oracleConnector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *oracleConnector) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.db.PingContext(ctx)
}

func (c *oracleConnector) FetchAllDDL() ([]*modelai.TableCache, error) {
	query := `SELECT table_name FROM user_tables ORDER BY table_name`
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query tables failed: %w", err)
	}
	defer rows.Close()

	var tables []*modelai.TableCache
	var skipped []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			skipped = append(skipped, fmt.Sprintf("scan error: %v", err))
			continue
		}
		ddl, err := c.FetchTableDDL(tableName)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", tableName, err))
			continue
		}
		cache, err := c.parser.ParseDDL(ddl)
		if err != nil || cache.TableNam == "" {
			skipped = append(skipped, fmt.Sprintf("%s: parse DDL failed", tableName))
			continue
		}
		cache.DBType = "oracle"
		tables = append(tables, cache)
	}
	if err := rows.Err(); err != nil {
		return tables, fmt.Errorf("rows iteration error: %w", err)
	}
	if len(skipped) > 0 {
		logx.Infof("[oracleConnector] FetchAllDDL skipped %d tables: %s", len(skipped), strings.Join(skipped, "; "))
	}
	return tables, nil
}

func (c *oracleConnector) FetchTableDDL(tableName string) (string, error) {
	if !isValidIdentifier(tableName) {
		return "", fmt.Errorf("invalid table name: %s", tableName)
	}

	query := fmt.Sprintf("SELECT DBMS_METADATA.GET_DDL('TABLE', '%s') FROM DUAL", tableName) //nolint:gosec // G201: tableName validated by isValidIdentifier
	row := c.db.QueryRow(query)
	var ddl string
	if err := row.Scan(&ddl); err != nil {
		return "", fmt.Errorf("get ddl for %s failed: %w", tableName, err)
	}
	return ddl, nil
}

type DatasourceScanner struct {
	repo    *airepo.DataSourceRepo
	parser  *DDLParser
	chunker *SQLSchemaChunker
}

func NewDatasourceScanner(repo *airepo.DataSourceRepo) *DatasourceScanner {
	return &DatasourceScanner{
		repo:    repo,
		parser:  NewDDLParser(),
		chunker: NewSQLSchemaChunker(),
	}
}

func (s *DatasourceScanner) SyncTables(ds *modelai.DataSource) ([]*modelai.TableCache, error) {
	cfg := DBConfig{
		DBType:   ds.DBType,
		Host:     ds.Host,
		Port:     ds.Port,
		DBName:   ds.DBName,
		Username: ds.Username,
		Password: ds.Password,
		Timeout:  15 * time.Second,
	}

	conn, err := NewDBConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("create connector failed: %w", err)
	}
	defer conn.Close()

	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	tables, err := conn.FetchAllDDL()
	if err != nil {
		return nil, fmt.Errorf("fetch ddl failed: %w", err)
	}

	return tables, nil
}

func (s *DatasourceScanner) TestConnection(ds *modelai.DataSource) error {
	cfg := DBConfig{
		DBType:   ds.DBType,
		Host:     ds.Host,
		Port:     ds.Port,
		DBName:   ds.DBName,
		Username: ds.Username,
		Password: ds.Password,
		Timeout:  5 * time.Second,
	}

	conn, err := NewDBConnector(cfg)
	if err != nil {
		return fmt.Errorf("create connector failed: %w", err)
	}
	defer conn.Close()

	if err := conn.Connect(); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	return conn.Ping()
}
