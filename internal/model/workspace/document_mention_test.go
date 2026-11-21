package workspace

import (
	"reflect"
	"testing"
)

func TestExtractMentions(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"hello @alice how are you", []string{"alice"}},
		{"@bob @alice @bob 重复的只保留一个", []string{"bob", "alice"}},
		{"@张伟 中文名也可 @李娜", []string{"张伟", "李娜"}},
		{"@alice, @bob 标点分隔", []string{"alice", "bob"}},
		{"无提及文本", nil},
		{"email@x.com 不是提及", nil},
	}
	for _, c := range cases {
		got := ExtractMentions(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExtractMentions(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
