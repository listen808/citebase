package middleware

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
	Cited         bool      `json:"cited,omitempty"`
	VerifyStatus  string    `json:"verify_status,omitempty"`
	DocURL        string    `json:"doc_url,omitempty"`
}

type ChatMessage struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type RAGDebugInfo struct {
	OriginalQuestion  string             `json:"original_question"`
	RewrittenQuestion string             `json:"rewritten_question,omitempty"`
	SubQuestions      []string           `json:"sub_questions,omitempty"`
	RetrievalSteps    []RAGRetrievalStep `json:"retrieval_steps"`
	CacheHit          bool               `json:"cache_hit"`
	TotalDurationMs   int64              `json:"total_duration_ms"`
}

type RAGRetrievalStep struct {
	Step        string `json:"step"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DurationMs  int64  `json:"duration_ms"`
	ResultCount int    `json:"result_count,omitempty"`
}

type RAGContext struct {
	Ctx            context.Context
	Question       string
	SpaceID        int
	Limit          int
	History        []ChatMessage
	Sources        []Source
	RewrittenQ     string
	SubQuestions   []string
	DebugInfo      *RAGDebugInfo
	Answer         string
	TokenUsage     map[string]int64
	Err            error
	SkipGeneration bool
	Data           map[string]interface{}
}

type RAGMiddleware interface {
	Name() string
	BeforeRetrieval(ctx *RAGContext)
	AfterRetrieval(ctx *RAGContext)
	BeforeGeneration(ctx *RAGContext)
	AfterGeneration(ctx *RAGContext)
}
