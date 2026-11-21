package analyzer

import "context"

type AnalysisTask struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	TargetSpaceIDs []int           `json:"target_space_ids"`
	TargetDocID    int             `json:"target_doc_id"`
	TargetDocIDs   []int           `json:"target_doc_ids,omitempty"` // P0-1: 多文档分析
	ModifiedSince  int64           `json:"modified_since,omitempty"` // P0-1: 日期范围过滤（Unix 时间戳）
	Status         string          `json:"status"`
	Progress       float64         `json:"progress"`
	Message        string          `json:"message"`
	TotalDocs      int             `json:"total_docs"`
	ProcessedDocs  int             `json:"processed_docs"`
	TokenUsage     int             `json:"token_usage"`
	CreatedAt      int64           `json:"created_at"`
	StartedAt      int64           `json:"started_at"`
	CompletedAt    int64           `json:"completed_at"`
	Result         *AnalysisResult `json:"result,omitempty"`
	cancel         context.CancelFunc
	Checkpoint     *AnalysisCheckpoint `json:"checkpoint,omitempty"` // S8: 断点信息
	// M-5: 上一次分析结果的内容哈希映射（docID → contentHash），用于增量差异比对
	CheckpointPrevHashes map[int]string `json:"-"`
}

// S8: AnalysisCheckpoint 断点续分析信息
type AnalysisCheckpoint struct {
	LastBatchIndex  int    `json:"last_batch_index"`
	ProcessedDocIDs []int  `json:"processed_doc_ids"`
	FailedDocIDs    []int  `json:"failed_doc_ids"`
	Phase           string `json:"phase"` // batch/extract/quality/cross_batch/normalize
}

type AnalysisResult struct {
	TotalDocs     int                 `json:"total_docs"`
	ProcessedDocs int                 `json:"processed_docs"`
	Categories    []Category          `json:"categories"`
	Summaries     []DocSummary        `json:"summaries"`
	Relations     []KnowledgeRelation `json:"relations"`
	Themes        []ThemeSummary      `json:"themes"`
	Gaps          []KnowledgeGap      `json:"gaps"` // S9: 知识缺口
	GeneratedAt   int64               `json:"generated_at"`

	// 新增分析结果字段
	ExtractedRelations []ExtractedRelation `json:"extracted_relations,omitempty"` // 关系抽取结果
	ExtractedEvents    []ExtractedEvent    `json:"extracted_events,omitempty"`    // 事件抽取结果
	SentimentResults   []DocSentiment      `json:"sentiment_results,omitempty"`   // 情感分析结果
	KeywordResults     []DocKeywords       `json:"keyword_results,omitempty"`     // 关键词提取结果
	KnowledgeGraph     *KnowledgeGraph     `json:"knowledge_graph,omitempty"`     // 知识图谱
}

// DocSentiment 文档情感分析结果
type DocSentiment struct {
	DocID     int              `json:"doc_id"`
	Sentiment *SentimentResult `json:"sentiment,omitempty"`
}

// DocKeywords 文档关键词提取结果
type DocKeywords struct {
	DocID    int                      `json:"doc_id"`
	Keywords *KeywordExtractionResult `json:"keywords,omitempty"`
}

type Category struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DocIDs      []int  `json:"doc_ids"`
	SpaceIDs    []int  `json:"space_ids"`
}

type DocSummary struct {
	SourceDocID   int      `json:"source_doc_id"`
	SourceSpaceID int      `json:"source_space_id"`
	SourceName    string   `json:"source_name"`
	SpaceName     string   `json:"space_name"`
	Category      string   `json:"category"`
	Summary       string   `json:"summary"`
	KeyPoints     []string `json:"key_points"`
	Tags          []string `json:"tags"`
	Confidence    string   `json:"confidence"`

	// Phase1 字段
	URLs        []URLInfo    `json:"urls"`
	ConfigItems []ConfigItem `json:"config_items"`
	Aliases     []string     `json:"aliases"`
	ActionItems []string     `json:"action_items"`

	// Phase2 优化：保留源文档完整内容与结构化章节
	FullContent string    `json:"full_content"`  // 源文档完整原文
	Sections    []Section `json:"sections"`      // 按标题分段的章节
	HasFullText bool      `json:"has_full_text"` // 是否保留了全文
	ContentSize int       `json:"content_size"`  // 源文档字符数

	// P0: 实体提取与质量评分集成
	Quality *QualityScore `json:"quality,omitempty"` // 质量评估结果（仅低置信度时填充）

	// M-18: 文档更新时间和过期预警
	UpdateTime     int    `json:"update_time,omitempty"`     // 源文档最后更新时间（Unix时间戳）
	ExpiredWarning string `json:"expired_warning,omitempty"` // 过期警告描述，空=未过期

	// M-5: 内容哈希，用于增量差异分析
	ContentHash string `json:"content_hash"` // SHA256 内容哈希
}

// Section 表示文档的一个结构化章节
type Section struct {
	Heading string `json:"heading"` // 章节标题
	Level   int    `json:"level"`   // 标题层级（1=h1,2=h2,3=h3）
	Content string `json:"content"` // 章节正文
	Size    int    `json:"size"`    // 字符数
}

type URLInfo struct {
	URL       string `json:"url"`
	Label     string `json:"label"`
	Type      string `json:"type"`
	IsPrimary bool   `json:"is_primary"`
}

type ConfigItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type KnowledgeRelation struct {
	SourceDocID  int    `json:"source_doc_id"`
	TargetDocID  int    `json:"target_doc_id"`
	RelationType string `json:"relation_type"`
	Description  string `json:"description"`
	Confidence   string `json:"confidence"`
}

type ThemeSummary struct {
	Theme         string `json:"theme"`
	Summary       string `json:"summary"`
	RelatedDocIDs []int  `json:"related_doc_ids"`
}

// S9: KnowledgeGap 知识缺口
type KnowledgeGap struct {
	Category       string `json:"category"`
	Description    string `json:"description"`
	SuggestedTitle string `json:"suggested_title"`
	Priority       string `json:"priority"` // high/medium/low
}

// BatchContext 批次间传递的富结构化上下文
type BatchContext struct {
	CategoryNames   []string         `json:"category_names"`
	KeyTerms        []string         `json:"key_terms"`         // 全局关键术语
	DocEntities     map[int][]string `json:"doc_entities"`      // 每篇文档的关键实体
	DocTopics       map[int]string   `json:"doc_topics"`        // 每篇文档的主题
	RelationDescs   []string         `json:"relation_descs"`    // 已发现的关系
	ProcessedDocIDs []int            `json:"processed_doc_ids"` // 已处理文档ID
}

const (
	TaskTypeFull        = "full"
	TaskTypeIncremental = "incremental"
	TaskTypeSpaceOnly   = "space_only"
	TaskTypeQuick       = "quick"
	TaskTypeSingleDoc   = "single_doc"
	TaskTypeMultiDoc    = "multi_doc" // P0-1: 多文档分析

	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusPaused    = "paused"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
)
