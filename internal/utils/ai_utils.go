package utils

import (
	"math"
	"strings"
)

func ExtractJSON(content string) string {
	if idx := strings.Index(content, "```json"); idx >= 0 {
		start := idx + 7
		if end := strings.Index(content[start:], "```"); end >= 0 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	if idx := strings.Index(content, "{"); idx >= 0 {
		trimmed := content[idx:]
		depth := 0
		for i, ch := range trimmed {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					return trimmed[:i+1]
				}
			}
		}
		return trimmed
	}
	return ""
}

// ExtractJSONArray 从文本中提取 JSON 数组（支持 [] 包裹）
func ExtractJSONArray(content string) string {
	if idx := strings.Index(content, "["); idx >= 0 {
		trimmed := content[idx:]
		depth := 0
		for i, ch := range trimmed {
			if ch == '[' {
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					return trimmed[:i+1]
				}
			}
		}
		return trimmed
	}
	return "[]"
}

// ExtractJSONField 从 JSON 字符串中简单提取指定字段值（不依赖标准库 JSON 解析）
func ExtractJSONField(jsonStr, fieldName string) string {
	key := `"` + fieldName + `"`
	idx := strings.Index(jsonStr, key)
	if idx < 0 {
		return ""
	}
	rest := jsonStr[idx+len(key):]
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[colonIdx+1:])

	if strings.HasPrefix(rest, `"`) {
		rest = rest[1:]
		endIdx := strings.Index(rest, `"`)
		if endIdx < 0 {
			return rest
		}
		return rest[:endIdx]
	}

	endIdx := strings.IndexAny(rest, ",}\n")
	if endIdx < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:endIdx])
}

func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// CosineSimilarityFloat32 计算 float32 向量的余弦相似度
func CosineSimilarityFloat32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
