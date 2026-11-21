package rag

import (
	"sort"
	"strings"
)

// RelevantSegmentExtractor 相关段落提取器
// 动态构建多chunk相关段落，为LLM提供更完整的上下文
type RelevantSegmentExtractor struct {
	windowSize int // 上下文窗口大小（前后各扩展的chunk数）
	maxLen     int // 最大输出长度
}

func NewRelevantSegmentExtractor(windowSize, maxLen int) *RelevantSegmentExtractor {
	if windowSize <= 0 {
		windowSize = 1
	}
	if maxLen <= 0 {
		maxLen = 3000
	}
	return &RelevantSegmentExtractor{
		windowSize: windowSize,
		maxLen:     maxLen,
	}
}

// ExtractSegments 从检索结果中提取相关段落
func (r *RelevantSegmentExtractor) ExtractSegments(sources []Source, allChunks []Chunk) []string {
	if len(sources) == 0 {
		return nil
	}

	// 按分数排序
	sorted := make([]Source, len(sources))
	copy(sorted, sources)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	// 为每个高分source扩展上下文窗口
	seen := make(map[int]bool)
	var segments []string
	totalLen := 0

	for _, src := range sorted {
		if totalLen >= r.maxLen {
			break
		}

		// 找到对应的chunk索引
		chunkIdx := r.findChunkIndex(allChunks, src)
		if chunkIdx < 0 {
			// 直接使用snippet
			if !seen[-1] || src.Score > 0.7 {
				segment := src.Snippet
				if len(segment) > 0 && totalLen+len(segment) <= r.maxLen {
					segments = append(segments, segment)
					totalLen += len(segment)
				}
			}
			continue
		}

		// 扩展上下文窗口
		start := chunkIdx - r.windowSize
		if start < 0 {
			start = 0
		}
		end := chunkIdx + r.windowSize + 1
		if end > len(allChunks) {
			end = len(allChunks)
		}

		for i := start; i < end; i++ {
			if seen[i] {
				continue
			}
			seen[i] = true

			chunk := allChunks[i]
			segment := chunk.Content
			if totalLen+len(segment) > r.maxLen {
				remaining := r.maxLen - totalLen
				if remaining > 100 {
					segment = truncateForRSE(segment, remaining)
				} else {
					break
				}
			}
			segments = append(segments, segment)
			totalLen += len(segment)
		}
	}

	return segments
}

// findChunkIndex 找到source对应的chunk索引
func (r *RelevantSegmentExtractor) findChunkIndex(chunks []Chunk, src Source) int {
	for i, chunk := range chunks {
		if chunk.StartLine == src.StartLine || chunk.HeadingPath == src.HeadingPath {
			return i
		}
	}
	// 通过内容匹配
	for i, chunk := range chunks {
		if strings.Contains(chunk.Content, src.Snippet) || strings.Contains(src.Snippet, chunk.Content) {
			return i
		}
	}
	return -1
}

func truncateForRSE(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
