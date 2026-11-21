package wiki

import (
	"reflect"
	"testing"
)

func TestContradictionBlockNone(t *testing.T) {
	if got := ContradictionBlock("普通正文\n没有标记"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestContradictionBlockParse(t *testing.T) {
	content := "正文内容\n\n## 待裁决矛盾\n\n" + contradictionMarker + "\n- 甲说 X=1\n- 乙说 X=2\n"
	got := ContradictionBlock(content)
	want := []string{"甲说 X=1", "乙说 X=2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestContradictionBlockStopsAtNonListItem(t *testing.T) {
	content := contradictionMarker + "\n- 一条矛盾\n\n后续段落\n"
	got := ContradictionBlock(content)
	want := []string{"一条矛盾"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
