package text2sql

import (
	"testing"
)

func TestNewASTCorrector(t *testing.T) {
	corrector := NewASTCorrector()
	if corrector == nil {
		t.Fatal("NewASTCorrector returned nil")
	}
	if len(corrector.rules) == 0 {
		t.Error("rules should be pre-populated with built-in correction rules")
	}
}

func TestCorrect_UnknownColumn(t *testing.T) {
	corrector := NewASTCorrector()
	availableFields := map[string][]string{
		"employees": {"id", "name", "monthly_salary", "department_id"},
	}
	result := corrector.Correct("SELECT e.salary FROM employees e", "Unknown column 'e.salary'", availableFields)
	if result.Corrected {
		// 应该建议 monthly_salary 作为替代
		if result.Suggestion == "" {
			t.Error("Should provide suggestion for unknown column")
		}
	}
}

func TestCorrect_AmbiguousColumn(t *testing.T) {
	corrector := NewASTCorrector()
	availableFields := map[string][]string{
		"employees":   {"id", "name", "dept_id"},
		"departments": {"id", "name"},
	}
	result := corrector.Correct("SELECT name FROM employees, departments", "Column 'name' is ambiguous", availableFields)
	if !result.Corrected {
		t.Error("Should correct ambiguous column by adding table prefix")
	}
}

func TestCorrect_MissingTablePrefix(t *testing.T) {
	corrector := NewASTCorrector()
	availableFields := map[string][]string{
		"employees": {"id", "name"},
	}
	result := corrector.Correct("SELECT name FROM employees e", "", availableFields)
	// 无错误时不应修正
	if result.Corrected {
		t.Error("Should not correct when there is no error")
	}
}

func TestFuzzyMatchField(t *testing.T) {
	corrector := NewASTCorrector()
	fields := []string{"monthly_salary", "annual_bonus", "employee_name", "hire_date"}

	match := corrector.fuzzyMatchField("salary", fields)
	if match != "monthly_salary" {
		t.Errorf("fuzzyMatchField('salary') = %q, want %q", match, "monthly_salary")
	}

	match = corrector.fuzzyMatchField("name", fields)
	if match != "employee_name" {
		t.Errorf("fuzzyMatchField('name') = %q, want %q", match, "employee_name")
	}
}

func TestFuzzyMatchField_NoMatch(t *testing.T) {
	corrector := NewASTCorrector()
	fields := []string{"id", "code"}

	match := corrector.fuzzyMatchField("salary", fields)
	if match != "" {
		t.Errorf("fuzzyMatchField should return empty for no match, got %q", match)
	}
}
