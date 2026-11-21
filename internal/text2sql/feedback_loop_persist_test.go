package text2sql

import (
	"sync"
	"testing"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
)

type fakeFeedbackRepo struct {
	mu      sync.Mutex
	records []*aimodel.Text2SQLFeedback
}

func (r *fakeFeedbackRepo) Create(record *aimodel.Text2SQLFeedback) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

func (r *fakeFeedbackRepo) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.records)
}

func (r *fakeFeedbackRepo) Last() *aimodel.Text2SQLFeedback {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.records) == 0 {
		return nil
	}
	return r.records[len(r.records)-1]
}

func TestFeedbackLoop_PersistSuccess(t *testing.T) {
	repo := &fakeFeedbackRepo{}
	fb := NewFeedbackLoopWithRepo(repo)

	fb.RecordSuccess(FeedbackRecord{
		SpaceID:      1,
		UserQuery:    "查询员工",
		GeneratedSQL: "SELECT * FROM employees",
		Duration:     100 * time.Millisecond,
	})

	time.Sleep(100 * time.Millisecond)
	if repo.Count() != 1 {
		t.Fatalf("expected 1 persisted record, got %d", repo.Count())
	}
	last := repo.Last()
	if last.SpaceID != 1 || last.UserQuery != "查询员工" || !last.ExecutionOK {
		t.Errorf("unexpected persisted record: %+v", last)
	}
}

func TestFeedbackLoop_PersistFailure(t *testing.T) {
	repo := &fakeFeedbackRepo{}
	fb := NewFeedbackLoopWithRepo(repo)

	fb.RecordFailure(FeedbackRecord{
		SpaceID:      2,
		UserQuery:    "查询薪资",
		GeneratedSQL: "SELECT salary FROM employees",
		ErrorType:    "unknown_column",
		ErrorMessage: "Unknown column 'salary'",
		Duration:     50 * time.Millisecond,
	})

	time.Sleep(100 * time.Millisecond)
	if repo.Count() != 1 {
		t.Fatalf("expected 1 persisted record, got %d", repo.Count())
	}
	last := repo.Last()
	if last.ExecutionOK || last.ErrorType != "unknown_column" {
		t.Errorf("unexpected persisted record: %+v", last)
	}
}

func TestFeedbackLoop_PersistCorrection(t *testing.T) {
	repo := &fakeFeedbackRepo{}
	fb := NewFeedbackLoopWithRepo(repo)

	fb.RecordCorrection(FeedbackRecord{
		SpaceID:       3,
		UserQuery:     "查询员工薪资",
		GeneratedSQL:  "SELECT e.salary FROM employees e",
		CorrectedSQL:  "SELECT e.monthly_salary FROM employees e",
		CorrectionSrc: "ast",
	})

	time.Sleep(100 * time.Millisecond)
	if repo.Count() != 1 {
		t.Fatalf("expected 1 persisted record, got %d", repo.Count())
	}
	last := repo.Last()
	if last.CorrectionSrc != "ast" || last.CorrectedSQL == "" {
		t.Errorf("unexpected persisted record: %+v", last)
	}
}
