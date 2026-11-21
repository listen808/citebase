package wiki

import (
	"context"
	"fmt"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// 元页面 slug。index 保持 BuildIndex 的运行时目录（确定性、可缓存），
// overview 与 log 落成真实页面行（type=overview / type=log）。
const (
	overviewSlug   = "overview"
	compileLogSlug = "wiki-log"
	maxLogRunes    = 20000 // 超出后从头部截断，保留最近的编译记录
)

// RefreshOverview 用已发布页面的目录生成/刷新空间总览页（overview 型）。
// overview 是纯派生物：任何时刻都可以从目录重建，因此不走 draft→审核流，
// 直接 upsert 成 published。页面数为 0 时不生成空页。
func (c *Compiler) RefreshOverview(ctx context.Context, spaceID int) error {
	if c.provider == nil {
		return fmt.Errorf("provider not available")
	}
	indexText, entries, err := c.BuildIndex(spaceID)
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}

	system := "你是知识库总览撰写助手。根据知识目录写一份 300 字以内的空间总览：先一句话概括这个知识库覆盖的领域，再列出最重要的 3-5 个主题及其关系。只使用目录中存在的信息，绝不编造。" + purposePromptBlock(spaceID)
	resp, err := c.provider.Chat(ctx, system,
		[]provider.Message{provider.NewTextMessage("user", indexText)},
		provider.WithMaxTokens(512), provider.WithTemperature(0.3))
	if err != nil {
		return fmt.Errorf("generate overview: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return nil
	}
	summary := firstSentence(content, 120)
	if _, err := c.pageRepo.UpsertSystemPage(spaceID, overviewSlug,
		string(knowledgemodel.WikiPageTypeOverview), "空间总览", summary, content,
		ContentHash(content)); err != nil {
		return fmt.Errorf("upsert overview page: %w", err)
	}
	InvalidateIndex(spaceID)
	return nil
}

// AppendCompileLog 往 wiki-log 页追加一次编译记录（确定性内容，不走 LLM）。
// 记录倒序排列（最新的在最上面），超长时从头截断。
func (c *Compiler) AppendCompileLog(spaceID int, lines []string) {
	if len(lines) == 0 {
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	section := fmt.Sprintf("## %s\n%s\n", now, strings.Join(lines, "\n"))

	content := section
	if existing, err := c.pageRepo.FindBySlug(spaceID, compileLogSlug); err == nil && existing.Content != "" {
		content = section + "\n" + existing.Content
	}
	content = truncate(content, maxLogRunes)
	summary := "编译操作日志（自动维护）"

	if _, err := c.pageRepo.UpsertSystemPage(spaceID, compileLogSlug,
		string(knowledgemodel.WikiPageTypeLog), "编译日志", summary, content,
		ContentHash(content)); err != nil {
		logx.Infof("[WikiCompiler] Warning: failed to append compile log for space %d: %v", spaceID, err)
		return
	}
	InvalidateIndex(spaceID)
}
