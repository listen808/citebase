package repocheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 模板解析不了的话，GitHub 不报错，只是**在"新建 issue"的下拉里看不见这个模板**——
// 于是仓库看起来"有模板"，实际从没被用过。这类静默失效只有把文件真解析一遍才看得见。
func TestGitHubIssueTemplatesParse(t *testing.T) {
	dir := filepath.Join("..", "..", ".github", "ISSUE_TEMPLATE")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读不到 %s: %v", dir, err)
	}

	parsed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		// config.yml 是"设置"文件（控制空白 issue 与联系链接），形态与表单模板不同，单独校验
		name := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读 %s: %v", name, err)
		}

		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Errorf("%s 解析失败: %v", e.Name(), err)
			continue
		}
		parsed++

		if e.Name() == "config.yml" {
			// 这份文件控制"能不能提空白 issue"与联系链接；字段名写错是静默失效
			if _, ok := doc["blank_issues_enabled"].(bool); !ok {
				t.Errorf("config.yml 的 blank_issues_enabled 必须是布尔值（缺失或写成字符串都不会生效）")
			}
			if links, ok := doc["contact_links"].([]any); ok {
				for i, raw := range links {
					l, _ := raw.(map[string]any)
					if u, _ := l["url"].(string); !strings.HasPrefix(u, "https://") {
						t.Errorf("config.yml contact_links[%d] 的 url 不是 https 绝对地址", i)
					}
				}
			}
			continue
		}
		if doc["name"] == nil || doc["description"] == nil {
			t.Errorf("%s 缺 name 或 description（GitHub 靠这两个字段渲染模板列表）", e.Name())
		}
		body, ok := doc["body"].([]any)
		if !ok || len(body) == 0 {
			t.Errorf("%s 的 body 缺失或为空", e.Name())
			continue
		}
		for i, raw := range body {
			field, ok := raw.(map[string]any)
			if !ok {
				t.Errorf("%s body[%d] 不是映射", e.Name(), i)
				continue
			}
			typ, _ := field["type"].(string)
			switch typ {
			case "markdown", "textarea", "input", "dropdown", "checkboxes", "tabs":
			default:
				t.Errorf("%s body[%d] 的 type=%q 不是 GitHub issue forms 认的类型", e.Name(), i, typ)
				continue
			}
			if typ == "markdown" {
				continue
			}
			// 除说明块之外，每个控件必须有 id、label，否则提交上来的值无法定位
			if id, _ := field["id"].(string); id == "" {
				t.Errorf("%s body[%d] (%s) 缺 id", e.Name(), i, typ)
			}
			fattrs, _ := field["attributes"].(map[string]any)
			if fattrs == nil || fattrs["label"] == nil {
				t.Errorf("%s body[%d] (%s) 缺 attributes.label", e.Name(), i, typ)
			}
			if typ == "dropdown" {
				opts, _ := fattrs["options"].([]any)
				if len(opts) < 2 {
					t.Errorf("%s body[%d] 下拉至少要有两个选项", e.Name(), i)
				}
			}
			if typ == "checkboxes" {
				opts, _ := fattrs["options"].([]any)
				if len(opts) == 0 {
					t.Errorf("%s body[%d] 勾选项为空", e.Name(), i)
				}
			}
		}
	}

	if parsed < 2 {
		t.Errorf("只解析到 %d 个模板文件，至少要 2 个（bug / 功能）——目录空了或被清空都会走到这里", parsed)
	}
}

// SECURITY.md 存在且留了可执行的报告渠道，否则"如何报告漏洞"这一节是空的。
func TestSecurityPolicyHasReportingChannel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "SECURITY.md"))
	if err != nil {
		t.Fatalf("仓库根必须有 SECURITY.md: %v", err)
	}
	text := string(data)
	for _, must := range []string{"Report a vulnerability", "不在范围内", "已知问题与限制"} {
		if !strings.Contains(text, must) {
			t.Errorf("SECURITY.md 缺少 %q 一节的内容", must)
		}
	}
	// "公开 issue 不是报告渠道"这句话必须在，否则贡献者默认就往 issue 里写细节了
	if !strings.Contains(text, "不要公开发帖") && !strings.Contains(text, "不要公开") {
		t.Error("SECURITY.md 必须明确禁止公开披露")
	}
}
