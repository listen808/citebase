package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type QualityPatrol struct {
	provider  provider.Provider
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
}

func NewQualityPatrol(p provider.Provider) *QualityPatrol {
	return &QualityPatrol{
		provider:  p,
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

type PatrolResult struct {
	TotalDocs   int            `json:"total_docs"`
	CheckedDocs int            `json:"checked_docs"`
	IssueCount  int            `json:"issue_count"`
	HealthScore float64        `json:"health_score"`
	Issues      []QualityIssue `json:"issues"`
	Suggestions []string       `json:"suggestions"`
}

type QualityIssue struct {
	DocumentID   int    `json:"document_id"`
	DocumentName string `json:"document_name"`
	IssueType    string `json:"issue_type"`
	Description  string `json:"description"`
	Severity     string `json:"severity"`
}

func (qp *QualityPatrol) RunPatrol(ctx context.Context, spaceID int) (*PatrolResult, error) {
	var docs []*workspacemodel.Document
	var err error

	if spaceID > 0 {
		// spaceID 由调用方/模型给定，先确认该空间对当前调用者可见，避免定向巡检私有空间。
		if err := ensureSpaceVisible(ctx, spaceID); err != nil {
			return nil, err
		}
		docs, err = qp.docRepo.FindBySpaceID(spaceID)
	} else {
		docs, err = qp.docRepo.FindAllDocuments()
	}
	if err != nil {
		return nil, err
	}

	// spaceID=0 时 FindAllDocuments 是全库口径，而巡检结果会把文档名写进问题项，
	// 因此必须把范围收敛到调用者可见空间；无 user_id 时退化为「仅公开空间」。
	visibleIDs, verr := qp.spaceRepo.FindAccessibleSpaceIDs(ctxUserID(ctx), false)
	if verr != nil {
		return nil, verr
	}
	visible := make(map[int]bool, len(visibleIDs))
	for _, id := range visibleIDs {
		visible[id] = true
	}
	accessibleDocs := make([]*workspacemodel.Document, 0, len(docs))
	for _, doc := range docs {
		if visible[doc.SpaceID] {
			accessibleDocs = append(accessibleDocs, doc)
		}
	}
	docs = accessibleDocs

	result := &PatrolResult{
		TotalDocs: len(docs),
	}

	now := time.Now().Unix()
	staleThreshold := now - 90*24*3600

	for _, doc := range docs {
		if doc.Type != workspacemodel.DocumentTypePage {
			continue
		}
		result.CheckedDocs++

		if doc.UpdateTime < int(staleThreshold) {
			result.Issues = append(result.Issues, QualityIssue{
				DocumentID:   doc.ID,
				DocumentName: doc.Name,
				IssueType:    "stale",
				Description:  "文档超过90天未更新",
				Severity:     "medium",
			})
		}

		if doc.Name == "" || len(doc.Name) < 2 {
			result.Issues = append(result.Issues, QualityIssue{
				DocumentID:   doc.ID,
				DocumentName: doc.Name,
				IssueType:    "name",
				Description:  "文档名称过短或为空",
				Severity:     "low",
			})
		}
	}

	result.IssueCount = len(result.Issues)

	if result.CheckedDocs > 0 {
		issueRate := float64(result.IssueCount) / float64(result.CheckedDocs)
		result.HealthScore = (1.0 - issueRate) * 100
	} else {
		result.HealthScore = 100
	}

	result.Suggestions = qp.generateSuggestions(result)

	return result, nil
}

func (qp *QualityPatrol) generateSuggestions(result *PatrolResult) []string {
	var suggestions []string

	staleCount := 0
	for _, issue := range result.Issues {
		if issue.IssueType == "stale" {
			staleCount++
		}
	}

	if staleCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("有 %d 篇文档超过90天未更新，建议安排刷新", staleCount))
	}

	if result.HealthScore < 70 {
		suggestions = append(suggestions, "知识库健康度较低，建议进行一次全量知识分析")
	}

	if result.IssueCount == 0 {
		suggestions = append(suggestions, "知识库状态良好，继续保持")
	}

	return suggestions
}
