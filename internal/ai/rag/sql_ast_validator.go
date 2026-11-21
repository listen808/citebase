package rag

import (
	"errors"
	"io"
	"regexp"
	"strings"

	"github.com/xwb1989/sqlparser"
)

// 预编译正则：匹配 SQL 字符串字面量，含"单个单引号写成两个"的转义形式。
// 措辞别改回字面的两连单引号：gofmt 会把注释里成对的 ' 规整成排版用的弯引号，
// 那条注释就从"如何转义"变成了"一个右双引号"，意思是错的，而 diff 看着只是改了个符号。
var sqlStringLiteralRe = regexp.MustCompile(`'(?:[^']|'')*'`)

// 预编译正则：匹配 -- 注释起始（任意位置）
// 安全优先：词法器对 -- 的注释判定与前字符无关（数字后同样可能是注释，如 "WHERE a=1--"），
// 按前字符放行会造成“尾部注释吃掉后追加的 LIMIT”的绕过，因此一律拒绝。
var sqlLineCommentRe = regexp.MustCompile(`--`)

// 预编译正则：匹配 MySQL 反引号标识符，含"单个反引号写成两个"的转义形式。
// 同上：字面的两连反引号会被 gofmt 规整成弯引号，改文字描述而不是改符号。
var sqlBacktickIdentRe = regexp.MustCompile("`(?:[^`]|``)*`")

var (
	// ErrMultipleStatements is returned when the SQL contains multiple statements.
	ErrMultipleStatements = errors.New("sql: multiple statements not allowed")
	// ErrNotSelect is returned when the SQL statement is not a SELECT.
	ErrNotSelect = errors.New("sql: only SELECT statements are allowed")
	// ErrSubqueryNotSelect is returned when a subquery is not a SELECT.
	ErrSubqueryNotSelect = errors.New("sql: subquery must be a SELECT statement")
	// ErrSQLCommentInjection is returned when SQL contains comment injection.
	ErrSQLCommentInjection = errors.New("sql: comment injection detected")
	// ErrEmptySQL is returned when the SQL string is empty or whitespace-only.
	ErrEmptySQL = errors.New("sql: empty statement")
)

// ValidateSQLAST validates a SQL string by parsing it into an AST and checking:
//   - Only a single SELECT statement is allowed
//   - All subqueries must also be SELECT
//   - No SQL comment injection (-- or /* */)
func ValidateSQLAST(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return ErrEmptySQL
	}

	// Check for comment injection before parsing
	if hasCommentInjection(trimmed) {
		return ErrSQLCommentInjection
	}

	// Use ParseNext to detect multiple statements
	tokenizer := sqlparser.NewStringTokenizer(trimmed)

	stmt, err := sqlparser.ParseNext(tokenizer)
	if err != nil {
		if err == io.EOF {
			return ErrEmptySQL
		}
		return err
	}

	// Check that the first statement is a SELECT
	if !isSelectStatement(stmt) {
		return ErrNotSelect
	}

	// Recursively check subqueries in the statement
	if err := checkSubqueries(stmt); err != nil {
		return err
	}

	// Check that there are no more statements
	nextStmt, err := sqlparser.ParseNext(tokenizer)
	if err == nil {
		// Another statement exists
		_ = nextStmt
		return ErrMultipleStatements
	}
	if err != io.EOF {
		// Parse error on second statement
		return ErrMultipleStatements
	}

	return nil
}

// isSelectStatement checks if a Statement is a SELECT statement (Select, Union, or ParenSelect).
func isSelectStatement(stmt sqlparser.Statement) bool {
	switch stmt.(type) {
	case *sqlparser.Select, *sqlparser.Union, *sqlparser.ParenSelect:
		return true
	default:
		return false
	}
}

// checkSubqueries walks the AST and verifies that all Subquery nodes
// contain only SELECT statements.
func checkSubqueries(node sqlparser.SQLNode) error {
	var subqErr error
	_ = sqlparser.Walk(func(n sqlparser.SQLNode) (bool, error) {
		if sq, ok := n.(*sqlparser.Subquery); ok {
			if sq.Select == nil {
				subqErr = ErrSubqueryNotSelect
				return false, subqErr
			}
			switch sq.Select.(type) {
			case *sqlparser.Select:
				// valid
			case *sqlparser.Union:
				// valid
			case *sqlparser.ParenSelect:
				// valid
			default:
				subqErr = ErrSubqueryNotSelect
				return false, subqErr
			}
		}
		return true, nil
	}, node)
	return subqErr
}

// hasCommentInjection checks for SQL comment patterns that could be used for injection.
// --、# 行注释与 /* */ 块注释均视为注入向量（fail-closed）：
// 词法器对 # 与数字后的 -- 同样按注释处理，若按前字符启发式放行，
// 攻击者可用尾部注释吃掉执行器在校验之后追加的 LIMIT，绕过行数上限。
// 字符串字面量与反引号标识符先剔除以避免误报。
func hasCommentInjection(sql string) bool {
	// 剔除字符串字面量，避免字符串内的 -- 或 /* 被误判
	stripped := sqlStringLiteralRe.ReplaceAllString(sql, "''")
	// 剔除反引号标识符（MySQL 允许标识符含 #），避免误判
	stripped = sqlBacktickIdentRe.ReplaceAllString(stripped, "`id`")
	// Single-line comment injection: -- 与 #（MySQL 词法器把 # 也当行注释）
	// 一律拒绝，不再按前字符放行，防止尾部注释吞掉执行器追加的 LIMIT
	if sqlLineCommentRe.MatchString(stripped) {
		return true
	}
	if strings.Contains(stripped, "#") {
		return true
	}
	// Block comment injection: /* or */ can be used to close an opened comment
	if strings.Contains(stripped, "/*") || strings.Contains(stripped, "*/") {
		return true
	}
	return false
}
