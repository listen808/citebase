package ai

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/log"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

func RecordAIAudit(c *gin.Context, userID int, action, targetID, question, answer, modelName, providerName, result, reason string, tokens int, sources int, durationMs int64) {
	auditLogRepo := airepo.NewAIAuditLogRepository()
	auditLog := &aimodel.AIAuditLog{
		UserID:      userID,
		Action:      action,
		TargetID:    targetID,
		Question:    question,
		Answer:      answer,
		TokenUsage:  tokens,
		SourceCount: sources,
		Model:       modelName,
		Provider:    providerName,
		Result:      result,
		Reason:      reason,
		DurationMs:  durationMs,
	}
	if c != nil {
		auditLog.IPAddress = c.ClientIP()
		auditLog.UserAgent = c.GetHeader("User-Agent")
	}
	if err := auditLogRepo.Create(auditLog); err != nil {
		log.Errorf("[AuditLog] Failed to record audit log: %v", err)
	}

	// P1-6：同步累加用量日聚合。挂在 RecordAIAudit 这一个收口上，
	// 所有 AI 行为（ask/summary/text2sql/...）自动都有成本底账，
	// 不用每个 handler 各写一遍。
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("[UsageDaily] panic recovered: %v", r)
			}
		}()
		usageRepo := airepo.NewAIUsageDailyRepository()
		if err := usageRepo.Increment(userID, providerName, modelName, action, tokens, durationMs, result != "success"); err != nil {
			log.Errorf("[UsageDaily] Failed to increment usage: %v", err)
		}
	}()
}

func RecordAIAuditSimple(c *gin.Context, userID int, action, targetID, result, reason string) {
	RecordAIAudit(c, userID, action, targetID, "", "", "", "", result, reason, 0, 0, 0)
}
