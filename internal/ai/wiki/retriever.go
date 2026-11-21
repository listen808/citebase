package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
)

// maxCandidates 候选页上限（AWS 实践封顶 7 页）。
const maxCandidates = 7

// Retrieved 一个召回候选。
type Retrieved struct {
	Slug       string  `json:"slug"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	Via        string  `json:"via"`        // directory / fulltext / both
	Confidence float64 `json:"confidence"` // 页面置信度 0–1
}

// Retriever 目录 ∪ 全文 双路召回。
//
// 单靠目录导航会漏：一行摘要天然是有损压缩，保留主题却丢掉具体数字、小众术语和
// 正文深处的操作细节。AWS 实测纯目录选页命中率 70%，叠加 BM25 全文旁路后 92.7%，
// 且逐题核对没有任何一题变差 —— 因此两路独立运行后取**并集**，
// 不让全文结果改写目录结果（让模型综合两路反而会误删正确的目录选择）。
type Retriever struct {
	pageRepo *knowledgerepo.WikiPageRepository
	provider provider.Provider
}

func NewRetriever(p provider.Provider) *Retriever {
	return &Retriever{
		pageRepo: knowledgerepo.NewWikiPageRepository(),
		provider: p,
	}
}

// Retrieve 返回候选页集合（目录路 ∪ 全文路），并按分数降序。
func (r *Retriever) Retrieve(ctx context.Context, spaceID int, question string, fulltextTopK int) ([]Retrieved, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, nil
	}
	if fulltextTopK <= 0 {
		fulltextTopK = 2
	}

	// 只取当前有效页：已发布且未被取代（superseded_by = ''），
	// 被新页取代的旧页不再参与召回，避免答案引用过期内容。
	pages, err := r.pageRepo.ListActivePages(spaceID)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}

	bySlug := make(map[string]*knowledgemodel.WikiPage, len(pages))
	for _, p := range pages {
		bySlug[p.Slug] = p
	}

	via := make(map[string]string, maxCandidates)
	scores := make(map[string]float64, maxCandidates)

	// 路径一：目录导航（LLM 读目录选页）
	for _, slug := range r.selectByDirectory(ctx, spaceID, pages, question) {
		if _, ok := bySlug[slug]; !ok {
			continue
		}
		via[slug] = "directory"
		scores[slug] = 1
	}

	// 路径二：BM25 全文旁路（只补充，不改写目录结果）
	for _, hit := range r.searchFulltext(pages, question, fulltextTopK) {
		if _, ok := via[hit.Slug]; ok {
			via[hit.Slug] = "both"
			scores[hit.Slug] += hit.Score
			continue
		}
		via[hit.Slug] = "fulltext"
		scores[hit.Slug] = hit.Score
	}

	// 路径三：个性化 PageRank 图扩展（N1/HippoRAG-2）。
	// 种子 = 前两路已选中的页；只把 PPR 高分的「邻居页」顶进候选窗口。
	// 分数已归一化到 0–1，量级小于 BM25，是补充信号而非改写信号。
	seeds := make(map[string]float64, len(via))
	for slug, s := range scores {
		if s > 0 {
			seeds[slug] = s
		}
	}
	if norm := normalizeForPPR(seeds); norm != nil {
		for _, cand := range r.graphExpand(ctx, spaceID, pages, norm) {
			if _, ok := bySlug[cand.Slug]; !ok {
				continue
			}
			if _, ok := via[cand.Slug]; ok {
				scores[cand.Slug] += cand.Score
				continue
			}
			via[cand.Slug] = "graph"
			scores[cand.Slug] = cand.Score
		}
	}

	out := make([]Retrieved, 0, len(via))
	for slug, v := range via {
		p := bySlug[slug]
		// 置信度加权：低置信度页面排序靠后，高置信度页面优先呈现
		weighted := scores[slug] * (0.5 + 0.5*p.Confidence)
		out = append(out, Retrieved{
			Slug:       p.Slug,
			Title:      p.Title,
			Summary:    p.Summary,
			Content:    p.Content,
			Score:      weighted,
			Via:        v,
			Confidence: p.Confidence,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out, nil
}

// selectByDirectory 让模型读目录选页。目录按 slug 固定排序，是稳定的 prompt 前缀。
func (r *Retriever) selectByDirectory(ctx context.Context, spaceID int, pages []*knowledgemodel.WikiPage, question string) []string {
	if r.provider == nil || len(pages) == 0 {
		return nil
	}
	var b strings.Builder
	for _, p := range pages {
		fmt.Fprintf(&b, "%s | %s | %s\n", p.Slug, p.Type, p.Summary)
	}
	prompt := fmt.Sprintf("知识目录（slug | 类型 | 摘要）：\n%s\n用户问题：%s\n\n"+
		"只回复最相关的页面 slug，JSON 数组格式，最多 5 个。不要解释。", b.String(), question)

	resp, err := r.provider.Chat(ctx,
		"你是知识库检索助手，负责从目录中挑选与问题最相关的页面。"+purposePromptBlock(spaceID),
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(256), provider.WithTemperature(0))
	if err != nil {
		return nil
	}
	return parseSlugArray(resp.Content)
}

var slugArrayPattern = regexp.MustCompile(`"([^"\s]+)"`)

func parseSlugArray(s string) []string {
	var direct []string
	if err := json.Unmarshal([]byte(extractJSONArray(s)), &direct); err == nil && len(direct) > 0 {
		return direct
	}
	// 退化：模型可能没严格输出 JSON，抓引号内的 token 也能用
	matches := slugArrayPattern.FindAllStringSubmatch(s, 5)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return "[]"
	}
	return s[start : end+1]
}

// searchFulltext 在页面正文上做 BM25 检索。
// 中文没有空格分词，采用 CJK 二元组切分（"检索增强" → [检索, 索增, 增强]），
// 这是中文全文检索的低成本有效方案。
func (r *Retriever) searchFulltext(pages []*knowledgemodel.WikiPage, question string, topK int) []Retrieved {
	qTerms := tokenize(question)
	if len(qTerms) == 0 {
		return nil
	}

	docs := make([][]string, 0, len(pages))
	kept := make([]*knowledgemodel.WikiPage, 0, len(pages))
	totalLen := 0
	for _, p := range pages {
		text := p.Title + " " + p.Summary + " " + p.Content
		terms := tokenize(text)
		if len(terms) == 0 {
			continue
		}
		docs = append(docs, terms)
		kept = append(kept, p)
		totalLen += len(terms)
	}
	if len(docs) == 0 {
		return nil
	}
	avgdl := float64(totalLen) / float64(len(docs))

	// 文档频率
	df := make(map[string]int)
	for _, d := range docs {
		seen := make(map[string]bool)
		for _, t := range d {
			if !seen[t] {
				seen[t] = true
				df[t]++
			}
		}
	}

	type hit struct {
		page  *knowledgemodel.WikiPage
		score float64
	}
	hits := make([]hit, 0, len(docs))
	const k1, b = 1.5, 0.75
	for i, d := range docs {
		tf := make(map[string]int)
		for _, t := range d {
			tf[t]++
		}
		var score float64
		for _, qt := range qTerms {
			n := tf[qt]
			if n == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(docs))-float64(df[qt])+0.5)/(float64(df[qt])+0.5))
			score += idf * (float64(n) * (k1 + 1)) /
				(float64(n) + k1*(1-b+b*float64(len(d))/avgdl))
		}
		if score > 0 {
			hits = append(hits, hit{page: kept[i], score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > topK {
		hits = hits[:topK]
	}

	out := make([]Retrieved, 0, len(hits))
	for _, h := range hits {
		out = append(out, Retrieved{
			Slug:    h.page.Slug,
			Title:   h.page.Title,
			Summary: h.page.Summary,
			Content: h.page.Content,
			Score:   h.score,
			Via:     "fulltext",
		})
	}
	return out
}

// tokenize 中文按二元组切分，拉丁文按词切分。
func tokenize(s string) []string {
	s = strings.ToLower(s)
	runes := []rune(s)
	var out []string
	var latin strings.Builder
	flush := func() {
		if latin.Len() > 0 {
			out = append(out, latin.String())
			latin.Reset()
		}
	}
	var prevCJK bool
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
			flush()
			if i+1 < len(runes) {
				out = append(out, string(runes[i:i+2]))
			}
			prevCJK = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			latin.WriteRune(r)
			prevCJK = false
		default:
			flush()
			_ = prevCJK
			prevCJK = false
		}
	}
	flush()
	return out
}
