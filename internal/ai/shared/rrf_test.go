package shared

import "testing"

func TestRRFMerge(t *testing.T) {
	list1 := []RRFItem{
		{Key: "A", Score: 0.9, Data: "list1"},
		{Key: "B", Score: 0.8, Data: "list1"},
		{Key: "C", Score: 0.7, Data: "list1"},
	}
	list2 := []RRFItem{
		{Key: "B", Score: 0.85, Data: "list2"},
		{Key: "D", Score: 0.6, Data: "list2"},
	}

	result := RRFMerge([][]RRFItem{list1, list2}, 60.0, 10)

	if len(result) != 4 {
		t.Errorf("expected 4 results, got %d", len(result))
	}
	if result[0].Key != "B" {
		t.Errorf("expected B first (appears in both lists), got %s", result[0].Key)
	}
}

func TestRRFMerge_Limit(t *testing.T) {
	list1 := []RRFItem{
		{Key: "A", Score: 0.9},
		{Key: "B", Score: 0.8},
		{Key: "C", Score: 0.7},
	}

	result := RRFMerge([][]RRFItem{list1}, 60.0, 2)

	if len(result) != 2 {
		t.Errorf("expected 2 results with limit, got %d", len(result))
	}
}
