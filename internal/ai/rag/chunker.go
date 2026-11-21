package rag

import (
	"regexp"
	"strings"
)

type Chunk struct {
	Content     string
	StartLine   int
	EndLine     int
	HeadingPath string
}

type Chunker interface {
	Chunk(content string) []Chunk
}

type MarkdownChunker struct {
	maxChunkSize int
}

func NewMarkdownChunker(maxChunkSize int) *MarkdownChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 1500
	}
	return &MarkdownChunker{maxChunkSize: maxChunkSize}
}

var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)`)

func (mc *MarkdownChunker) Chunk(content string) []Chunk {
	if content == "" {
		return nil
	}

	sections := mc.splitByHeadings(content)
	var chunks []Chunk

	for i, sec := range sections {
		if len(sec.Content) <= mc.maxChunkSize {
			chunk := Chunk{
				Content:     sec.Content,
				StartLine:   sec.StartLine,
				EndLine:     sec.EndLine,
				HeadingPath: sec.Heading,
			}
			chunk.Content = mc.addContext(chunks, sec, i, sections)
			chunks = append(chunks, chunk)
		} else {
			subChunks := mc.splitOversizedSection(sec, i, sections, chunks)
			chunks = append(chunks, subChunks...)
		}
	}

	return chunks
}

type section struct {
	Content   string
	Heading   string
	StartLine int
	EndLine   int
}

func (mc *MarkdownChunker) splitByHeadings(content string) []section {
	lines := strings.Split(content, "\n")
	var sections []section
	var current section
	currentLine := 1

	for _, line := range lines {
		if matches := headingRegex.FindStringSubmatch(line); matches != nil {
			if current.Content != "" {
				current.EndLine = currentLine - 1
				sections = append(sections, current)
			}
			current = section{
				Content:   line,
				Heading:   strings.TrimSpace(matches[2]),
				StartLine: currentLine,
			}
		} else {
			if current.Content == "" {
				current = section{
					Content:   line,
					StartLine: currentLine,
				}
			} else {
				current.Content += "\n" + line
			}
		}
		currentLine++
	}

	if current.Content != "" {
		current.EndLine = currentLine - 1
		sections = append(sections, current)
	}

	return sections
}

func (mc *MarkdownChunker) addContext(existingChunks []Chunk, sec section, idx int, sections []section) string {
	result := sec.Content

	if idx > 0 {
		prev := sections[idx-1]
		prevLines := strings.Split(prev.Content, "\n")
		contextLen := 0
		var contextLines []string
		for i := len(prevLines) - 1; i >= 0 && contextLen < 100; i-- {
			lineLen := len(prevLines[i])
			if contextLen+lineLen > 100 {
				break
			}
			contextLines = append([]string{prevLines[i]}, contextLines...)
			contextLen += lineLen + 1
		}
		if len(contextLines) > 0 {
			result = strings.Join(contextLines, "\n") + "\n---\n" + result
		}
	}

	if idx < len(sections)-1 {
		next := sections[idx+1]
		nextLines := strings.Split(next.Content, "\n")
		contextLen := 0
		var contextLines []string
		for i := 0; i < len(nextLines) && contextLen < 100; i++ {
			lineLen := len(nextLines[i])
			if contextLen+lineLen > 100 {
				break
			}
			contextLines = append(contextLines, nextLines[i])
			contextLen += lineLen + 1
		}
		if len(contextLines) > 0 {
			result = result + "\n---\n" + strings.Join(contextLines, "\n")
		}
	}

	return result
}

func (mc *MarkdownChunker) splitOversizedSection(sec section, idx int, sections []section, existingChunks []Chunk) []Chunk {
	paragraphs := strings.Split(sec.Content, "\n\n")
	var chunks []Chunk
	var current strings.Builder
	currentLen := 0
	startLine := sec.StartLine

	for _, para := range paragraphs {
		paraLen := len(para)
		if currentLen+paraLen+2 > mc.maxChunkSize && currentLen > 0 {
			content := current.String()
			content = mc.addChunkContext(content, idx, sections)
			chunks = append(chunks, Chunk{
				Content:     content,
				StartLine:   startLine,
				EndLine:     startLine + strings.Count(content, "\n"),
				HeadingPath: sec.Heading,
			})
			startLine += strings.Count(current.String(), "\n") + 1
			current.Reset()
			currentLen = 0
		}
		if currentLen > 0 {
			current.WriteString("\n\n")
			currentLen += 2
		}
		current.WriteString(para)
		currentLen += paraLen
	}

	if currentLen > 0 {
		content := current.String()
		content = mc.addChunkContext(content, idx, sections)
		chunks = append(chunks, Chunk{
			Content:     content,
			StartLine:   startLine,
			EndLine:     sec.EndLine,
			HeadingPath: sec.Heading,
		})
	}

	return chunks
}

func (mc *MarkdownChunker) addChunkContext(content string, idx int, sections []section) string {
	if idx > 0 {
		prev := sections[idx-1]
		prevLines := strings.Split(prev.Content, "\n")
		contextLen := 0
		var contextLines []string
		for i := len(prevLines) - 1; i >= 0 && contextLen < 100; i-- {
			lineLen := len(prevLines[i])
			if contextLen+lineLen > 100 {
				break
			}
			contextLines = append([]string{prevLines[i]}, contextLines...)
			contextLen += lineLen + 1
		}
		if len(contextLines) > 0 {
			content = strings.Join(contextLines, "\n") + "\n---\n" + content
		}
	}
	if idx < len(sections)-1 {
		next := sections[idx+1]
		nextLines := strings.Split(next.Content, "\n")
		contextLen := 0
		var contextLines []string
		for i := 0; i < len(nextLines) && contextLen < 100; i++ {
			lineLen := len(nextLines[i])
			if contextLen+lineLen > 100 {
				break
			}
			contextLines = append(contextLines, nextLines[i])
			contextLen += lineLen + 1
		}
		if len(contextLines) > 0 {
			content = content + "\n---\n" + strings.Join(contextLines, "\n")
		}
	}
	return content
}

type RecursiveChunker struct {
	maxChunkSize int
}

func NewRecursiveChunker(maxChunkSize int) *RecursiveChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 1500
	}
	return &RecursiveChunker{maxChunkSize: maxChunkSize}
}

func (rc *RecursiveChunker) Chunk(content string) []Chunk {
	if content == "" {
		return nil
	}
	if len(content) <= rc.maxChunkSize {
		return []Chunk{{Content: content, StartLine: 1, EndLine: strings.Count(content, "\n") + 1}}
	}

	separators := []string{"\n\n", "\n", "。", ".", "！", "！", "；", ";", " "}
	var chunks []Chunk
	rc.recursiveSplit(content, separators, 0, &chunks)

	line := 1
	for i := range chunks {
		chunks[i].StartLine = line
		lineCount := strings.Count(chunks[i].Content, "\n")
		chunks[i].EndLine = line + lineCount
		line = chunks[i].EndLine + 1
	}

	chunks = rc.addOverlap(chunks)
	return chunks
}

func (rc *RecursiveChunker) addOverlap(chunks []Chunk) []Chunk {
	if len(chunks) <= 1 {
		return chunks
	}
	overlapSize := rc.maxChunkSize / 7
	if overlapSize < 50 {
		overlapSize = 50
	}
	result := make([]Chunk, len(chunks))
	for i, c := range chunks {
		if i > 0 {
			prevContent := chunks[i-1].Content
			runes := []rune(prevContent)
			if len(runes) > overlapSize {
				c.Content = string(runes[len(runes)-overlapSize:]) + "\n" + c.Content
			}
		}
		result[i] = c
	}
	return result
}

func (rc *RecursiveChunker) recursiveSplit(text string, separators []string, depth int, chunks *[]Chunk) {
	if len(text) <= rc.maxChunkSize {
		*chunks = append(*chunks, Chunk{Content: text})
		return
	}

	if depth >= len(separators) {
		runes := []rune(text)
		for i := 0; i < len(runes); i += rc.maxChunkSize {
			end := i + rc.maxChunkSize
			if end > len(runes) {
				end = len(runes)
			}
			*chunks = append(*chunks, Chunk{Content: string(runes[i:end])})
		}
		return
	}

	sep := separators[depth]
	parts := strings.Split(text, sep)

	var current strings.Builder
	currentLen := 0

	for _, part := range parts {
		partLen := len(part)

		if partLen > rc.maxChunkSize {
			if currentLen > 0 {
				*chunks = append(*chunks, Chunk{Content: current.String()})
				current.Reset()
				currentLen = 0
			}
			rc.recursiveSplit(part, separators, depth+1, chunks)
			continue
		}

		if currentLen+partLen+len(sep) > rc.maxChunkSize && currentLen > 0 {
			*chunks = append(*chunks, Chunk{Content: current.String()})
			current.Reset()
			currentLen = 0
		}

		if currentLen > 0 {
			current.WriteString(sep)
			currentLen += len(sep)
		}
		current.WriteString(part)
		currentLen += partLen
	}

	if currentLen > 0 {
		*chunks = append(*chunks, Chunk{Content: current.String()})
	}
}
