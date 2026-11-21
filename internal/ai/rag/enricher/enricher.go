package enricher

import "context"

type Source struct {
	DocumentID    int       `json:"document_id"`
	DocumentName  string    `json:"document_name"`
	SpaceID       int       `json:"space_id"`
	Score         float64   `json:"score"`
	Snippet       string    `json:"snippet"`
	StartLine     int       `json:"start_line"`
	EndLine       int       `json:"end_line"`
	HeadingPath   string    `json:"heading_path"`
	Keywords      []string  `json:"keywords"`
	CitationIndex int       `json:"citation_index"`
	UpdatedAt     int64     `json:"updated_at"`
	Embedding     []float64 `json:"embedding,omitempty"`
	SourceType    string    `json:"source_type,omitempty"`
}

type ContextEnricher interface {
	Name() string
	Enrich(ctx context.Context, question string, sources []Source) ([]Source, string)
}
