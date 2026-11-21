package shared

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// StopWords 中文停用词表（统一版本）
var StopWords = map[string]bool{
	"的": true, "了": true, "是": true, "在": true, "有": true,
	"和": true, "与": true, "或": true, "把": true, "被": true,
	"让": true, "给": true, "对": true, "从": true, "到": true,
	"这": true, "那": true, "我": true, "你": true, "他": true,
	"她": true, "它": true, "们": true, "都": true, "也": true,
	"就": true, "不": true, "会": true, "能": true, "可以": true,
	"但": true, "而": true, "且": true, "虽": true, "如果": true,
	"因为": true, "所以": true, "然后": true, "接着": true,
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"dare": true, "ought": true, "used": true, "to": true, "of": true,
	"in": true, "for": true, "on": true, "with": true, "at": true,
	"by": true, "from": true, "as": true, "into": true, "through": true,
	"during": true, "before": true, "after": true, "above": true, "below": true,
	"between": true, "out": true, "off": true, "over": true, "under": true,
	"again": true, "further": true, "then": true, "once": true, "and": true,
	"but": true, "or": true, "nor": true, "not": true, "so": true,
	"very": true, "just": true, "than": true, "too": true, "also": true,
}

// ExtractKeywords 提取关键词（统一实现）
// text: 输入文本
// maxKeywords: 最大关键词数
// minLen: 最小词长
// maxLen: 最大词长
func ExtractKeywords(text string, maxKeywords, minLen, maxLen int) []string {
	// 提取中文词
	chineseWords := extractChineseNgrams(text, minLen, maxLen)
	// 提取英文词
	englishWords := extractEnglishWords(text, minLen, maxLen)

	// 合并并计算频率
	freq := make(map[string]int)
	for _, w := range chineseWords {
		if !StopWords[w] {
			freq[w]++
		}
	}
	for _, w := range englishWords {
		lower := strings.ToLower(w)
		if !StopWords[lower] && len(lower) >= minLen {
			freq[lower]++
		}
	}

	// 按频率排序
	type wordFreq struct {
		word  string
		count int
	}
	var sorted []wordFreq
	for word, count := range freq {
		sorted = append(sorted, wordFreq{word, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// 返回 top N
	var result []string
	for i := 0; i < len(sorted) && i < maxKeywords; i++ {
		result = append(result, sorted[i].word)
	}
	return result
}

// extractChineseNgrams 提取中文 n-gram
func extractChineseNgrams(text string, minLen, maxLen int) []string {
	var result []string
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		if unicode.Is(unicode.Han, runes[i]) {
			for n := minLen; n <= maxLen && i+n <= len(runes); n++ {
				ngram := string(runes[i : i+n])
				// 检查是否全是中文
				allChinese := true
				for _, r := range ngram {
					if !unicode.Is(unicode.Han, r) {
						allChinese = false
						break
					}
				}
				if allChinese {
					result = append(result, ngram)
				}
			}
		}
	}
	return result
}

// extractEnglishWords 提取英文词
func extractEnglishWords(text string, minLen, maxLen int) []string {
	re := regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9_]*`)
	words := re.FindAllString(text, -1)
	var result []string
	for _, w := range words {
		if len(w) >= minLen && len(w) <= maxLen {
			result = append(result, w)
		}
	}
	return result
}
