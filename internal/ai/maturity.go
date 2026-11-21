package ai

type MaturityLevel string

const (
	MaturityStable       MaturityLevel = "stable"
	MaturityBeta         MaturityLevel = "beta"
	MaturityExperimental MaturityLevel = "experimental"
)

func (m MaturityLevel) String() string {
	return string(m)
}

func (m MaturityLevel) Gte(other MaturityLevel) bool {
	order := map[MaturityLevel]int{
		MaturityExperimental: 0,
		MaturityBeta:         1,
		MaturityStable:       2,
	}
	return order[m] >= order[other]
}

type FeatureMaturity struct {
	Name        string        `json:"name"`
	Category    string        `json:"category"`
	Maturity    MaturityLevel `json:"maturity"`
	Description string        `json:"description"`
}

var RAGFeatures = []FeatureMaturity{
	{Name: "hybrid_search", Category: "retrieval", Maturity: MaturityStable, Description: "向量 + 关键词混合检索"},
	{Name: "rerank", Category: "retrieval", Maturity: MaturityBeta, Description: "CrossEncoder 重排序"},
	{Name: "hyde", Category: "retrieval", Maturity: MaturityBeta, Description: "HyDE 假设文档嵌入扩展检索"},
	{Name: "graph_rag", Category: "retrieval", Maturity: MaturityBeta, Description: "知识图谱增强检索"},
	{Name: "multi_hop", Category: "retrieval", Maturity: MaturityExperimental, Description: "多跳推理检索"},
	{Name: "self_rag", Category: "retrieval", Maturity: MaturityExperimental, Description: "Self-RAG 自适应检索"},
	{Name: "corrective_rag", Category: "retrieval", Maturity: MaturityExperimental, Description: "Corrective RAG 检索修正"},
	{Name: "adaptive_retrieval", Category: "retrieval", Maturity: MaturityExperimental, Description: "自适应检索策略"},
	{Name: "hierarchical_index", Category: "retrieval", Maturity: MaturityExperimental, Description: "分层索引检索"},
	{Name: "raptor", Category: "retrieval", Maturity: MaturityExperimental, Description: "RAPTOR 树状摘要索引"},
	{Name: "mmr_rerank", Category: "retrieval", Maturity: MaturityBeta, Description: "MMR 多样性重排序"},
	{Name: "context_compression", Category: "compression", Maturity: MaturityBeta, Description: "上下文压缩"},
	{Name: "contextual_compression", Category: "compression", Maturity: MaturityExperimental, Description: "上下文感知压缩"},
	{Name: "long_context_handler", Category: "compression", Maturity: MaturityExperimental, Description: "长上下文处理器"},
	{Name: "hallucination_detection", Category: "safety", Maturity: MaturityBeta, Description: "幻觉检测"},
	{Name: "safety_guard", Category: "safety", Maturity: MaturityStable, Description: "安全守卫"},
	{Name: "query_cache", Category: "performance", Maturity: MaturityStable, Description: "查询缓存"},
	{Name: "semantic_cache", Category: "performance", Maturity: MaturityBeta, Description: "语义缓存"},
	{Name: "query_rewrite", Category: "optimization", Maturity: MaturityStable, Description: "查询重写"},
	{Name: "memory", Category: "optimization", Maturity: MaturityBeta, Description: "对话记忆管理"},
	{Name: "workflow_engine", Category: "pipeline", Maturity: MaturityBeta, Description: "工作流引擎"},
	{Name: "react_agent", Category: "agent", Maturity: MaturityBeta, Description: "ReAct Agent 工具调用"},
	{Name: "multi_agent", Category: "agent", Maturity: MaturityExperimental, Description: "多智能体协作"},
	{Name: "self_eval", Category: "evaluation", Maturity: MaturityExperimental, Description: "自评估"},
	{Name: "ragas_eval", Category: "evaluation", Maturity: MaturityBeta, Description: "RAGAS 评估框架"},
	{Name: "auto_evaluator", Category: "evaluation", Maturity: MaturityExperimental, Description: "自动评估器"},
	{Name: "deepeval", Category: "evaluation", Maturity: MaturityExperimental, Description: "DeepEval 评估"},
	{Name: "vision_rag", Category: "multimodal", Maturity: MaturityExperimental, Description: "多模态视觉 RAG"},
	{Name: "multimodal", Category: "multimodal", Maturity: MaturityExperimental, Description: "多模态支持"},
}

var ToolMaturity = map[string]MaturityLevel{
	"get_document":      MaturityStable,
	"list_documents":    MaturityStable,
	"list_spaces":       MaturityStable,
	"search_user":       MaturityStable,
	"calculate":         MaturityStable,
	"get_current_time":  MaturityStable,
	"json_formatter":    MaturityStable,
	"string_manipulate": MaturityStable,
	"random_number":     MaturityStable,
	"edit_document":     MaturityBeta,
	"send_email":        MaturityBeta,
	"fetch_web":         MaturityBeta,
	"generate_chart":    MaturityBeta,
	"query_stats":       MaturityBeta,
	"web_search":        MaturityBeta,
	"api_call":          MaturityBeta,
	"file_operation":    MaturityBeta,
	"execute_code":      MaturityExperimental,
	"execute_command":   MaturityExperimental,
}

var Text2SQLFeatures = []FeatureMaturity{
	{Name: "basic_generation", Category: "core", Maturity: MaturityStable, Description: "基础 SQL 生成"},
	{Name: "schema_linking", Category: "core", Maturity: MaturityStable, Description: "Schema 链接"},
	{Name: "multi_candidate", Category: "generation", Maturity: MaturityBeta, Description: "多候选生成"},
	{Name: "self_consistency", Category: "generation", Maturity: MaturityBeta, Description: "自一致性投票"},
	{Name: "multi_hop", Category: "generation", Maturity: MaturityBeta, Description: "多跳 SQL 推理"},
	{Name: "execution_feedback", Category: "generation", Maturity: MaturityStable, Description: "执行反馈循环"},
	{Name: "cot", Category: "generation", Maturity: MaturityStable, Description: "思维链生成"},
	{Name: "function_calling", Category: "generation", Maturity: MaturityBeta, Description: "Function Calling 生成器"},
	{Name: "ast_corrector", Category: "validation", Maturity: MaturityStable, Description: "AST 修正器"},
	{Name: "structural_validator", Category: "validation", Maturity: MaturityStable, Description: "结构校验器"},
	{Name: "sql_self_validator", Category: "validation", Maturity: MaturityBeta, Description: "SQL 自校验"},
	{Name: "llm_refiner", Category: "optimization", Maturity: MaturityBeta, Description: "LLM 优化器"},
	{Name: "schema_optimizer", Category: "optimization", Maturity: MaturityBeta, Description: "Schema 优化器"},
	{Name: "query_cache", Category: "performance", Maturity: MaturityStable, Description: "查询缓存"},
	{Name: "conversation_memory", Category: "memory", Maturity: MaturityBeta, Description: "对话记忆"},
	{Name: "row_level_security", Category: "security", Maturity: MaturityBeta, Description: "行级安全"},
	{Name: "data_masking", Category: "security", Maturity: MaturityBeta, Description: "数据脱敏"},
	{Name: "agent", Category: "agent", Maturity: MaturityExperimental, Description: "Text2SQL Agent"},
	{Name: "natural_language_feedback", Category: "explain", Maturity: MaturityBeta, Description: "自然语言结果解释"},
}

var AnalyzerFeatures = []FeatureMaturity{
	{Name: "entity_extraction", Category: "core", Maturity: MaturityStable, Description: "实体提取"},
	{Name: "classification", Category: "core", Maturity: MaturityStable, Description: "文档分类"},
	{Name: "keyword_extraction", Category: "core", Maturity: MaturityStable, Description: "关键词提取"},
	{Name: "summary", Category: "core", Maturity: MaturityStable, Description: "文档摘要"},
	{Name: "duplicate_detection", Category: "quality", Maturity: MaturityBeta, Description: "重复检测"},
	{Name: "contradiction_detection", Category: "quality", Maturity: MaturityBeta, Description: "矛盾检测"},
	{Name: "quality_assessment", Category: "quality", Maturity: MaturityBeta, Description: "质量评估"},
	{Name: "knowledge_graph", Category: "graph", Maturity: MaturityBeta, Description: "知识图谱构建"},
	{Name: "relation_extraction", Category: "graph", Maturity: MaturityBeta, Description: "关系提取"},
	{Name: "event_extraction", Category: "graph", Maturity: MaturityExperimental, Description: "事件提取"},
	{Name: "sentiment_analysis", Category: "analysis", Maturity: MaturityBeta, Description: "情感分析"},
	{Name: "batch_processing", Category: "performance", Maturity: MaturityStable, Description: "批量处理"},
	{Name: "incremental", Category: "performance", Maturity: MaturityStable, Description: "增量分析"},
}

func GetStableRAGFeatures() []FeatureMaturity {
	return FilterFeaturesByMaturity(RAGFeatures, MaturityStable)
}

func GetStableTools() map[string]MaturityLevel {
	result := make(map[string]MaturityLevel)
	for name, level := range ToolMaturity {
		if level == MaturityStable {
			result[name] = level
		}
	}
	return result
}

func FilterFeaturesByMaturity(features []FeatureMaturity, min MaturityLevel) []FeatureMaturity {
	var result []FeatureMaturity
	for _, f := range features {
		if f.Maturity.Gte(min) {
			result = append(result, f)
		}
	}
	return result
}
