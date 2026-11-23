package ai

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSortedPageNamesStableOrder(t *testing.T) {
	pages := map[string]string{"b.md": "", "a.md": "", "c.md": ""}
	got := sortedPageNames(pages)
	want := []string{"a.md", "b.md", "c.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedPageNames = %v, want %v", got, want)
	}
}

func TestSortedPageNamesEmpty(t *testing.T) {
	if got := sortedPageNames(map[string]string{}); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		raw  string
		want int
		ok   bool
	}{
		{"12", 12, true},
		{"abc", 0, false},
		{"0", 0, false},
		{"-3", 0, false},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: tc.raw}}
		got, ok := pathID(c)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("pathID(%q) = (%d,%v), want (%d,%v)", tc.raw, got, ok, tc.want, tc.ok)
		}
		if !tc.ok && w.Code != 400 {
			t.Fatalf("pathID(%q) 非法时应已写出 400，实际 %d", tc.raw, w.Code)
		}
	}
}
