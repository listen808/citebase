package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/delivery"
	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// PinCheck 一条 pin 的复核结果。
type PinCheck struct {
	PinID  int    `json:"pin_id"`
	Claim  string `json:"claim"`
	Status string `json:"status"` // satisfied / conflict / orphan / unknown
	Note   string `json:"note"`
}

// CheckPins 重编译后逐条复核人工 pin。
//
// 三条规则（AWS 实践，12 个 pin 实验静默丢失 0）：
//  1. 新页面仍满足 claim → 放行，哪怕措辞已经完全变化
//  2. 新源与 claim 冲突 → 不自动覆盖，进人工裁决
//  3. 原锚定章节已不存在 → 标记 orphan，提示审核者重新确认位置
//
// 无法确定时统一进人工审核，而不是自动选一个答案 —— 对知识系统来说，
// 这种失败方式更可控：暴露冲突，而非静默丢失。
func (c *Compiler) CheckPins(ctx context.Context, spaceID int, slug string) ([]PinCheck, error) {
	if c.pinRepo == nil {
		return nil, nil
	}
	pins, err := c.pinRepo.ListActiveByPage(spaceID, slug)
	if err != nil || len(pins) == 0 {
		return nil, err
	}

	page, err := c.pageRepo.FindBySlug(spaceID, slug)
	if err != nil {
		return nil, err
	}

	results := make([]PinCheck, 0, len(pins))
	toVerify := make([]*knowledgemodel.WikiPin, 0, len(pins))

	// 锚点检查不需要模型：章节还在不在是确定性事实
	for _, p := range pins {
		if p.Anchor != "" && !strings.Contains(page.Content, strings.TrimPrefix(p.Anchor, "#")) {
			results = append(results, PinCheck{PinID: p.ID, Claim: p.Claim,
				Status: string(knowledgemodel.WikiPinStatusOrphan),
				Note:   fmt.Sprintf("锚定章节 %q 在重新编译后不存在", p.Anchor)})
			_ = c.pinRepo.UpdateStatus(p.ID, string(knowledgemodel.WikiPinStatusOrphan))
			continue
		}
		toVerify = append(toVerify, p)
	}

	if len(toVerify) == 0 {
		return results, nil
	}

	if c.provider == nil {
		// 没有模型可用于语义复核，一律保守地标记为待人工确认
		for _, p := range toVerify {
			results = append(results, PinCheck{PinID: p.ID, Claim: p.Claim,
				Status: "unknown", Note: "无可用模型做语义复核，需人工确认"})
		}
		return results, nil
	}

	verdicts := c.llmVerifyPins(ctx, page.Content, toVerify)
	for i, p := range toVerify {
		status := "unknown"
		note := "模型未能给出确定结论，需人工确认"
		if i < len(verdicts) {
			switch verdicts[i] {
			case "satisfied":
				status, note = "satisfied", "新页面仍满足该事实"
			case "conflict":
				status, note = string(knowledgemodel.WikiPinStatusConflict), "新内容与人工确认的事实冲突，需人工裁决"
			}
		}
		if status == string(knowledgemodel.WikiPinStatusConflict) {
			_ = c.pinRepo.UpdateStatus(p.ID, status)
			c.enqueueReview(ctx, spaceID, slug, "pin_conflict", note, page.BaseVersion, p.CreatedBy)
		} else if status != "satisfied" {
			_ = c.pinRepo.UpdateStatus(p.ID, string(knowledgemodel.WikiPinStatusOrphan))
		}
		results = append(results, PinCheck{PinID: p.ID, Claim: p.Claim, Status: status, Note: note})
	}
	return results, nil
}

// llmVerifyPins 一次调用复核多条 claim，返回与输入等长的结果序列。
func (c *Compiler) llmVerifyPins(ctx context.Context, content string, pins []*knowledgemodel.WikiPin) []string {
	var b strings.Builder
	for i, p := range pins {
		fmt.Fprintf(&b, "%d. %s\n", i+1, p.Claim)
	}
	prompt := fmt.Sprintf("页面正文：\n%s\n\n人工确认过的事实清单：\n%s\n\n"+
		"逐条判断页面正文是否仍然满足该事实。输出 JSON 数组，每项为 \"satisfied\"、"+
		"\"conflict\" 或 \"unknown\"，顺序与清单一致，不要解释。", truncate(content, 6000), b.String())

	resp, err := c.provider.Chat(ctx,
		"你是知识一致性检查助手。只判断页面正文是否仍然满足给定事实陈述，不考虑事实本身的对错。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(256), provider.WithTemperature(0))
	if err != nil {
		logx.Infof("[WikiCompiler] Pin verification failed: %v", err)
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(extractJSONArray(resp.Content)), &out); err != nil {
		return nil
	}
	return out
}

// confidenceReviewThreshold 低于此置信度的页面自动进入审核队列。
const confidenceReviewThreshold = 0.40

// CheckConfidence 重编译后检查页面置信度：低于阈值的页面自动路由到人工审核。
// 低置信度意味着 LLM 对提取结果的把握不足，未经人工确认不应直接发布。
func (c *Compiler) CheckConfidence(ctx context.Context, spaceID int, slug string) {
	if c.reviewRepo == nil {
		return
	}
	page, err := c.pageRepo.FindBySlug(spaceID, slug)
	if err != nil {
		return
	}
	// 人工确认过的页面不受置信度约束
	if page.Provenance == knowledgemodel.WikiProvenanceHuman {
		return
	}
	if page.Confidence >= confidenceReviewThreshold {
		return
	}
	reason := fmt.Sprintf("页面置信度 %.2f 低于阈值 %.2f，需人工确认内容准确性", page.Confidence, confidenceReviewThreshold)
	c.enqueueReview(ctx, spaceID, slug, "low_confidence", reason, page.BaseVersion, 0)
}

// enqueueReview 把需人工判断的项放入异步队列，不阻塞编译，并通知到人。
// 不通知的话这些待裁决项会静默堆在队列里，没人知道知识库里挂着冲突。
func (c *Compiler) enqueueReview(ctx context.Context, spaceID int, slug, kind, reason string, baseVersion int, notifyUserID int) {
	if c.reviewRepo == nil {
		return
	}
	exists, err := c.reviewRepo.HasPending(spaceID, slug)
	if err != nil || exists {
		return
	}
	item := &knowledgemodel.WikiReview{
		SpaceID:     spaceID,
		PageSlug:    slug,
		Kind:        kind,
		Action:      knowledgemodel.WikiReviewActionCreatePage,
		Reason:      reason,
		BaseVersion: baseVersion,
		Status:      knowledgemodel.WikiReviewStatusPending,
	}
	if err := c.reviewRepo.Create(item); err != nil {
		logx.Infof("[WikiCompiler] Enqueue review failed slug=%s: %v", slug, err)
		return
	}
	notify(ctx, delivery.EventAIInsight, "知识页面待裁决："+slug, reason, notifyUserID, item.ID)
}

// SupersedePage 标记旧页面被新页面取代（LLM Wiki 取代链）。
//
// 规则：
//  1. 旧页面保留内容（可回溯），但 superseded_by 指向新页面 slug
//  2. 检索时跳过已被取代的页面（ListActivePages 过滤 superseded_by != ”）
//  3. 已人工确认（provenance=human）的页面不允许被自动取代
//  4. 已取代的页面不再被重复取代（只保留第一级取代关系）
func (c *Compiler) SupersedePage(spaceID int, oldSlug, newSlug string) error {
	oldPage, err := c.pageRepo.FindBySlug(spaceID, oldSlug)
	if err != nil {
		return err
	}
	// 人工页面不被自动取代
	if oldPage.Provenance == knowledgemodel.WikiProvenanceHuman {
		return nil
	}
	// 已经取代过的不重复处理
	if oldPage.SupersededBy != "" {
		return nil
	}
	if err := c.pageRepo.MarkSuperseded(spaceID, oldSlug, newSlug); err != nil {
		return err
	}
	// 写入取代链链接（link_type = "supersedes"）
	if c.linkRepo != nil {
		_ = c.linkRepo.Upsert(spaceID, newSlug, oldSlug, "supersedes", 1.0)
	}
	return nil
}
