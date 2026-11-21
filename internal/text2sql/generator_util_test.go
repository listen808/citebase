package text2sql

import (
	"strings"
	"testing"
)

func TestCleanTableName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"users", "users"},
		{"`users`", "users"},
		{"\"users\"", "users"},
		{"db.users", "users"},
		{"`db`.`users`", "users"},
		{"schema.public.orders", "orders"},
		{"'orders'", "orders"},
	}
	for _, c := range cases {
		if got := cleanTableName(c.in); got != c.want {
			t.Errorf("cleanTableName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCleanColumnName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"id", "id"},
		{"`id`", "id"},
		{"users.id", "id"},
		{"`users`.`id`", "id"},
	}
	for _, c := range cases {
		if got := cleanColumnName(c.in); got != c.want {
			t.Errorf("cleanColumnName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeSQL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"adds trailing semicolon", "SELECT 1", "SELECT 1;"},
		{"keeps single semicolon", "SELECT 1;", "SELECT 1;"},
		{"trims trailing whitespace", "SELECT 1   \n", "SELECT 1;"},
		{"strips duplicate trailing semicolons", "SELECT 1;;", "SELECT 1;"},
	}
	for _, c := range cases {
		if got := sanitizeSQL(c.in); got != c.want {
			t.Errorf("%s: sanitizeSQL(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestSanitizeSQLQualifiedIdentifiers(t *testing.T) {
	in := `SELECT "users"."id" FROM "users"`
	got := sanitizeSQL(in)
	if strings.Contains(got, `"users"."id"`) {
		t.Errorf("sanitizeSQL did not simplify qualified identifiers: %q", got)
	}
	if !strings.HasSuffix(got, ";") {
		t.Errorf("sanitizeSQL should append semicolon: %q", got)
	}
}

func TestStripCommonAffixes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"user_id", "user"},
		{"param_status", "status"},
		{"sys_config_code", "config"},
		{"order_date", "order"},
		{"plain", "plain"},
	}
	for _, c := range cases {
		if got := stripCommonAffixes(c.in); got != c.want {
			t.Errorf("stripCommonAffixes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
	}
	for _, c := range cases {
		if got := levenshteinDistance(c.a, c.b); got != c.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMin3(t *testing.T) {
	cases := []struct {
		a, b, c, want int
	}{
		{1, 2, 3, 1},
		{3, 2, 1, 1},
		{2, 1, 3, 1},
		{5, 5, 5, 5},
	}
	for _, c := range cases {
		if got := min3(c.a, c.b, c.c); got != c.want {
			t.Errorf("min3(%d,%d,%d) = %d, want %d", c.a, c.b, c.c, got, c.want)
		}
	}
}

func TestIsSQLKeyword(t *testing.T) {
	keywords := []string{"SELECT", "FROM", "WHERE", "JOIN", "GROUP", "ORDER", "COUNT"}
	for _, k := range keywords {
		if !isSQLKeyword(k) {
			t.Errorf("isSQLKeyword(%q) = false, want true", k)
		}
	}
	nonKeywords := []string{"users", "id", "created_at", "orders"}
	for _, n := range nonKeywords {
		if isSQLKeyword(n) {
			t.Errorf("isSQLKeyword(%q) = true, want false", n)
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	got := extractKeywords("show all orders from users table")
	// stop words like show/all/from should be filtered
	for _, w := range got {
		if w == "show" || w == "all" || w == "from" {
			t.Errorf("extractKeywords should filter stop word %q, got %v", w, got)
		}
	}
	// meaningful tokens should be retained
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "orders") || !strings.Contains(joined, "users") {
		t.Errorf("extractKeywords missing meaningful tokens: %v", got)
	}
}

func TestExtractKeywordsDedup(t *testing.T) {
	got := extractKeywords("orders orders orders")
	count := 0
	for _, w := range got {
		if w == "orders" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("extractKeywords should dedup, got %d occurrences of 'orders': %v", count, got)
	}
}
