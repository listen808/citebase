package trust

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// TestDefaultPolicyStillDeniesSystemCatalog 钉住"没有策略行 ≠ 没有黑名单"。
//
// 修复前 defaultPolicy() 只给 MaxRows/TimeoutSec，DenyPatterns 是空串，
// 而 matchPatterns 对空串一条都不匹配 —— 于是没配过策略的空间等于完全敞着：
// Text2SQL 拿的是主业务库连接，一条 SELECT ... FROM information_schema 就能拖出全库表结构，
// INTO OUTFILE / LOAD_FILE 更是直接读写服务器磁盘。
// 这一条只能靠测试钉住：静态看代码时，"默认值是空的"看起来完全正常。
func TestDefaultPolicyStillDeniesSystemCatalog(t *testing.T) {
	e := NewPolicyEngine(nil) // repo 为 nil 走 defaultPolicy()
	ctx := context.Background()

	denied := []string{
		"SELECT * FROM information_schema.tables",
		"select table_name from INFORMATION_SCHEMA.COLUMNS",
		"SELECT user, authentication_string FROM mysql.user",
		"SELECT * FROM performance_schema.events",
		"SELECT LOAD_FILE('/etc/passwd')",
		"SELECT 1 INTO OUTFILE '/tmp/x.csv'",
		"SELECT SLEEP(30)",
		"SELECT BENCHMARK(100000000, SHA1('a'))",
		"SELECT * FROM pg_catalog.pg_tables",
		"SELECT * FROM sqlite_master",
	}
	for _, sql := range denied {
		res, err := e.Check(ctx, 0, sql, 10)
		if err != nil {
			t.Fatalf("Check(%q) 返回错误: %v", sql, err)
		}
		if res.Allowed {
			t.Errorf("底线黑名单没拦住 %q —— 这一条允许通过就等于把主业务库的元数据与磁盘读写开放出去", sql)
		}
	}

	allowed := []string{
		"SELECT id, name FROM aw_document WHERE space_id = 1 LIMIT 10",
		"SELECT COUNT(*) FROM aw_user",
	}
	for _, sql := range allowed {
		res, err := e.Check(ctx, 0, sql, 10)
		if err != nil {
			t.Fatalf("Check(%q) 返回错误: %v", sql, err)
		}
		if !res.Allowed {
			t.Errorf("业务查询 %q 被底线规则误拦（%s）—— 黑名单不能把正常读也一起挡掉", sql, res.Reason)
		}
	}
}

// TestBaselineRulesCompile 钉的是这条底线自己的可执行性：
// matchPatterns 对编译失败的正则只记一条 info 日志然后**跳过**，
// 所以写坏一行 = 那一条防护静默消失，而 Check 的返回值看起来完全正常。
// 这条底线初稿里 performance_schema 那一行就写成了 `(?i,\s*)?` —— 非法语法，
// 没有这条测试它会一直"在清单上"，实际一条也没拦过。
func TestBaselineRulesCompile(t *testing.T) {
	if strings.TrimSpace(DefaultDenyPatterns) == "" {
		t.Fatal("DefaultDenyPatterns 是空的 —— 这条底线本身就没写进去，上面所有断言都是假的")
	}
	lines := strings.Split(DefaultDenyPatterns, "\n")
	compiled := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, err := regexp.Compile(line); err != nil {
			t.Errorf("底线规则 %q 编译失败: %v（matchPatterns 会静默跳过它，等于这条不存在）", line, err)
			continue
		}
		compiled++
	}
	if compiled < len(lines) {
		t.Errorf("底线共 %d 行、只有 %d 行可编译", len(lines), compiled)
	}
}

// TestBaselineRulesAreCaseInsensitive 钉住每条规则都要对大小写不敏感：
// SQL 里的系统表名几乎不会被写成全小写，而 Check 拿到的就是用户/模型原文。
// 少了 (?i) 的规则只能拦住一种写法，看起来"有规则"，实际是漏的。
func TestBaselineRulesAreCaseInsensitive(t *testing.T) {
	for _, line := range strings.Split(DefaultDenyPatterns, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, err := regexp.Compile(line); err != nil {
			continue // 编译问题由上一条测试负责报错
		}
		if !strings.Contains(strings.ToLower(line), "(?i)") {
			t.Errorf("底线规则 %q 没写 (?i) —— SQL 关键字大小写随意，这条只能拦住一种写法", line)
		}
	}
}
