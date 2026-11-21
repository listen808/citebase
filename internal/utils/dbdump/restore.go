package dbdump

// 恢复：把 Dump 产出的 SQL 文本逐条回灌到 DSN 指向的库，不依赖 mysql 命令行客户端。
//
// 为什么要自己切语句：exec.Command("mysql") 除了要求机器上有客户端，还会把口令拼在
// `-p<password>` 上暴露在进程列表里。这里用驱动直连，按 MySQL 词法切语句，
// 字符串/标识符里的分号不会被误当结束符。

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
)

// RestoreOptions 恢复选项，零值可用。
type RestoreOptions struct {
	// ReplaceTables 遇到 CREATE TABLE 先 DROP 同名表。Dump 默认不写 DROP TABLE
	// （见 Options.AddDropTable），不替换的话，往有数据的库上回灌会撞 Error 1050。
	// 关闭它等于"只补不覆盖"：已有同名表时 CREATE 直接失败。
	ReplaceTables bool
	// Progress 每执行完一条语句回调一次（已执行条数、动词、表名）；nil 表示不汇报。
	Progress func(done int, verb, table string)
	// Warn 非致命问题回调；nil 表示忽略。
	Warn func(msg string)
}

// RestoreStats 恢复结果统计。
type RestoreStats struct {
	Statements int
	Tables     int
	Bytes      int64
}

// Restore 执行 r 里的 SQL。全程钉在一条连接上逐条执行：外键检查先关掉，
// 表清单按名字排序、父表不一定在前，不关就会卡在顺序上。
func Restore(dsn string, r io.Reader, opt RestoreOptions) (RestoreStats, error) {
	var st RestoreStats
	if dsn == "" {
		return st, errors.New("dbdump: DSN 为空")
	}
	if r == nil {
		return st, errors.New("dbdump: SQL 数据源为空")
	}
	schema, err := SchemaFromDSN(dsn)
	if err != nil {
		return st, err
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
	ctx := context.Background()
	// 必须钉住同一条连接：SET SESSION 只对当前连接生效，走连接池的话后面的语句读不到。
	conn, err := db.Conn(ctx)
	if err != nil {
		return st, fmt.Errorf("dbdump: 取连接失败: %w", err)
	}
	defer conn.Close()

	// 服务端单个包的上限；超过它语句会在驱动层被拒，先量出来才能给出可读的报错。
	var packet int64
	if err := conn.QueryRowContext(ctx, "SELECT @@max_allowed_packet").Scan(&packet); err != nil {
		warn("读取 max_allowed_packet 失败，按 %d 字节估计: %v", defaultMaxPacket, err)
	}
	if packet <= 0 {
		packet = defaultMaxPacket
	}

	if _, err := conn.ExecContext(ctx, "SET SESSION FOREIGN_KEY_CHECKS = 0"); err != nil {
		warn("关闭外键检查失败: %v", err)
	}

	before, err := listTables(ctx, conn, schema, nil)
	if err != nil {
		return st, err
	}

	sc := newScanner(bufio.NewReaderSize(r, 1<<20))
	touched := map[string]bool{}
	for {
		stmt, err := sc.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return st, fmt.Errorf("dbdump: 解析 SQL 失败（第 %d 条之后）: %w", st.Statements+1, err)
		}
		if stmt == "" {
			continue
		}
		if len(stmt) > int(packet*9/10) {
			return st, fmt.Errorf("dbdump: 第 %d 条语句 %d 字节，超过服务端 max_allowed_packet(%d) 的 90%%；用更小的 -chunk 重新导出，或调大该参数",
				st.Statements+1, len(stmt), packet)
		}
		verb, table := statementTarget(stmt)
		if verb == "CREATE" && table != "" && opt.ReplaceTables {
			if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+Quote(schema)+"."+Quote(table)); err != nil {
				return st, fmt.Errorf("dbdump: 删除待重建的 %s 失败: %w", table, err)
			}
		}
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return st, fmt.Errorf("dbdump: 第 %d 条语句失败（%s %s）: %w", st.Statements+1, verb, table, err)
		}
		if table != "" {
			touched[table] = true
		}
		st.Statements++
		st.Bytes += int64(len(stmt))
		if opt.Progress != nil {
			opt.Progress(st.Statements, verb, table)
		}
	}
	st.Tables = len(touched)

	// 备份里没有、库里却有的表不会被这次恢复碰到，说清楚免得事后以为整库被替换了。
	var left []string
	for _, t := range before {
		if !touched[t] {
			left = append(left, t)
		}
	}
	if len(left) > 0 {
		warn("库 %s 里另有 %d 张表不在本次备份中，未被改动（最多列出 10 张：%s）",
			schema, len(left), strings.Join(left[:min(10, len(left))], ", "))
	}
	return st, nil
}

// statementTarget 取出语句的动词和目标表名（SET、BEGIN 之类返回空表名）。
// 只用于日志、进度和 CREATE 的替换判断，不参与权限判断，所以解析从宽：认不出来就当没有表。
func statementTarget(stmt string) (verb, table string) {
	toks := firstTokens(stmt, 8)
	if len(toks) == 0 {
		return "", ""
	}
	verb = strings.ToUpper(toks[0])
	if !isTableVerb(verb) {
		return verb, ""
	}
	for i := 1; i < len(toks); i++ {
		switch strings.ToUpper(toks[i]) {
		case "TABLE", "INTO", "IGNORE", "IF", "NOT", "EXISTS", "TEMPORARY", "LOW_PRIORITY", "DELAYED":
			continue
		}
		return verb, unquoteIdent(toks[i])
	}
	return verb, ""
}

func isTableVerb(verb string) bool {
	switch verb {
	case "CREATE", "DROP", "ALTER", "TRUNCATE", "INSERT", "REPLACE", "UPDATE", "DELETE", "RENAME":
		return true
	}
	return false
}

// unquoteIdent 去掉反引号/双引号并还原双写的引号；`库`.`表` 形式只取表名那一段。
func unquoteIdent(s string) string {
	if len(s) < 2 || (s[0] != '`' && s[0] != '"') {
		return s
	}
	q := string(s[0])
	if i := strings.LastIndex(s, q+"."+q); i >= 0 {
		s = s[i+len(q)+2:]
	}
	s = strings.TrimPrefix(s, q)
	s = strings.TrimSuffix(s, q)
	return strings.ReplaceAll(s, q+q, q)
}

// firstTokens 按空白、括号、逗号切出至多 n 个词就收工：只用于读语句开头的动词和表名，
// 不能把整条几百 KB 的 INSERT 都扫一遍。引号内的空白不切分，反斜杠转义跳过一个字符。
func firstTokens(stmt string, n int) []string {
	var toks []string
	var cur []byte
	var quote byte
	flush := func() {
		if len(cur) > 0 {
			toks = append(toks, string(cur))
			cur = cur[:0]
		}
	}
	for i := 0; i < len(stmt); i++ {
		c := stmt[i]
		switch {
		case quote != 0:
			cur = append(cur, c)
			switch {
			case c == '\\' && quote != '`' && i+1 < len(stmt):
				i++
				cur = append(cur, stmt[i])
			case c == quote && i+1 < len(stmt) && stmt[i+1] == quote:
				i++
				cur = append(cur, stmt[i])
			case c == quote:
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
			cur = append(cur, c)
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' || c == ')' || c == ';' || c == ',':
			flush()
			if len(toks) >= n {
				return toks
			}
		default:
			cur = append(cur, c)
		}
	}
	flush()
	return toks
}

// scanner 是流式语句切分器：认单/双引号与反引号、反斜杠转义、双写的引号、
// `--` 与 `#` 行注释、`/* */` 块注释，因此字符串里的分号不会被误判成结束符。
// 不支持 DELIMITER（含存储过程的导出文件会被明确拒绝）；块注释一律丢弃，包括 MySQL
// 会执行的 /*! … */ 条件注释——导出文件里这类语句都只是会话变量，不依赖它。
type scanner struct {
	r     *bufio.Reader
	buf   []byte
	quote byte
}

func newScanner(r *bufio.Reader) *scanner { return &scanner{r: r} }

// next 返回下一条语句（已去掉结束分号与首尾空白）。文本读完时返回 io.EOF。
func (sc *scanner) next() (string, error) {
	for {
		c, err := sc.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if sc.quote != 0 {
					return "", errors.New("字符串或标识符未闭合")
				}
				if s := sc.take(); s != "" {
					return s, nil
				}
				return "", io.EOF
			}
			return "", err
		}
		if sc.quote != 0 {
			if err := sc.inQuote(c); err != nil {
				return "", err
			}
			continue
		}

		switch c {
		case '\'', '"', '`':
			sc.quote = c
			sc.buf = append(sc.buf, c)
		case '-':
			n, err := sc.r.ReadByte()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return "", err
				}
				sc.buf = append(sc.buf, c)
				continue
			}
			if n != '-' {
				_ = sc.r.UnreadByte()
				sc.buf = append(sc.buf, c)
				continue
			}
			m, err := sc.r.ReadByte()
			switch {
			case err == nil:
				_ = sc.r.UnreadByte()
				if !isCommentBreak(m) { // `a--b` 里的 `--` 不是注释：MySQL 要求其后是空白或行尾
					sc.buf = append(sc.buf, c, '-')
					continue
				}
			case !errors.Is(err, io.EOF):
				return "", err
			}
			sc.skipLine()
		case '#':
			sc.skipLine()
		case '/':
			n, err := sc.r.ReadByte()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return "", err
				}
				sc.buf = append(sc.buf, c)
				continue
			}
			if n != '*' {
				_ = sc.r.UnreadByte()
				sc.buf = append(sc.buf, c)
				continue
			}
			if err := sc.skipBlock(); err != nil {
				return "", err
			}
		case ';':
			sc.buf = append(sc.buf, c)
			s := sc.take()
			if s == "" { // 连续分号或整条都是注释
				continue
			}
			if strings.HasPrefix(strings.ToLower(s), "delimiter") {
				return "", errors.New("SQL 里含 DELIMITER（存储过程导出？），本恢复器不支持")
			}
			return s, nil
		default:
			sc.buf = append(sc.buf, c)
		}
	}
}

// inQuote 处理字符串/标识符内部的字符：转义与双写引号都要吃掉下一个字符，
// 只有真正的闭合引号才把状态清零。
func (sc *scanner) inQuote(c byte) error {
	sc.buf = append(sc.buf, c)
	if c == '\\' && sc.quote != '`' {
		n, err := sc.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("字符串字面量未闭合")
			}
			return err
		}
		sc.buf = append(sc.buf, n)
		return nil
	}
	if c != sc.quote {
		return nil
	}
	n, err := sc.r.ReadByte()
	switch {
	case err == nil && n == sc.quote: // 双写：'' 表示一个引号
		sc.buf = append(sc.buf, n)
	case err == nil:
		_ = sc.r.UnreadByte()
		sc.quote = 0
	case errors.Is(err, io.EOF):
		return errors.New("字符串字面量未闭合")
	default:
		return err
	}
	return nil
}

// take 收口当前缓冲。
func (sc *scanner) take() string {
	s := strings.TrimSpace(string(sc.buf))
	s = strings.TrimSuffix(s, ";")
	sc.buf = sc.buf[:0]
	return strings.TrimSpace(s)
}

func isCommentBreak(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

// skipLine 丢弃行注释余下的字符，保留换行本身（避免把注释和下一条语句粘在一起）。
func (sc *scanner) skipLine() {
	for {
		c, err := sc.r.ReadByte()
		if err != nil {
			return
		}
		if c == '\n' {
			sc.buf = append(sc.buf, '\n')
			return
		}
	}
}

// skipBlock 丢弃 /* */ 整段，用一个空格代替，避免注释两侧的词粘成一个。
func (sc *scanner) skipBlock() error {
	prev := byte(0)
	for {
		c, err := sc.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("块注释未闭合")
			}
			return err
		}
		if prev == '*' && c == '/' {
			sc.buf = append(sc.buf, ' ')
			return nil
		}
		prev = c
	}
}
