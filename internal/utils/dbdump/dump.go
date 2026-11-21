// Package dbdump 提供不依赖外部二进制的 MySQL/MariaDB 整库导出与回灌。
//
// 存在的理由：internal/service/admin/backup.go 的备份与恢复原先分别 exec mysqldump 与
// mysql，只装了数据库客户端（没有服务端工具链）的机器上"系统管理 → 备份/恢复"必然失败；
// 同时把口令拼在 `-p<password>` 命令行参数上，会出现在进程列表里。本包用驱动直连，二者都不需要。
//
// 一致性：整库在同一条 REPEATABLE READ + CONSISTENT SNAPSHOT 事务里读取，
// 等价于 mysqldump --single-transaction（仅对事务引擎成立，非事务表会给出警告回调）。
// 回灌侧见 restore.go。
package dbdump

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"

	"github.com/listen808/citebase/internal/brand"
)

// Options 导出选项，零值可用（默认：结构+数据、200 行/条 INSERT、不带 DROP TABLE）。
type Options struct {
	// Tables 限定表名，支持以 * 结尾的前缀通配（如 "aw_*"）；为空表示全库。
	Tables []string
	// SchemaOnly 只导结构；NoCreateInfo 只导数据。两者互斥，由调用方保证。
	SchemaOnly   bool
	NoCreateInfo bool
	// AddDropTable 在每张表的 CREATE 前写 DROP TABLE IF EXISTS。
	// 默认关闭：与 mysqldump 一致，导出文件回灌已有库时应当"补行"而不是"清空"。
	AddDropTable bool
	// Chunk 每条 INSERT 最多多少行，<=0 时取 DefaultChunk。
	Chunk int
	// Schema 覆盖从 DSN 解析出的库名。用于连接账号能看见旁系库、但 DSN 指向另一个库的场景
	// （表名、SHOW CREATE 都已按 schema 限定，不依赖连接的默认库）。为空表示用 DSN 里的。
	Schema string
	// Progress 每完成一张表回调一次（table, rows）；nil 表示不汇报。
	Progress func(table string, rows int)
	// Warn 非致命问题回调（如一致性快照不可用）；nil 表示忽略。
	Warn func(msg string)
}

// Stats 导出结果统计。
type Stats struct {
	Tables int
	Rows   int
	Bytes  int64
}

// DefaultChunk 单条 INSERT 的默认行数上限。
const DefaultChunk = 200

func (o Options) mode() string {
	switch {
	case o.SchemaOnly:
		return "仅结构"
	case o.NoCreateInfo:
		return "仅数据"
	default:
		return "结构+数据"
	}
}

// maxChunkBytes 单条 INSERT 缓冲的字节上限，与按行计数取先到的那个。
const maxChunkBytes = 4 << 20

// defaultMaxPacket 服务端 max_allowed_packet 读不到时的估计值（MySQL 5.7 的默认值）。
const defaultMaxPacket = 4 << 20

// Dump 把 DSN 指向的库导出为可回灌的 SQL 文本。
func Dump(dsn string, w io.Writer, opt Options) (Stats, error) {
	var st Stats
	if dsn == "" {
		return st, errors.New("dbdump: DSN 为空")
	}
	if opt.Chunk <= 0 {
		opt.Chunk = DefaultChunk
	}
	if opt.SchemaOnly && opt.NoCreateInfo {
		return st, errors.New("dbdump: SchemaOnly 与 NoCreateInfo 互斥")
	}
	schema := opt.Schema
	if schema == "" {
		var err error
		if schema, err = SchemaFromDSN(dsn); err != nil {
			return st, err
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return st, fmt.Errorf("dbdump: 打开连接失败: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return st, fmt.Errorf("dbdump: 连接 %s 失败: %w", schema, err)
	}

	warn := func(format string, a ...any) {
		if opt.Warn != nil {
			opt.Warn(fmt.Sprintf(format, a...))
		}
	}

	// 单条 INSERT 不能超过服务端 max_allowed_packet，否则导出文件在回灌时被驱动直接拒收。
	var packet int64
	if err := db.QueryRow("SELECT @@max_allowed_packet").Scan(&packet); err != nil {
		warn("读取 max_allowed_packet 失败，按 %d 字节估计: %v", defaultMaxPacket, err)
	}
	if packet <= 0 {
		packet = defaultMaxPacket
	}
	byteCap := int64(maxChunkBytes)
	if packet*3/4 < byteCap {
		byteCap = packet * 3 / 4
	}
	if byteCap < 64<<10 {
		byteCap = 64 << 10
	}

	ctx := context.Background()
	// 隔离级别必须与事务落在同一条连接上，所以先取定一条连接再开事务；
	// 走 db.Exec + db.Begin 的话，连接池可能给它们两条不同的连接。
	conn, err := db.Conn(ctx)
	if err != nil {
		return st, fmt.Errorf("dbdump: 取连接失败: %w", err)
	}
	defer conn.Close()
	// 用 SESSION 级而不是 TRANSACTION 级：下面 START TRANSACTION 会隐式结束上一个空事务，
	// TRANSACTION 级的设置只对它那一次生效。
	if _, err := conn.ExecContext(ctx, "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		warn("设置隔离级别失败，导出可能不是同一时刻的快照: %v", err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return st, fmt.Errorf("dbdump: 开启事务失败: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // 只读事务，回滚只是收尾
	if _, err := tx.Exec("START TRANSACTION WITH CONSISTENT SNAPSHOT"); err != nil {
		warn("一致性快照不可用（引擎不支持事务？）: %v", err)
	}
	// 表清单按名字排序，父表不一定在前；导出侧与恢复侧都关外键检查才不会卡在顺序上。
	if _, err := tx.Exec("SET SESSION FOREIGN_KEY_CHECKS = 0"); err != nil {
		warn("关闭外键检查失败: %v", err)
	}

	tables, err := listTables(ctx, tx, schema, opt.Tables)
	if err != nil {
		return st, err
	}
	if len(tables) == 0 {
		return st, fmt.Errorf("dbdump: 库 %s 里没有匹配的表", schema)
	}

	cw := &countWriter{w: bufio.NewWriterSize(w, 1<<20)}
	fmt.Fprintf(cw, "-- %s 数据导出（dbdump，不依赖 mysqldump）\n", brand.DefaultName)
	fmt.Fprintf(cw, "-- 源库: %s   时间: %s   表数: %d   模式: %s\n",
		schema, time.Now().Format("2006-01-02 15:04:05 -07:00"), len(tables), opt.mode())
	fmt.Fprint(cw, "SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS = 0;\nSET SQL_MODE = 'NO_AUTO_VALUE_ON_ZERO';\nSET UNIQUE_CHECKS = 0;\n\n")

	for _, t := range tables {
		if !opt.NoCreateInfo {
			ddl, err := showCreate(tx, schema, t)
			if err != nil {
				return st, err
			}
			if opt.AddDropTable {
				fmt.Fprintf(cw, "DROP TABLE IF EXISTS %s;\n", Quote(t))
			}
			fmt.Fprintf(cw, "-- 表 %s\n%s;\n\n", t, ddl)
		}
		rows := 0
		if !opt.SchemaOnly {
			if rows, err = dumpTable(tx, cw, schema, t, opt.Chunk, int(byteCap)); err != nil {
				return st, fmt.Errorf("dbdump: 导出 %s 失败: %w", t, err)
			}
			st.Rows += rows
		}
		if opt.Progress != nil {
			opt.Progress(t, rows)
		}
	}
	fmt.Fprint(cw, "SET FOREIGN_KEY_CHECKS = 1;\nSET UNIQUE_CHECKS = 1;\n")

	bw, _ := cw.w.(*bufio.Writer)
	if err := bw.Flush(); err != nil {
		return st, fmt.Errorf("dbdump: 写出失败: %w", err)
	}
	st.Tables = len(tables)
	st.Bytes = cw.n
	return st, nil
}

// countWriter 统计写出字节数（bufio 包在外层，所以只能在此处计数）。
type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// SchemaFromDSN 从 go-sql-driver 的 DSN 里取库名：user:pw@tcp(host:port)/dbname?params
func SchemaFromDSN(dsn string) (string, error) {
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 || slash == len(dsn)-1 {
		return "", fmt.Errorf("dbdump: DSN 里没有库名: %s", redact(dsn))
	}
	rest := dsn[slash+1:]
	if q := strings.Index(rest, "?"); q >= 0 {
		rest = rest[:q]
	}
	if rest == "" {
		return "", fmt.Errorf("dbdump: DSN 里没有库名: %s", redact(dsn))
	}
	return rest, nil
}

// redact 抹掉 DSN 里的口令，避免它进错误信息或日志。
func redact(dsn string) string {
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		return "***@..." + dsn[at+1:]
	}
	return dsn
}

// queryer 让 *sql.Tx（导出）与 *sql.Conn（恢复）共用同一份查询代码。
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// listTables 按名字排序列出基表；filter 为空表示全库，支持以 * 结尾的前缀通配。
func listTables(ctx context.Context, q queryer, schema string, filter []string) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`, schema)
	if err != nil {
		return nil, fmt.Errorf("dbdump: 查表清单失败: %w", err)
	}
	defer rows.Close()

	var all []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		all = append(all, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(filter) == 0 {
		return all, nil
	}

	want := map[string]bool{}
	prefixes := []string{}
	for _, f := range filter {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if strings.HasSuffix(f, "*") {
			prefixes = append(prefixes, strings.TrimSuffix(f, "*"))
		} else {
			want[f] = true
		}
	}
	var out []string
	for _, t := range all {
		if want[t] {
			out = append(out, t)
			continue
		}
		for _, p := range prefixes {
			if strings.HasPrefix(t, p) {
				out = append(out, t)
				break
			}
		}
	}
	// 点名的表不存在必须报错：静默少导一张表比失败更难发现。
	var missing []string
	for t := range want {
		if !contains(all, t) {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("dbdump: 库里不存在这些表: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

func showCreate(tx *sql.Tx, schema, table string) (string, error) {
	var name, ddl string
	if err := tx.QueryRow("SHOW CREATE TABLE "+Quote(schema)+"."+Quote(table)).Scan(&name, &ddl); err != nil {
		return "", fmt.Errorf("dbdump: 读取 %s 建表语句失败: %w", table, err)
	}
	return ddl, nil
}

func dumpTable(tx *sql.Tx, w io.Writer, schema, table string, chunk, capBytes int) (int, error) {
	cols, err := columns(tx, schema, table)
	if err != nil {
		return 0, err
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("%s 没有可导出的列", table)
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = Quote(c)
	}
	head := fmt.Sprintf("INSERT INTO %s (%s) VALUES\n", Quote(table), strings.Join(names, ", "))

	rows, err := tx.Query("SELECT * FROM " + Quote(schema) + "." + Quote(table)) //nolint:gosec // G202: identifiers are backtick-quoted via Quote()
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var n int
	var buf strings.Builder
	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		if _, err := fmt.Fprint(w, head, strings.TrimSuffix(buf.String(), ",\n"), ";\n\n"); err != nil {
			return err
		}
		buf.Reset()
		return nil
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return n, err
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			parts[i] = valueSQL(v)
		}
		buf.WriteString("(" + strings.Join(parts, ",") + "),\n")
		n++
		if n%chunk == 0 || buf.Len() > capBytes {
			if err := flush(); err != nil {
				return n, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return n, err
	}
	return n, flush()
}

func columns(tx *sql.Tx, schema, table string) ([]string, error) {
	rows, err := tx.Query(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// Quote 把标识符包成反引号形式，内部的反引号按 SQL 规则双写。
func Quote(id string) string { return "`" + strings.ReplaceAll(id, "`", "``") + "`" }

// valueSQL 把扫描值渲染成 SQL 字面量。一律按字符串字面量输出即可：引号内的数字会被
// 服务端隐式转成目标数值列类型，省掉按列类型分派的复杂度；NULL 走无引号分支，
// 非 UTF-8 字节（二进制列）走十六进制字面量，避免字符集转换毁数据。
func valueSQL(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		if !utf8.Valid(x) {
			return "0x" + fmt.Sprintf("%X", x)
		}
		return "'" + EscapeString(string(x)) + "'"
	case string:
		if !utf8.ValidString(x) {
			return "0x" + fmt.Sprintf("%X", []byte(x))
		}
		return "'" + EscapeString(x) + "'"
	case time.Time:
		return "'" + x.Format("2006-01-02 15:04:05.000000") + "'"
	case bool:
		if x {
			return "1"
		}
		return "0"
	default:
		return "'" + EscapeString(fmt.Sprint(x)) + "'"
	}
}

// EscapeString 按 MySQL 反斜杠转义规则处理字符串字面量内容。
func EscapeString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case 0:
			b.WriteString("\\0")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case 26:
			b.WriteString("\\Z")
		case '\'':
			b.WriteString("\\'")
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
