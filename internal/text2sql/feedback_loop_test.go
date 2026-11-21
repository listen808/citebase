package text2sql

import (
	"testing"
	"time"
)

func TestNewFeedbackLoop(t *testing.T) {
	fb := NewFeedbackLoop()
	if fb == nil {
		t.Fatal("NewFeedbackLoop returned nil")
	}
	if fb.errorPatterns == nil {
		t.Error("errorPatterns should be initialized")
	}
}

func TestRecordSuccess(t *testing.T) {
	fb := NewFeedbackLoop()
	fb.RecordSuccess(FeedbackRecord{
		SpaceID:      1,
		UserQuery:    "查询员工",
		GeneratedSQL: "SELECT * FROM employees",
		Duration:     100 * time.Millisecond,
	})

	if len(fb.successRecords) != 1 {
		t.Fatalf("successRecords length = %d, want 1", len(fb.successRecords))
	}
	if fb.successRecords[0].UserQuery != "查询员工" {
		t.Errorf("UserQuery = %q, want %q", fb.successRecords[0].UserQuery, "查询员工")
	}
}

func TestRecordFailure(t *testing.T) {
	fb := NewFeedbackLoop()
	fb.RecordFailure(FeedbackRecord{
		SpaceID:      1,
		UserQuery:    "查询员工薪资",
		GeneratedSQL: "SELECT e.salary FROM employees e",
		ErrorType:    "unknown_column",
		ErrorMessage: "Unknown column 'e.salary'",
		Duration:     50 * time.Millisecond,
	})

	if len(fb.failureRecords) != 1 {
		t.Fatalf("failureRecords length = %d, want 1", len(fb.failureRecords))
	}
	if fb.failureRecords[0].ErrorType != "unknown_column" {
		t.Errorf("ErrorType = %q, want %q", fb.failureRecords[0].ErrorType, "unknown_column")
	}
}

func TestRecordCorrection(t *testing.T) {
	fb := NewFeedbackLoop()
	fb.RecordCorrection(FeedbackRecord{
		SpaceID:       1,
		UserQuery:     "查询员工薪资",
		GeneratedSQL:  "SELECT e.salary FROM employees e",
		CorrectedSQL:  "SELECT e.monthly_salary FROM employees e",
		CorrectionSrc: "user",
	})

	if len(fb.correctionRecords) != 1 {
		t.Fatalf("correctionRecords length = %d, want 1", len(fb.correctionRecords))
	}
}

func TestGetErrorPatternFrequency(t *testing.T) {
	fb := NewFeedbackLoop()
	fb.RecordFailure(FeedbackRecord{ErrorType: "unknown_column", ErrorMessage: "Unknown column 'salary'"})
	fb.RecordFailure(FeedbackRecord{ErrorType: "unknown_column", ErrorMessage: "Unknown column 'age'"})
	fb.RecordFailure(FeedbackRecord{ErrorType: "unknown_table", ErrorMessage: "Table 'emp' doesn't exist"})

	freq := fb.GetErrorPatternFrequency()
	if freq["unknown_column"] != 2 {
		t.Errorf("unknown_column frequency = %d, want 2", freq["unknown_column"])
	}
	if freq["unknown_table"] != 1 {
		t.Errorf("unknown_table frequency = %d, want 1", freq["unknown_table"])
	}
}

func TestGetSuccessExamples(t *testing.T) {
	fb := NewFeedbackLoop()
	fb.RecordSuccess(FeedbackRecord{UserQuery: "查询员工", GeneratedSQL: "SELECT * FROM employees"})
	fb.RecordSuccess(FeedbackRecord{UserQuery: "查询部门", GeneratedSQL: "SELECT * FROM departments"})

	examples := fb.GetSuccessExamples(5)
	if len(examples) != 2 {
		t.Errorf("GetSuccessExamples length = %d, want 2", len(examples))
	}
}
