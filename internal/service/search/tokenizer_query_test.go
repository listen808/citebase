package search

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	modelsearch "github.com/listen808/citebase/internal/model/search"
)

// setupSearchDict 用一份极小词典重置分词器与索引，避免测试依赖构建机的 gse module cache 路径。
func setupSearchDict(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	// gse 词典格式是「词 频率 词性」（空格分隔，与官方 zh/s_1.txt 一致）。
	// 必须同时给出低频率的单字条目：否则 gse 会对未知字符走 HMM 逐字切分，
	// 短词永远切不出来，与真实词典（含全部单字）的行为不一致，断言就失去意义。
	entries := []string{
		"集团 3000000 n", "接入 3000000 n", "大屏 3000000 n", "能力 3000000 n",
		"系统 3000000 n", "模块 3000000 n", "监控 3000000 n", "告警 3000000 n",
		"集 1 n", "团 1 n", "接 1 n", "入 1 n", "大 1 n", "屏 1 n", "能 1 n", "力 1 n",
		"系 1 n", "统 1 n", "模 1 n", "块 1 n", "监 1 n", "控 1 n", "告 1 n", "警 1 n",
	}
	if err := os.WriteFile(filepath.Join(dir, "s_1.txt"), []byte(strings.Join(entries, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t_1.txt"), []byte("产品 3000000 n\n说明 3000000 n\n本文 3000000 r\n介绍 3000000 v\n只有 3000000 v\n字母 3000000 n\n知识 3000000 n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIWIKI_JIEBA_DICT", dir)

	jiebaMu.Lock()
	jiebaSeg, jiebaInited, jiebaNextTry = nil, false, time.Time{}
	jiebaMu.Unlock()
	initJieba()
	jiebaMu.Lock()
	ok := jiebaInited
	jiebaMu.Unlock()
	if !ok {
		t.Fatal("测试词典加载失败")
	}

	idxDir := t.TempDir()
	if err := InitIndex(filepath.Join(idxDir, "idx")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CloseIndex() })

	docs := []modelsearch.SearchDocument{
		{ID: 1, SpaceID: 1, Name: "AB系统 A模块", Content: "监控 大屏 集团接入 Grafana A 告警", Type: 1, Path: "0"},
		{ID: 2, SpaceID: 1, Name: "产品说明", Content: "本文介绍集团接入大屏能力", Type: 1, Path: "0"},
		{ID: 3, SpaceID: 1, Name: "B模块", Content: "只有 B 字母", Type: 1, Path: "0"},
	}
	for i := range docs {
		if err := IndexDocument(strconv.Itoa(docs[i].ID), &docs[i]); err != nil {
			t.Fatal(err)
		}
	}
}

func tokenTerms(t *testing.T, input string) []string {
	t.Helper()
	stream := NewJiebaTokenizer().Tokenize([]byte(input))
	out := make([]string, 0, len(stream))
	for _, tk := range stream {
		if tk.End > len(input) || tk.Start >= tk.End {
			t.Fatalf("token 偏移越界: %q start=%d end=%d len=%d", tk.Term, tk.Start, tk.End, len(input))
		}
		if string([]byte(input)[tk.Start:tk.End]) != string(tk.Term) {
			t.Fatalf("token 偏移与文本不一致: %q != %q (input=%q)", string([]byte(input)[tk.Start:tk.End]), string(tk.Term), input)
		}
		out = append(out, strings.ToLower(string(tk.Term)))
	}
	return out
}

// 覆盖「单字母、单汉字、字母数字混合、中英混排」四类分词，且偏移必须与原文对齐
func TestTokenizerHandlesSingleCharAndMixedScripts(t *testing.T) {
	setupSearchDict(t)

	assertContains := func(input string, wants ...string) {
		got := strings.Join(tokenTerms(t, input), "|")
		for _, w := range wants {
			if !strings.Contains(got, strings.ToLower(w)) {
				t.Errorf("分词 %q 缺少词项 %q，实际=%v", input, w, got)
			}
		}
	}

	assertContains("A", "a")
	assertContains("AB", "ab")
	assertContains("Grafana2024", "grafana2024", "grafana", "2024")
	// 中英混排：ASCII 段必须切成完整词（不再被逐字母拆开），中文段仍走词典分词
	assertContains("AB系统 A模块", "ab", "a", "系", "统", "模", "块")
	assertContains("集团接入大屏能力", "集团", "接入", "大屏", "集", "能")

	// 纯符号输入也不能产出空 token 流（否则查询侧永远 0 结果）
	if len(tokenTerms(t, "—— ！！")) == 0 {
		t.Error("纯符号输入产出空 token 流")
	}
}

// 回归：单字查询、多词查询、短语查询都要有结果，且不能出现「AB 命中 B模块」这类噪声
func TestSearchSingleCharAndMultiTermQueries(t *testing.T) {
	setupSearchDict(t)

	cases := []struct {
		query    string
		wantMin  uint64
		wantOnly []string // 命中文档 ID 集合（精确断言）
	}{
		{query: "A", wantMin: 1, wantOnly: []string{"1"}},
		{query: "a", wantMin: 1, wantOnly: []string{"1"}},
		{query: "AB", wantMin: 1, wantOnly: []string{"1"}},
		{query: "AB A", wantMin: 1, wantOnly: []string{"1"}},
		{query: "B", wantMin: 1, wantOnly: []string{"3"}},
		{query: "集", wantMin: 1},
		{query: "集团", wantMin: 2},
		{query: "大屏", wantMin: 2},
		{query: "Grafana", wantMin: 1, wantOnly: []string{"1"}},
	}

	for _, tc := range cases {
		res, err := SearchDocumentsGlobal(tc.query, 10)
		if err != nil {
			t.Fatalf("query %q 搜索失败: %v", tc.query, err)
		}
		if res.Total < tc.wantMin {
			t.Errorf("query %q 命中 %d 条，期望 >= %d", tc.query, res.Total, tc.wantMin)
		}
		if tc.wantOnly != nil {
			got := make([]string, 0, len(res.Hits))
			for _, h := range res.Hits {
				got = append(got, h.ID)
			}
			if strings.Join(got, ",") != strings.Join(tc.wantOnly, ",") {
				t.Errorf("query %q 命中集合=%v，期望=%v（精度回归）", tc.query, got, tc.wantOnly)
			}
		}
	}
}

// OR 语义（A|B）与字段限定符（title:）不能被分词改动破坏
func TestSearchOrAndFieldQualifierStillWorks(t *testing.T) {
	setupSearchDict(t)

	res, err := SearchDocumentsGlobal("B|AB系统", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total < 2 {
		t.Errorf("OR 查询应至少命中 2 篇，实际=%d", res.Total)
	}

	res, err = SearchDocumentsGlobal("title:产品", 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total < 1 {
		t.Errorf("title: 限定查询应命中「产品说明」，实际=%d", res.Total)
	}
}
