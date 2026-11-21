package knowledge

import "time"

// WikiPageType 知识页面类型，对应 Karpathy LLM Wiki 的 wiki/ 子目录划分。
type WikiPageType string

const (
	WikiPageTypeConcept  WikiPageType = "concept"  // 概念页：跨源综合
	WikiPageTypeEntity   WikiPageType = "entity"   // 实体页：人/组织/产品/工具
	WikiPageTypeSource   WikiPageType = "source"   // 源摘要页：单篇源文档的结构化摘要
	WikiPageTypeMethod   WikiPageType = "method"   // 方法论页
	WikiPageTypeCompare  WikiPageType = "compare"  // 对比分析页
	WikiPageTypeAnalysis WikiPageType = "analysis" // 深度分析页（优质问答/研究沉淀）
	WikiPageTypeOverview WikiPageType = "overview" // 空间总览页
	WikiPageTypeIndex    WikiPageType = "index"    // 目录页
	WikiPageTypeLog      WikiPageType = "log"      // 编译操作日志页（自动维护，不进审核流）
)

// WikiPageStatus 页面生命周期。
// planned 是编译一致性的关键：Plan 阶段串行写入占位，让后续并发任务看得见
// "这份知识已经决定落到哪一页"，从而避免语义等价的知识被重复创建成两个页面
// （AWS 实践：并发编译曾让 38% 的页面语义重复）。
type WikiPageStatus string

const (
	WikiPageStatusPlanned   WikiPageStatus = "planned"   // 已规划，待生成
	WikiPageStatusDraft     WikiPageStatus = "draft"     // 草稿，待人工审核
	WikiPageStatusPublished WikiPageStatus = "published" // 已发布，可被查询读取
	WikiPageStatusOrphan    WikiPageStatus = "orphan"    // 源已删除/锚点失效，待确认
)

const (
	// WikiProvenanceLLM 页面内容由模型编译生成。
	WikiProvenanceLLM = "llm"
	// WikiProvenanceHuman 页面内容含人工确认/修正的知识，与 LLM 产物分开管理生命周期。
	WikiProvenanceHuman = "human"
)

// WikiPage 编译产物：一个持久、互链、可版本化的知识页面。
type WikiPage struct {
	ID           int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID      int        `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Slug         string     `gorm:"column:slug;size:190;not null" json:"slug"`
	Type         string     `gorm:"column:type;size:32;not null;default:'concept'" json:"type"`
	Title        string     `gorm:"column:title;size:255;not null" json:"title"`
	Summary      string     `gorm:"column:summary;type:text" json:"summary"`
	Content      string     `gorm:"column:content;type:longtext" json:"content"`
	Sources      IntArray   `gorm:"column:sources;type:text" json:"sources"`
	Status       string     `gorm:"column:status;size:20;not null;default:'planned'" json:"status"`
	Version      int        `gorm:"column:version;not null;default:0" json:"version"`
	BaseVersion  int        `gorm:"column:base_version;not null;default:0" json:"base_version"`
	SourceCount  int        `gorm:"column:source_count;not null;default:0" json:"source_count"`
	Provenance   string     `gorm:"column:provenance;size:16;not null;default:'llm'" json:"provenance"`
	Confidence   float64    `gorm:"column:confidence;type:decimal(4,2);not null;default:0.50" json:"confidence"`
	SupersededBy string     `gorm:"column:superseded_by;size:190;not null;default:''" json:"superseded_by"` // 被哪个新页面取代（slug），空=当前有效
	ContentHash  string     `gorm:"column:content_hash;size:64;not null;default:''" json:"content_hash"`
	CompiledAt   *time.Time `gorm:"column:compiled_at" json:"compiled_at"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (WikiPage) TableName() string { return "aw_wiki_page" }

// WikiLink 页面间双向交叉引用。链接结构本身即知识图谱。
type WikiLink struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID   int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	FromSlug  string    `gorm:"column:from_slug;size:190;not null" json:"from_slug"`
	ToSlug    string    `gorm:"column:to_slug;size:190;not null;index:idx_to_slug" json:"to_slug"`
	LinkType  string    `gorm:"column:link_type;size:32;not null;default:'ref'" json:"link_type"`
	Weight    float64   `gorm:"column:weight;type:decimal(6,3);not null;default:1" json:"weight"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (WikiLink) TableName() string { return "aw_wiki_link" }
