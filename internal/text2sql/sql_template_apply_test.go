package text2sql

import (
	"strings"
	"testing"
)

func TestApplyTemplate_Aggregation(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	sql, score := lib.ApplyTemplate("统计每个部门的员工数量", "aggregation", []string{"employees", "departments"}, []string{"dept_id", "name"}, nil)
	if sql == "" {
		t.Fatal("expected SQL from aggregation template")
	}
	if score < 0.7 {
		t.Fatalf("expected score >= 0.7, got %v", score)
	}
	if !strings.Contains(sql, "COUNT(*)") || !strings.Contains(sql, "GROUP BY") {
		t.Errorf("expected aggregation SQL, got %q", sql)
	}
}

func TestApplyTemplate_Ranking(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	sql, score := lib.ApplyTemplate("查询前5名销售额", "ranking", []string{"sales"}, []string{"amount"}, nil)
	if sql == "" {
		t.Fatal("expected SQL from ranking template")
	}
	if score < 0.7 {
		t.Fatalf("expected score >= 0.7, got %v", score)
	}
	if !strings.Contains(sql, "ORDER BY") || !strings.Contains(sql, "LIMIT 5") {
		t.Errorf("expected ranking SQL with LIMIT 5, got %q", sql)
	}
}

func TestApplyTemplate_Filter(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	sql, score := lib.ApplyTemplate("筛选用户信息", "filter", []string{"users"}, []string{"id", "name", "age"}, nil)
	if sql == "" {
		t.Fatal("expected SQL from filter template")
	}
	if score < 0.7 {
		t.Fatalf("expected score >= 0.7, got %v", score)
	}
	if !strings.HasPrefix(sql, "SELECT") {
		t.Errorf("expected SELECT SQL, got %q", sql)
	}
}

func TestApplyTemplate_Trend(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	sql, score := lib.ApplyTemplate("按月统计订单趋势", "trend", []string{"orders"}, []string{"created_at"}, nil)
	if sql == "" {
		t.Fatal("expected SQL from trend template")
	}
	if score < 0.7 {
		t.Fatalf("expected score >= 0.7, got %v", score)
	}
	if !strings.Contains(sql, "DATE_FORMAT") || !strings.Contains(sql, "GROUP BY") {
		t.Errorf("expected trend SQL, got %q", sql)
	}
}

func TestApplyTemplate_NoMatch(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	sql, score := lib.ApplyTemplate("随便说说", "unknown", []string{"users"}, []string{"id"}, nil)
	if sql != "" || score != 0 {
		t.Errorf("expected no match, got sql=%q score=%v", sql, score)
	}
}
