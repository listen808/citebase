package provider

import (
	"bufio"
	"io"
	"strings"
)

type sseEvent struct {
	Data string
}

type sseReader struct {
	scanner *bufio.Scanner
}

func newSSEReader(body io.Reader) *sseReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	return &sseReader{scanner: scanner}
}

func (r *sseReader) Next() (*sseEvent, error) {
	var data strings.Builder
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			if data.Len() > 0 {
				return &sseEvent{Data: data.String()}, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data.WriteString(strings.TrimPrefix(line, "data: "))
		} else if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimPrefix(line, "data:"))
		}
	}
	if data.Len() > 0 {
		return &sseEvent{Data: data.String()}, nil
	}
	return nil, r.scanner.Err()
}
