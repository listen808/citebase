package workspace

import (
	"strings"
	"testing"
)

// TestImportPathParsing 验证 ImportDocuments 中 path 解析的边界情况
func TestImportPathParsing(t *testing.T) {
	testCases := []struct {
		name    string
		path    string
		skipped bool
	}{
		{"单文件 test.md", "test.md", false},
		{"空路径", "", true},
		{"子目录文件", "subdir/test.md", false},
		{"前导斜杠", "/test.md", false},
		{"末尾斜杠", "test.md/", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.Split(strings.Trim(tc.path, "/"), "/")
			skip := len(parts) == 0 || parts[0] == ""
			if skip != tc.skipped {
				t.Errorf("path=%q 期望 skipped=%v, 实际 skipped=%v", tc.path, tc.skipped, skip)
			}
		})
	}
}

// TestUniqueNameBatchDeduplication 验证批次内 uniqueName 不会重复
func TestUniqueNameBatchDeduplication(t *testing.T) {
	nameMap := make(map[string]int)
	svc := &DocumentService{}

	names := []string{
		svc.uniqueName(0, "A", nameMap),
		svc.uniqueName(0, "A", nameMap),
		svc.uniqueName(0, "A", nameMap),
	}

	if names[0] != "A" {
		t.Errorf("第一个名称应为 A, 实际为 %q", names[0])
	}
	if names[1] != "A (1)" {
		t.Errorf("第二个名称应为 A (1), 实际为 %q", names[1])
	}
	if names[2] != "A (2)" {
		t.Errorf("第三个名称应为 A (2), 实际为 %q", names[2])
	}
}

// TestUniqueNameParentIsolation 验证不同 parentID 的同名不会冲突
func TestUniqueNameParentIsolation(t *testing.T) {
	nameMap := make(map[string]int)
	svc := &DocumentService{}

	n1 := svc.uniqueName(10, "A", nameMap)
	n2 := svc.uniqueName(20, "A", nameMap)

	if n1 != "A" || n2 != "A" {
		t.Errorf("不同 parentID 的同名应互不影响, 实际: %q, %q", n1, n2)
	}
}
