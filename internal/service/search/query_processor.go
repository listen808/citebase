package search

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// QueryIntent classifies the user's search query.
type QueryIntent string

const (
	// IntentKeyword is a keyword-based query (short, no question words).
	IntentKeyword QueryIntent = "keyword"
	// IntentQuestion is a natural language question.
	IntentQuestion QueryIntent = "question"
	// IntentHybrid contains both keyword and natural language elements.
	IntentHybrid QueryIntent = "hybrid"
)

// SearchMode constants for the hybrid search system.
const (
	ModeHybrid   = "hybrid"
	ModeFulltext = "fulltext"
	ModeSemantic = "semantic"
)

// questionWords are common Chinese and English question indicators.
var questionWords = []string{
	"什么", "怎么", "如何", "为什么", "哪些", "哪里", "哪个", "多少",
	"是否", "能否", "可以", "怎样", "什么时候", "请问", "是什么",
	"what", "how", "why", "which", "where", "when", "who", "can", "is", "are", "do", "does",
}

// questionMarks are punctuation marks that indicate questions.
var questionMarks = []string{"?", "？"}

// DetectIntent determines if a query is keyword-based, a natural language question, or hybrid.
func DetectIntent(query string) QueryIntent {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return IntentKeyword
	}

	lower := strings.ToLower(trimmed)

	// Check for question marks
	for _, mark := range questionMarks {
		if strings.Contains(trimmed, mark) {
			return IntentQuestion
		}
	}

	// Count question words found
	questionWordCount := 0
	for _, word := range questionWords {
		if strings.Contains(lower, word) {
			questionWordCount++
		}
	}

	// Word count (approximate for CJK)
	wordCount := countWords(trimmed)

	// Short queries (1-3 words) without question words are keyword queries
	if wordCount <= 3 && questionWordCount == 0 {
		return IntentKeyword
	}

	// Queries with multiple question words or long natural language are questions
	if questionWordCount >= 1 && wordCount >= 4 {
		return IntentQuestion
	}

	// Long queries without question words are hybrid
	if wordCount > 5 {
		return IntentHybrid
	}

	return IntentKeyword
}

// SelectSearchMode decides the search mode based on query intent and user-requested mode.
// If the user explicitly requests a mode (non-empty, not "hybrid"), that mode is used.
// Otherwise, the system selects based on intent.
func SelectSearchMode(intent QueryIntent, requestedMode string) string {
	// User explicitly chose a specific mode
	if requestedMode != "" && requestedMode != ModeHybrid {
		return requestedMode
	}

	switch intent {
	case IntentKeyword:
		return ModeFulltext
	case IntentQuestion:
		return ModeSemantic
	case IntentHybrid:
		return ModeHybrid
	default:
		return ModeHybrid
	}
}

// ExpandQuery adds synonyms and related terms to the query.
// Currently returns the original query as-is; synonym expansion is handled
// by the existing synonym system in the API layer.
func ExpandQuery(query string) string {
	return strings.TrimSpace(query)
}

// countWords approximates word count for mixed CJK and Latin text.
// CJK characters each count as one word; Latin words are split by whitespace.
func countWords(text string) int {
	count := 0
	inLatin := false

	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if unicode.IsSpace(r) {
			if inLatin {
				count++
				inLatin = false
			}
			i += size
			continue
		}

		if isCJKChar(r) {
			if inLatin {
				count++
				inLatin = false
			}
			count++
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			inLatin = true
		}
		i += size
	}

	if inLatin {
		count++
	}

	return count
}

// isCJKChar checks if a rune is a CJK ideograph (Han, Hiragana, Katakana, or Hangul).
func isCJKChar(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}
