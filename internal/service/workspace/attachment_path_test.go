package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/listen808/citebase/internal/utils/config"
)

// TestResolveAttachmentPath 钉住 aw_attachment.path 的目录归属：
// 库里同时存在 images/（旧图片）与 attachment/（新图片与附件）两种前缀，
// 下载/删除/导出都靠它选目录，选错就会静默找不到文件。
func TestResolveAttachmentPath(t *testing.T) {
	attDir := filepath.Join(t.TempDir(), "attachment")
	imgDir := filepath.Join(t.TempDir(), "images")
	oldAtt := config.AppConfig.Storage.AttachmentDir
	oldImg := config.AppConfig.Storage.ImageDir
	config.AppConfig.Storage.AttachmentDir = attDir
	config.AppConfig.Storage.ImageDir = imgDir
	t.Cleanup(func() {
		config.AppConfig.Storage.AttachmentDir = oldAtt
		config.AppConfig.Storage.ImageDir = oldImg
	})

	legacy := filepath.Join(attDir, "legacy", "a.png")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"attachment 前缀", "attachment/4/486/a.png", filepath.Join(attDir, "4/486/a.png")},
		{"images 前缀", "images/1/30/b.jpg", filepath.Join(imgDir, "1/30/b.jpg")},
		{"反斜杠前缀", `attachment\4\486\a.png`, filepath.Join(attDir, "4/486/a.png")},
		{"空路径", "", ""},
		{"路径穿越", "../etc/passwd", ""},
		{"带前缀穿越", "images/../../etc/passwd", ""},
		{"无前缀命中附件目录", "legacy/a.png", legacy},
		{"无前缀回退图片目录", "legacy/missing.png", filepath.Join(imgDir, "legacy/missing.png")},
	}

	for _, tc := range cases {
		if got := ResolveAttachmentPath(tc.raw); got != tc.want {
			t.Errorf("%s: ResolveAttachmentPath(%q) = %q, want %q", tc.name, tc.raw, got, tc.want)
		}
	}
}
