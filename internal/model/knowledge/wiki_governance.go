package knowledge

import "time"

// WikiPinStatus pin 的生命周期。
type WikiPinStatus string

const (
	WikiPinStatusActive   WikiPinStatus = "active"   // 生效，重编译后需复核
	WikiPinStatusOrphan   WikiPinStatus = "orphan"   // 锚定章节已不存在，需重新定位
	WikiPinStatusConflict WikiPinStatus = "conflict" // 与新源冲突，等待人工裁决
	WikiPinStatusResolved WikiPinStatus = "resolved" // 已处理
)

// WikiPin 人工确认过的事实锚点。
//
// 为什么要 pin 而不是保存文本 diff：LLM 重写页面时会改措辞、移段落、重组章节，
// 基于行号或上下文的 patch 很容易失去锚点。真正要保存的不是"哪几个字符变了"，
// 而是"专家希望这个事实保持为什么"。
type WikiPin struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID    int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	PageSlug   string    `gorm:"column:page_slug;size:190;not null;index:idx_page_slug" json:"page_slug"`
	Kind       string    `gorm:"column:kind;size:32;not null;default:'correction'" json:"kind"`
	Claim      string    `gorm:"column:claim;type:text;not null" json:"claim"`
	Anchor     string    `gorm:"column:anchor;size:255;not null;default:''" json:"anchor"`
	Provenance string    `gorm:"column:provenance;size:16;not null;default:'human'" json:"provenance"`
	Status     string    `gorm:"column:status;size:20;not null;default:'active'" json:"status"`
	CreatedBy  int       `gorm:"column:created_by;not null;default:0" json:"created_by"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (WikiPin) TableName() string { return "aw_wiki_pin" }

// WikiReview 异步人工审核项。
//
// 审核门放在 Generate 之后而非 Plan 之后：看"准备新建 3 页、更新 2 页"大多数都显得
// 合理，真正的错误（事实错、口径偏、遗漏）只有页面生成完之后才能发现。
type WikiReview struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID     int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	PageSlug    string    `gorm:"column:page_slug;size:190;not null;index:idx_page_slug" json:"page_slug"`
	Kind        string    `gorm:"column:kind;size:32;not null;default:'draft'" json:"kind"`
	Action      string    `gorm:"column:action;size:32;not null;default:''" json:"action"`
	Reason      string    `gorm:"column:reason;type:text" json:"reason"`
	BaseVersion int       `gorm:"column:base_version;not null;default:0" json:"base_version"`
	Status      string    `gorm:"column:status;size:20;not null;default:'pending'" json:"status"`
	ResolvedBy  int       `gorm:"column:resolved_by;not null;default:0" json:"resolved_by"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (WikiReview) TableName() string { return "aw_wiki_review" }

const (
	WikiReviewStatusPending  = "pending"
	WikiReviewStatusAccepted = "accepted"
	WikiReviewStatusRejected = "rejected"
)

// 审核项的预定义动作类型。限定动作集合是为了避免模型臆造出无法执行的动作。
const (
	WikiReviewActionCreatePage   = "create_page"
	WikiReviewActionDeepResearch = "deep_research"
	WikiReviewActionSkip         = "skip"
)

// WikiLintIssue Lint 体检发现的问题。
type WikiLintIssue struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID   int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	PageSlug  string    `gorm:"column:page_slug;size:190;not null;default:''" json:"page_slug"`
	Category  string    `gorm:"column:category;size:32;not null" json:"category"`
	Severity  string    `gorm:"column:severity;size:16;not null;default:'warning'" json:"severity"`
	Detail    string    `gorm:"column:detail;type:text" json:"detail"`
	Status    string    `gorm:"column:status;size:20;not null;default:'open'" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (WikiLintIssue) TableName() string { return "aw_wiki_lint_issue" }

// Lint 问题类别：孤立页 / 稀疏社区 / 死链 / 过时 / 矛盾 / 缺口。
const (
	LintCategoryIsolated   = "isolated"
	LintCategorySparse     = "sparse"
	LintCategoryDeadLink   = "dead_link"
	LintCategoryStale      = "stale"
	LintCategoryContradict = "contradiction"
	LintCategoryGap        = "gap"
)
