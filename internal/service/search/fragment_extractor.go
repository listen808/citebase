package search

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2/search"
	modelsearch "github.com/listen808/citebase/internal/model/search"
)

// matchLoc 匹配位置（内部使用）
type matchLoc struct {
	Start int
	End   int
	Term  string
}

// matchCluster 匹配簇（内部使用）
type matchCluster struct {
	Start   int
	End     int
	Terms   map[string]bool
	Matches int
}

// ExtractFragments 从 Bleve Locations 提取多个高亮片段
// locations: Bleve 返回的 TermLocationMap（特定字段的匹配位置）
// content: 原文内容
// fragmentSize: 每个片段的字符数（默认 150）
// numFragments: 最多返回的片段数（默认 3）
func ExtractFragments(
	locations search.TermLocationMap,
	content string,
	fragmentSize int,
	numFragments int,
) []modelsearch.HighlightFragment {
	if fragmentSize <= 0 {
		fragmentSize = 150
	}
	if numFragments <= 0 {
		numFragments = 3
	}

	if len(locations) == 0 {
		return nil
	}

	// 收集该字段的所有匹配位置。
	// Bleve TermLocation 的 Start/End 是原文的 UTF-8 字节偏移，而下方切片用的是
	// []rune 下标：中文（每字 3 字节）会让位置放大最多 3 倍、高亮与命中词错位。
	// 统一在入口处把字节偏移换算成 rune 下标。
	var matches []matchLoc
	for term, locs := range locations {
		for _, loc := range locs {
			matches = append(matches, matchLoc{
				Start: byteToRuneIndex(content, int(loc.Start)),
				End:   byteToRuneIndex(content, int(loc.End)),
				Term:  term,
			})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// 按起始位置排序
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})

	// 合并相邻/重叠的匹配簇
	var clusters []matchCluster
	for _, m := range matches {
		merged := false
		for i := range clusters {
			c := &clusters[i]
			mergeDistance := fragmentSize / 2
			if m.Start <= c.End+mergeDistance {
				if m.End > c.End {
					c.End = m.End
				}
				c.Terms[m.Term] = true
				c.Matches++
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, matchCluster{
				Start:   m.Start,
				End:     m.End,
				Terms:   map[string]bool{m.Term: true},
				Matches: 1,
			})
		}
	}

	// 提取片段
	contentRunes := []rune(content)
	contentLen := len(contentRunes)

	type scoredFragment struct {
		fragment  modelsearch.HighlightFragment
		termCount int
	}
	var fragments []scoredFragment

	for _, c := range clusters {
		centerPos := (c.Start + c.End) / 2
		halfSize := fragmentSize / 2

		startRune := centerPos - halfSize
		if startRune < 0 {
			startRune = 0
		}
		endRune := startRune + fragmentSize
		if endRune > contentLen {
			endRune = contentLen
			startRune = endRune - fragmentSize
			if startRune < 0 {
				startRune = 0
			}
		}

		fragmentText := string(contentRunes[startRune:endRune])

		// 收集片段内的匹配位置
		var fragmentMatches []matchLoc
		for _, m := range matches {
			if m.Start >= startRune && m.End <= endRune {
				fragmentMatches = append(fragmentMatches, m)
			} else if m.Start < endRune && m.End > startRune {
				fragmentMatches = append(fragmentMatches, m)
			}
		}

		fragmentHTML := buildHighlightedHTML(fragmentText, fragmentMatches, startRune)

		// 智能评分：考虑关键词密度和多样性
		// - 不同关键词数量越多，得分越高
		// - 匹配次数越多，得分越高
		// - 片段长度越短且包含多个关键词，密度越高
		termCount := len(c.Terms)
		fragmentLen := float64(fragmentSize)
		// 密度分：不同关键词数 / 片段长度 * 100
		densityScore := float64(termCount) / fragmentLen * 100
		// 匹配分：总匹配次数 * 0.5
		matchScore := float64(c.Matches) * 0.5
		// 多样性加分：多个不同关键词时给予额外加分
		diversityBonus := float64(termCount) * 2.0
		score := densityScore + matchScore + diversityBonus

		fragments = append(fragments, scoredFragment{
			fragment: modelsearch.HighlightFragment{
				Text:  fragmentText,
				HTML:  fragmentHTML,
				Score: score,
			},
			termCount: termCount,
		})
	}

	// 按得分排序（多关键词优先，其次按密度）
	sort.Slice(fragments, func(i, j int) bool {
		if fragments[i].termCount != fragments[j].termCount {
			return fragments[i].termCount > fragments[j].termCount
		}
		return fragments[i].fragment.Score > fragments[j].fragment.Score
	})

	result := make([]modelsearch.HighlightFragment, 0, numFragments)
	for i, f := range fragments {
		if i >= numFragments {
			break
		}
		result = append(result, f.fragment)
	}

	return result
}

// byteToRuneIndex 把 content 中的 UTF-8 字节偏移换算为 rune 下标（越界时钳制）。
func byteToRuneIndex(content string, bytePos int) int {
	if bytePos <= 0 {
		return 0
	}
	if bytePos >= len(content) {
		return utf8.RuneCountInString(content)
	}
	return utf8.RuneCountInString(content[:bytePos])
}

// buildHighlightedHTML 构建带高亮标记的 HTML
func buildHighlightedHTML(text string, matches []matchLoc, offset int) string {
	if len(matches) == 0 {
		return escapeHTMLFrag(text)
	}

	textRunes := []rune(text)
	textLen := len(textRunes)

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Start < matches[j].Start
	})

	type highlightRange struct {
		Start int
		End   int
	}
	var ranges []highlightRange

	for _, m := range matches {
		start := m.Start - offset
		end := m.End - offset
		if start < 0 {
			start = 0
		}
		if end > textLen {
			end = textLen
		}
		if start >= end {
			continue
		}

		if len(ranges) > 0 {
			last := &ranges[len(ranges)-1]
			if start <= last.End {
				if end > last.End {
					last.End = end
				}
				continue
			}
		}
		ranges = append(ranges, highlightRange{Start: start, End: end})
	}

	var result strings.Builder
	pos := 0
	for _, r := range ranges {
		if r.Start > pos {
			result.WriteString(escapeHTMLFrag(string(textRunes[pos:r.Start])))
		}
		result.WriteString("<mark>")
		result.WriteString(escapeHTMLFrag(string(textRunes[r.Start:r.End])))
		result.WriteString("</mark>")
		pos = r.End
	}
	if pos < textLen {
		result.WriteString(escapeHTMLFrag(string(textRunes[pos:])))
	}

	return result.String()
}

// escapeHTMLFrag 转义 HTML 特殊字符
func escapeHTMLFrag(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#039;")
	return s
}

// CountMatches 统计匹配总数（遍历所有字段）
func CountMatches(locations search.FieldTermLocationMap) int {
	count := 0
	for _, termLocMap := range locations {
		for _, locs := range termLocMap {
			count += len(locs)
		}
	}
	return count
}
