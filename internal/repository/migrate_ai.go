package repository

import (
	modelai "github.com/listen808/citebase/internal/model/ai"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// AI / 知识图谱 / Text2SQL / RAG 评估相关表。
func init() {
	// P1-6: 用量日聚合（成本/配额底账）
	RegisterModels(&modelai.AIUsageDaily{})
	RegisterModels(
		// 知识分析与图谱
		&knowledgemodel.AITask{},
		&knowledgemodel.AIDocMapping{},
		&knowledgemodel.AIExtractedEntities{},
		&knowledgemodel.AIDocCluster{},
		&knowledgemodel.VectorSyncTask{},    // E2: 向量同步持久化
		&knowledgemodel.VectorSyncState{},   // P0: 文档级向量索引指纹（增量缓存）
		&knowledgemodel.KnowledgeEntity{},   // M1: 知识图谱实体
		&knowledgemodel.KnowledgeRelation{}, // M1: 知识图谱关系
		&knowledgemodel.AIFeedback{},        // L3: AI反馈
		&knowledgemodel.AiRagasEval{},       // L-10: RAGAS评估持久化
		&knowledgemodel.AIAnalysisResult{},  // AI分析结果

		// LLM Wiki 编译产物（P0-1）：持久、互链、可版本化的知识页面
		&knowledgemodel.WikiPage{},
		&knowledgemodel.WikiLink{},

		// LLM Wiki 治理（P1）：人工修正锚定 / 异步审核 / 体检问题
		&knowledgemodel.WikiPin{},
		&knowledgemodel.WikiReview{},
		&knowledgemodel.WikiLintIssue{},
		&knowledgemodel.WikiPageHistory{}, // 版本历史（回滚的基线来源）

		// 对话与审计
		&modelai.AIConversation{}, // S3: 对话历史持久化
		&modelai.AIAuditLog{},     // L6: AI审计日志

		// Text-to-SQL 核心
		&modelai.BusinessRule{},        // 业务规则
		&modelai.TableCache{},          // 表缓存
		&modelai.TableRelation{},       // 表关系
		&modelai.DataSource{},          // v2 数据源
		&modelai.SQLHistory{},          // v2 SQL历史
		&modelai.SQLSecurityPolicy{},   // v2 SQL安全策略
		&modelai.Text2SQLFeedback{},    // v2 执行反馈记录
		&modelai.PromptTemplate{},      // v2 提示词模板
		&modelai.SQLExample{},          // Few-shot 示例
		&modelai.BusinessGlossary{},    // 业务术语映射
		&modelai.DataSensitivityRule{}, // 字段级脱敏规则

		// AI 记忆系统
		&modelai.AIMemory{},        // 用户记忆（偏好/知识/实体）
		&modelai.AIMemorySummary{}, // 会话记忆摘要

		// Text2SQL 多轮对话持久化
		&modelai.SQLConversationTurn{},    // 对话轮次
		&modelai.SQLConversationSummary{}, // 对话摘要
		&modelai.SQLTurnEmbedding{},       // 向量记录
		&modelai.SQLConversationSession{}, // 会话状态

		// Text2SQL 语义指标与链路追踪
		&modelai.SemanticMetric{},    // 语义指标
		&modelai.SemanticDimension{}, // 语义维度
		&modelai.ColumnStats{},       // 列级统计
		&modelai.SQLTrace{},          // SQL生成链路追踪

		// Text2SQL Prompt 版本与 A/B 实验
		&modelai.PromptVersion{}, // Prompt 版本
		&modelai.ABExperiment{},  // A/B 实验
	)
}
