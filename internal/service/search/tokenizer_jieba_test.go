package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFallbackTokenizeProducesUsableTokens(t *testing.T) {
	input := []byte("集团接入大屏地址 Grafana2024 监控")
	stream := fallbackTokenize(input)
	if len(stream) == 0 {
		t.Fatal("fallback 返回空 token 流")
	}
	joined := make([]string, 0, len(stream))
	for _, tk := range stream {
		joined = append(joined, string(tk.Term))
	}
	all := strings.Join(joined, "|")
	// 旧实现会把整段文本当成一个 token，任何词项都命中不了；降级为「单字 + 相邻二元组」
	for _, want := range []string{"集", "集团", "接入", "大屏", "监控", "Grafana2024"} {
		if !strings.Contains(all, want) {
			t.Errorf("降级分词缺少词项 %q，实际=%v", want, joined)
		}
	}
	// 偏移必须落在原文上，否则高亮与短语查询定位失准
	for _, tk := range stream {
		if tk.End > len(input) || tk.Start >= tk.End {
			t.Fatalf("token 偏移越界: start=%d end=%d len=%d term=%q", tk.Start, tk.End, len(input), string(tk.Term))
		}
		if string(input[tk.Start:tk.End]) != string(tk.Term) {
			t.Errorf("token 偏移与文本不一致: %q != %q", string(input[tk.Start:tk.End]), string(tk.Term))
		}
	}
}

func TestResolveJiebaDictUsesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	dict := filepath.Join(dir, "s_1.txt")
	// gse 词典格式：词 频率 词性
	if err := os.WriteFile(dict, []byte("大屏 5 n\n监控 3 n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIWIKI_JIEBA_DICT", dir)

	got := resolveJiebaDict()
	if got == "" {
		t.Fatal("未解析到词典路径")
	}
	if !strings.Contains(got, dict) {
		t.Errorf("词典路径应指向环境变量指定目录，实际=%s", got)
	}
}

func TestResolveJiebaDictFromPackagedLayout(t *testing.T) {
	// 模拟部署包布局：<工作目录>/data/dict/zh/s_1.txt（scripts/package-windows.bat 复制出来的结构），
	// 且不设置 AIWIKI_JIEBA_DICT —— 这正是换机器部署后的真实场景。
	dir := t.TempDir()
	target := filepath.Join(dir, "data", "dict", "zh")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"s_1.txt", "t_1.txt"} {
		if err := os.WriteFile(filepath.Join(target, name), []byte("大屏 5 n\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("AIWIKI_JIEBA_DICT", "")
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	got := resolveJiebaDict()
	if !strings.Contains(got, filepath.Join("data", "dict", "zh", "s_1.txt")) {
		t.Fatalf("未解析到随包词典，实际=%q", got)
	}
}

func TestInitJiebaLoadsExplicitDictPath(t *testing.T) {
	// 回归验证：gse 的无参 LoadDict() 会解析到「构建机 module cache 的绝对路径」，
	// 换机器部署必然加载失败（打包后搜索不可用的根因）；显式传路径必须能加载成功。
	dir := t.TempDir()
	dict := filepath.Join(dir, "s_1.txt")
	if err := os.WriteFile(dict, []byte("大屏 5 n\n监控 3 n\n接入 4 v\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIWIKI_JIEBA_DICT", dir)

	// 重置全局状态，模拟进程首次加载
	jiebaMu.Lock()
	jiebaSeg, jiebaInited, jiebaNextTry = nil, false, time.Time{}
	jiebaMu.Unlock()

	initJieba()
	jiebaMu.Lock()
	inited := jiebaInited
	jiebaMu.Unlock()
	if !inited {
		t.Fatal("显式词典路径下仍然加载失败")
	}

	stream := NewJiebaTokenizer().Tokenize([]byte("集团接入大屏"))
	joined := make([]string, 0, len(stream))
	for _, s := range stream {
		joined = append(joined, string(s.Term))
	}
	if !strings.Contains(strings.Join(joined, "|"), "大屏") {
		t.Errorf("分词结果未包含词典词「大屏」，实际=%v", joined)
	}
}
