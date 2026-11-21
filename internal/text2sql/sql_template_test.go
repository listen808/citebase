package text2sql

import (
	"testing"
)

func TestNewSQLTemplateLibrary(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	if lib == nil {
		t.Fatal("NewSQLTemplateLibrary returned nil")
	}
	if len(lib.templates) == 0 {
		t.Error("templates should be pre-populated with built-in templates")
	}
}

func TestMatchTemplate_Aggregation(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	params := TemplateParams{
		Intent:    "aggregation",
		Tables:    []string{"employees"},
		GroupBy:   []string{"department"},
		Aggregate: "COUNT(*)",
	}
	match := lib.MatchTemplate(params)
	if match == nil {
		t.Fatal("MatchTemplate should match aggregation template")
	}
	if match.Template.ID != "agg_group_by" {
		t.Errorf("Template ID = %q, want %q", match.Template.ID, "agg_group_by")
	}
}

func TestMatchTemplate_TopN(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	params := TemplateParams{
		Intent:  "ranking",
		Tables:  []string{"products"},
		OrderBy: []string{"sales DESC"},
		Limit:   10,
	}
	match := lib.MatchTemplate(params)
	if match == nil {
		t.Fatal("MatchTemplate should match ranking template")
	}
	if match.Template.ID != "top_n" {
		t.Errorf("Template ID = %q, want %q", match.Template.ID, "top_n")
	}
}

func TestFillTemplate(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	tpl := lib.GetTemplate("agg_group_by")
	if tpl == nil {
		t.Fatal("GetTemplate should return template")
	}

	params := TemplateParams{
		Tables:       []string{"employees"},
		GroupBy:      []string{"department"},
		Aggregate:    "COUNT(*)",
		SelectFields: []string{"department", "COUNT(*) AS cnt"},
	}
	sql := lib.FillTemplate(tpl, params)
	if sql == "" {
		t.Error("FillTemplate should return non-empty SQL")
	}
}

func TestMatchTemplate_NoMatch(t *testing.T) {
	lib := NewSQLTemplateLibrary()
	params := TemplateParams{
		Intent: "unknown_complex_query",
		Tables: []string{},
	}
	match := lib.MatchTemplate(params)
	if match != nil {
		t.Error("MatchTemplate should return nil for unmatched params")
	}
}
