package rag

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"
)

type PromptTemplate struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	System    string    `json:"system"`
	User      string    `json:"user"`
	Variables []string  `json:"variables"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PromptTemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*PromptTemplate
	storePath string
}

var promptManager *PromptTemplateManager

func InitPromptTemplateManager(storePath string) {
	promptManager = &PromptTemplateManager{
		templates: make(map[string]*PromptTemplate),
		storePath: storePath,
	}
	promptManager.loadDefaults()
	if storePath != "" {
		promptManager.loadFromFile()
	}
}

func (ptm *PromptTemplateManager) loadFromFile() {
	data, err := os.ReadFile(ptm.storePath)
	if err != nil {
		return
	}
	var saved map[string]*PromptTemplate
	if err := json.Unmarshal(data, &saved); err != nil {
		return
	}
	for k, t := range saved {
		ptm.templates[k] = t
	}
}

func GetPromptTemplateManager() *PromptTemplateManager {
	if promptManager == nil {
		promptManager = &PromptTemplateManager{
			templates: make(map[string]*PromptTemplate),
		}
		promptManager.loadDefaults()
	}
	return promptManager
}

func (ptm *PromptTemplateManager) loadDefaults() {
	now := time.Now()

	ptm.templates["rag_system"] = &PromptTemplate{
		ID:   "rag_system",
		Name: "RAG系统提示",
		System: `你是一个专业的知识库AI助手。请基于以下提供的文档内容回答问题。

## 回答原则
1. **忠实于文档**：只使用文档中明确提到的信息回答，不要编造或推断文档中不存在的内容
2. **引用来源**：在回答中使用 [来源N] 格式标注引用的文档来源
3. **结构化回答**：使用清晰的结构组织回答（要点、步骤、分类等）
4. **坦诚不足**：如果文档内容不足以回答问题，请如实说明，并建议用户可以如何获取更多信息
5. **关联分析**：如果问题涉及多个文档，综合分析各文档的信息，找出关联和差异

## 回答格式
- 先给出直接答案（1-2句话）
- 然后展开详细说明（如有必要）
- 最后列出引用来源

文档内容：
{{context}}`,
		User:      "{{question}}",
		Variables: []string{"context", "question"},
		Version:   2,
		UpdatedAt: now,
	}

	ptm.templates["rag_stream_system"] = &PromptTemplate{
		ID:   "rag_stream_system",
		Name: "RAG流式系统提示",
		System: `你是一个知识库AI助手。请基于以下提供的文档内容回答问题。
如果文档内容不足以回答问题，请如实说明你无法从已有知识中找到答案。
请用中文回答，保持简洁清晰。

文档内容：
{{context}}`,
		User:      "{{question}}",
		Variables: []string{"context", "question"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["query_rewrite"] = &PromptTemplate{
		ID:   "query_rewrite",
		Name: "查询改写",
		System: `你是一个查询优化专家。请将用户查询改写为更适合知识库检索的形式。

## 改写策略
1. **补充同义词**：将专业术语的同义词/近义词也加入查询
2. **消除歧义**：如果查询中有代词或模糊表述，用具体名词替换
3. **扩展缩写**：将缩写展开为完整表述
4. **分解复杂查询**：如果查询包含多个独立信息需求，拆分为关键检索词
5. **保留核心意图**：改写后的查询必须保持原始查询的核心意图

只输出改写后的查询，不要解释。多个改写结果用换行分隔。`,
		User:      "{{question}}",
		Variables: []string{"question"},
		Version:   2,
		UpdatedAt: now,
	}

	ptm.templates["question_decompose"] = &PromptTemplate{
		ID:   "question_decompose",
		Name: "问题分解",
		System: `如果以下问题可以分解为多个子问题，请分解。输出JSON数组，每个元素是一个子问题。如果问题简单不需要分解，输出单元素数组。
示例：["什么是JWT？", "JWT的刷新机制是什么？"]`,
		User:      "{{question}}",
		Variables: []string{"question"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["summary"] = &PromptTemplate{
		ID:        "summary",
		Name:      "文档摘要",
		System:    `你是一个知识库AI助手，擅长总结文档要点。`,
		User:      `请对以下文档内容生成一个简洁的中文摘要，不超过200字，突出核心要点：\n\n{{content}}`,
		Variables: []string{"content"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["write_rewrite"] = &PromptTemplate{
		ID:        "write_rewrite",
		Name:      "写作改写",
		System:    `你是一个专业的写作助手，请按要求处理文本。`,
		User:      `请改写以下文本，使其更加清晰、专业。{{instruction}}\n\n{{text}}`,
		Variables: []string{"text", "instruction"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["write_continue"] = &PromptTemplate{
		ID:        "write_continue",
		Name:      "续写",
		System:    `你是一个专业的写作助手，请按要求处理文本。`,
		User:      `请续写以下文本，保持风格一致，续写100-200字：\n\n{{text}}`,
		Variables: []string{"text"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["write_translate"] = &PromptTemplate{
		ID:        "write_translate",
		Name:      "翻译",
		System:    `你是一个专业的写作助手，请按要求处理文本。`,
		User:      `请将以下文本翻译为{{language}}：\n\n{{text}}`,
		Variables: []string{"text", "language"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["write_summarize"] = &PromptTemplate{
		ID:        "write_summarize",
		Name:      "总结",
		System:    `你是一个专业的写作助手，请按要求处理文本。`,
		User:      `请总结以下文本的核心要点，不超过150字：\n\n{{text}}`,
		Variables: []string{"text"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["write_template"] = &PromptTemplate{
		ID:     "write_template",
		Name:   "模板生成",
		System: `你是一个专业的写作助手，请按要求处理文本。`,
		User: `请根据以下描述生成一份{{template_type}}的Markdown文档模板。
要求：
- 包含清晰的标题层级（#、##、###）
- 在需要填充内容的地方使用 [待填写] 占位符
- 包含适当的示例内容
- 模板应实用且可直接使用

描述：{{text}}`,
		Variables: []string{"text", "template_type"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["write_diff_desc"] = &PromptTemplate{
		ID:        "write_diff_desc",
		Name:      "变更摘要",
		System:    `你是一个专业的写作助手，请按要求处理文本。`,
		User:      `请用中文描述以下文档变更的核心内容（不超过100字）：\n\n旧内容（前1000字）：\n{{old_content}}\n\n新内容（前1000字）：\n{{new_content}}`,
		Variables: []string{"old_content", "new_content"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["history_compress"] = &PromptTemplate{
		ID:        "history_compress",
		Name:      "对话历史压缩",
		System:    `你是一个对话摘要助手。`,
		User:      `请将以下对话历史压缩为一段简洁的摘要（不超过300字），保留关键信息、结论和上下文：\n\n{{history}}`,
		Variables: []string{"history"},
		Version:   1,
		UpdatedAt: now,
	}

	ptm.templates["safety_guard"] = &PromptTemplate{
		ID:   "safety_guard",
		Name: "安全护栏",
		System: `你是一个知识库AI助手。请遵守以下规则：
1. 只基于提供的文档内容回答问题，不编造信息
2. 不泄露其他空间或用户的信息
3. 不生成有害、歧视性或违法的内容
4. 如遇敏感信息请求，礼貌拒绝

文档内容：
{{context}}`,
		User:      "{{question}}",
		Variables: []string{"context", "question"},
		Version:   1,
		UpdatedAt: now,
	}
}

func (ptm *PromptTemplateManager) Get(id string) *PromptTemplate {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()
	return ptm.templates[id]
}

func (ptm *PromptTemplateManager) List() []*PromptTemplate {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	result := make([]*PromptTemplate, 0, len(ptm.templates))
	for _, t := range ptm.templates {
		result = append(result, t)
	}
	return result
}

func (ptm *PromptTemplateManager) Update(t *PromptTemplate) {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	if existing, ok := ptm.templates[t.ID]; ok {
		t.Version = existing.Version + 1
	} else {
		t.Version = 1
	}
	t.UpdatedAt = time.Now()
	ptm.templates[t.ID] = t

	// P3：同步持久化，避免异步 goroutine 竞争写同一文件 + 错误被吞
	if err := ptm.persistLocked(); err != nil {
		logx.Infof("[PromptTemplate] persist failed: %v", err)
	}
}

func (ptm *PromptTemplateManager) Render(id string, vars map[string]string) (system string, user string, ok bool) {
	ptm.mu.RLock()
	t, exists := ptm.templates[id]
	ptm.mu.RUnlock()

	if !exists {
		return "", "", false
	}

	system = t.System
	user = t.User
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		system = replaceAll(system, placeholder, v)
		user = replaceAll(user, placeholder, v)
	}
	return system, user, true
}

func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

func (ptm *PromptTemplateManager) persist() error {
	if ptm.storePath == "" {
		return nil
	}

	ptm.mu.RLock()
	data, err := json.MarshalIndent(ptm.templates, "", "  ")
	ptm.mu.RUnlock()

	if err != nil {
		return err
	}

	tmpPath := ptm.storePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, ptm.storePath)
}

// persistLocked 在已持写锁的情况下持久化（避免重入死锁）
func (ptm *PromptTemplateManager) persistLocked() error {
	if ptm.storePath == "" {
		return nil
	}

	data, err := json.MarshalIndent(ptm.templates, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := ptm.storePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, ptm.storePath)
}

func (ptm *PromptTemplateManager) LoadFromHTTP(url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url) //nolint:noctx // one-shot admin fetch, no request ctx available
	if err != nil {
		return fmt.Errorf("fetch templates: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read templates: %w", err)
	}

	var templates map[string]*PromptTemplate
	if err := json.Unmarshal(body, &templates); err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	ptm.mu.Lock()
	defer ptm.mu.Unlock()
	for k, t := range templates {
		ptm.templates[k] = t
	}

	return nil
}
