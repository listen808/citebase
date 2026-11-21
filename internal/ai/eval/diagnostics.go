package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// FailurePatternID represents one of the 12 canonical RAG failure patterns.
type FailurePatternID string

const (
	P01_UnderspecifiedQuery   FailurePatternID = "P01_underspecified_query"
	P02_QueryAmbiguity        FailurePatternID = "P02_query_ambiguity"
	P03_Hallucination         FailurePatternID = "P03_hallucination"
	P04_IncompleteRetrieval   FailurePatternID = "P04_incomplete_retrieval"
	P05_IrrelevantRetrieval   FailurePatternID = "P05_irrelevant_retrieval"
	P06_ContextWindowOverflow FailurePatternID = "P06_context_window_overflow"
	P07_ConflictingEvidence   FailurePatternID = "P07_conflicting_evidence"
	P08_FactualDrift          FailurePatternID = "P08_factual_drift"
	P09_PoorCitation          FailurePatternID = "P09_poor_citation"
	P10_BrokenChainOfThought  FailurePatternID = "P10_broken_chain_of_thought"
	P11_SafetyBreach          FailurePatternID = "P11_safety_breach"
	P12_TimeoutOrRateLimit    FailurePatternID = "P12_timeout_or_rate_limit"
)

// FailurePattern describes a single canonical failure mode with its symptoms,
// root cause, and recommended remediation actions.
type FailurePattern struct {
	ID            FailurePatternID `json:"id"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Symptoms      []string         `json:"symptoms"`
	RootCause     string           `json:"root_cause"`
	Remediation   []string         `json:"remediation"`
	Severity      float64          `json:"severity"` // 0.0 – 1.0
	DetectionHint string           `json:"detection_hint"`
}

// DiagnosticReport is the output of running diagnostics on a single QA pair.
type DiagnosticReport struct {
	PatternID   FailurePatternID `json:"pattern_id"`
	Matched     bool             `json:"matched"`
	Confidence  float64          `json:"confidence"`  // 0.0 – 1.0
	Severity    float64          `json:"severity"`    // from pattern
	Evidence    string           `json:"evidence"`    // what triggered the match
	Remediation []string         `json:"remediation"` // actionable steps
}

// DiagnosticSummary aggregates multiple DiagnosticReports for a single QA pair.
type DiagnosticSummary struct {
	Question        string             `json:"question"`
	TotalPatterns   int                `json:"total_patterns"`
	MatchedPatterns int                `json:"matched_patterns"`
	Reports         []DiagnosticReport `json:"reports"`
	OverallHealth   float64            `json:"overall_health"` // 0.0 (terrible) – 1.0 (perfect)
	TopFailure      *DiagnosticReport  `json:"top_failure,omitempty"`
}

// FailurePatternLibrary is the 12-pattern diagnostic library.
type FailurePatternLibrary struct {
	patterns map[FailurePatternID]*FailurePattern
}

// NewFailurePatternLibrary creates the library with all 12 canonical patterns.
func NewFailurePatternLibrary() *FailurePatternLibrary {
	lib := &FailurePatternLibrary{
		patterns: make(map[FailurePatternID]*FailurePattern),
	}
	lib.registerDefaults()
	return lib
}

func (lib *FailurePatternLibrary) registerDefaults() {
	lib.register(&FailurePattern{
		ID:          P01_UnderspecifiedQuery,
		Name:        "查询未充分指定",
		Description: "用户的查询缺少关键上下文或限定词，导致检索到过于宽泛的结果",
		Symptoms: []string{
			"检索结果覆盖面过广",
			"答案包含大量不相关信息",
			"检索召回数量远超有效数量",
		},
		RootCause:     "用户输入过于简短或模糊，缺少必要的限定条件",
		Remediation:   []string{"自动扩展查询（Query Expansion）", "引导用户补充查询细节", "使用 HyDE 生成假设性答案进行语义扩展"},
		Severity:      0.5,
		DetectionHint: "query_length < 10 && retrieved_count > 20",
	})

	lib.register(&FailurePattern{
		ID:          P02_QueryAmbiguity,
		Name:        "查询歧义",
		Description: "查询可以被多种方式理解，导致检索到不一致的上下文",
		Symptoms: []string{
			"检索到的文档片段互相矛盾",
			"LLM 给出多个可能的解释",
			"用户需要进一步澄清",
		},
		RootCause:     "查询中包含多义词、代词指代不明、或存在多种理解方式",
		Remediation:   []string{"查询消歧（Query Disambiguation）", "返回多个候选理解让用户选择", "结合对话历史确定指代"},
		Severity:      0.6,
		DetectionHint: "semantic_variance(retrieved) > threshold",
	})

	lib.register(&FailurePattern{
		ID:          P03_Hallucination,
		Name:        "幻觉",
		Description: "生成的回答包含源文档中不存在的信息",
		Symptoms: []string{
			"回答中出现文档未提及的事实",
			"引用了不存在的来源",
			"数字、日期、名称与源文档不符",
		},
		RootCause:     "LLM 的参数化知识与检索上下文冲突，或上下文不足时 LLM 自行补全",
		Remediation:   []string{"强制引用验证（Verifiable RAG）", "降低 temperature", "追加幻觉重试提示", "使用 Self-RAG 反思标记"},
		Severity:      0.9,
		DetectionHint: "hallucination_detector.has_hallucination == true",
	})

	lib.register(&FailurePattern{
		ID:          P04_IncompleteRetrieval,
		Name:        "检索不完整",
		Description: "检索到的文档片段不足以回答查询的全部方面",
		Symptoms: []string{
			"答案部分正确但遗漏关键信息",
			"LLM 回答'根据已有信息无法完全回答'",
			"上下文召回率低",
		},
		RootCause:     "检索 Top-K 太小、向量相似度阈值过高、或文档覆盖不全",
		Remediation:   []string{"增加 Top-K", "降低相似度阈值", "使用 Multi-Hop 多跳推理", "使用 GraphRAG 补充关系路径"},
		Severity:      0.7,
		DetectionHint: "context_recall < 0.5 || answer mentions '不完全'/'无法确认'",
	})

	lib.register(&FailurePattern{
		ID:          P05_IrrelevantRetrieval,
		Name:        "检索不相关",
		Description: "检索到的文档片段与查询主题不相关",
		Symptoms: []string{
			"检索结果包含无关文档",
			"答案与问题不匹配",
			"上下文精确度低",
		},
		RootCause:     "查询与文档语义空间存在偏差，或 embedding 模型对特定领域适配不佳",
		Remediation:   []string{"启用 Reranker 重排序", "使用 HyDE 语义扩展", "调整向量检索权重", "补充领域微调 embedding"},
		Severity:      0.6,
		DetectionHint: "context_precision < 0.4",
	})

	lib.register(&FailurePattern{
		ID:          P06_ContextWindowOverflow,
		Name:        "上下文窗口溢出",
		Description: "检索到的上下文总长度超出 LLM 的上下文窗口限制",
		Symptoms: []string{
			"答案截断或不完整",
			"LLM 忽略了后半部分上下文",
			"Token 使用量接近上限",
		},
		RootCause:     "检索结果过多或单篇文档过长，未做有效的上下文压缩",
		Remediation:   []string{"启用 SmartCrusher 对话压缩", "启用 ContextCompression 上下文压缩", "减少 Top-K", "使用 Map-Reduce 分段处理"},
		Severity:      0.5,
		DetectionHint: "total_tokens > context_window * 0.9",
	})

	lib.register(&FailurePattern{
		ID:          P07_ConflictingEvidence,
		Name:        "证据冲突",
		Description: "检索到的多个文档片段包含相互矛盾的信息",
		Symptoms: []string{
			"答案中出现'另一方面'/'然而'",
			"LLM 无法确定哪个版本正确",
			"不同来源的数据不一致",
		},
		RootCause:     "知识库中存在过时或错误的信息、或同一主题有不同版本的文档",
		Remediation:   []string{"使用时间戳优先级排序", "标记矛盾来源让用户决策", "启用 ContradictionDetector 检测矛盾", "使用 GraphRAG 验证关系一致性"},
		Severity:      0.7,
		DetectionHint: "contradiction_detector.conflicts > 0",
	})

	lib.register(&FailurePattern{
		ID:          P08_FactualDrift,
		Name:        "事实偏移",
		Description: "回答中的事实与源文档一致但被错误解读或过度推断",
		Symptoms: []string{
			"回答看似合理但细节不准确",
			"数字或百分比有微小偏差",
			"因果关系被颠倒",
		},
		RootCause:     "LLM 在推理过程中引入了参数化知识的偏移",
		Remediation:   []string{"启用 Self-RAG 自我反思", "强制逐条引用验证", "使用 CoT 逐步推理提示", "降低 temperature"},
		Severity:      0.8,
		DetectionHint: "hallucination_detector.confidence > 0.6 && claim_count > 3",
	})

	lib.register(&FailurePattern{
		ID:          P09_PoorCitation,
		Name:        "引用质量差",
		Description: "回答未能正确引用来源，或引用与内容不匹配",
		Symptoms: []string{
			"引用的来源编号在上下文中找不到对应内容",
			"缺少引用标注",
			"引用指向错误的文档",
		},
		RootCause:     "生成过程中引用追踪丢失，或检索排序后引用索引错位",
		Remediation:   []string{"启用 Verifiable RAG 引用验证", "在 prompt 中强制要求引用格式", "使用后处理验证引用有效性"},
		Severity:      0.4,
		DetectionHint: "citation_verification.mismatch_rate > 0.3",
	})

	lib.register(&FailurePattern{
		ID:          P10_BrokenChainOfThought,
		Name:        "思维链断裂",
		Description: "LLM 的推理过程出现逻辑跳跃或自相矛盾",
		Symptoms: []string{
			"推理步骤之间缺少必要的中间推导",
			"结论与前面的推理矛盾",
			"推理过程中引用了不存在的上下文",
		},
		RootCause:     "上下文不连贯或推理链过长导致 LLM 失去上下文跟踪",
		Remediation:   []string{"缩短推理链长度", "在推理链中插入检查点", "使用 ReAct 模式逐步验证", "补充更多相关上下文"},
		Severity:      0.6,
		DetectionHint: "answer contains logical connectors but missing supporting context",
	})

	lib.register(&FailurePattern{
		ID:          P11_SafetyBreach,
		Name:        "安全违规",
		Description: "回答违反安全规则，泄露敏感信息或产生有害内容",
		Symptoms: []string{
			"SafetyGuard 拦截",
			"包含敏感关键词",
			"包含注入攻击特征",
		},
		RootCause:     "输入绕过了安全检查，或 LLM 未能遵守安全指令",
		Remediation:   []string{"增强安全过滤规则", "使用更强的安全系统提示", "启用输出验证", "记录审计日志"},
		Severity:      1.0,
		DetectionHint: "safety_guard.blocked == true",
	})

	lib.register(&FailurePattern{
		ID:          P12_TimeoutOrRateLimit,
		Name:        "超时或限流",
		Description: "请求因超时或 API 限流而失败",
		Symptoms: []string{
			"请求超时错误",
			"429 Too Many Requests",
			"部分结果缺失",
		},
		RootCause:     "LLM 响应过慢、网络问题、或并发请求超出 API 限额",
		Remediation:   []string{"增加请求超时时间", "启用请求重试机制", "使用连接池化 Provider", "降低并发或添加队列"},
		Severity:      0.3,
		DetectionHint: "error_type == 'timeout' || error_type == 'rate_limit'",
	})
}

func (lib *FailurePatternLibrary) register(p *FailurePattern) {
	lib.patterns[p.ID] = p
}

// Get returns a pattern by ID.
func (lib *FailurePatternLibrary) Get(id FailurePatternID) *FailurePattern {
	return lib.patterns[id]
}

// All returns all registered patterns.
func (lib *FailurePatternLibrary) All() []*FailurePattern {
	out := make([]*FailurePattern, 0, len(lib.patterns))
	for _, p := range lib.patterns {
		out = append(out, p)
	}
	return out
}

// DiagnoseInput holds the data needed for a single diagnostic run.
type DiagnoseInput struct {
	Question       string
	Answer         string
	RetrievedTexts []string
	Sources        []string
	SafetyBlocked  bool
	ErrorMessage   string
	TokenCount     int
	ContextWindow  int
}

// DiagnosticEngine runs pattern detection against QA pairs.
type DiagnosticEngine struct {
	lib      *FailurePatternLibrary
	provider provider.Provider
}

// NewDiagnosticEngine creates a diagnostic engine with the 12-pattern library.
func NewDiagnosticEngine(p provider.Provider) *DiagnosticEngine {
	return &DiagnosticEngine{
		lib:      NewFailurePatternLibrary(),
		provider: p,
	}
}

// Diagnose runs heuristic rule-based pattern matching against the input.
// Returns a DiagnosticSummary with all matched patterns.
func (de *DiagnosticEngine) Diagnose(ctx context.Context, input DiagnoseInput) *DiagnosticSummary {
	summary := &DiagnosticSummary{
		Question:      input.Question,
		TotalPatterns: len(de.lib.All()),
	}

	// Rule-based detection for each pattern
	detectors := []func(DiagnoseInput) *DiagnosticReport{
		de.detectP01_Underspecified,
		de.detectP02_Ambiguity,
		de.detectP04_IncompleteRetrieval,
		de.detectP05_IrrelevantRetrieval,
		de.detectP06_ContextOverflow,
		de.detectP09_PoorCitation,
		de.detectP11_SafetyBreach,
		de.detectP12_TimeoutOrRateLimit,
	}

	for _, detect := range detectors {
		report := detect(input)
		if report != nil && report.Matched {
			summary.Reports = append(summary.Reports, *report)
			summary.MatchedPatterns++
		}
	}

	// LLM-assisted detection for harder patterns
	if de.provider != nil && input.Answer != "" && len(input.RetrievedTexts) > 0 {
		llmReports := de.detectWithLLM(ctx, input)
		for _, r := range llmReports {
			if r.Matched {
				summary.Reports = append(summary.Reports, r)
				summary.MatchedPatterns++
			}
		}
	}

	// Compute overall health
	if len(summary.Reports) > 0 {
		totalSeverity := 0.0
		for _, r := range summary.Reports {
			totalSeverity += r.Severity * r.Confidence
		}
		summary.OverallHealth = math.Max(0, 1.0-totalSeverity/float64(len(summary.Reports)))
		// Find top failure by severity*confidence
		var top *DiagnosticReport
		for i := range summary.Reports {
			if top == nil || summary.Reports[i].Severity*summary.Reports[i].Confidence > top.Severity*top.Confidence {
				top = &summary.Reports[i]
			}
		}
		summary.TopFailure = top
	} else {
		summary.OverallHealth = 1.0
	}

	return summary
}

// ---------------------------------------------------------------------------
// Heuristic detectors for rule-based patterns
// ---------------------------------------------------------------------------

func (de *DiagnosticEngine) detectP01_Underspecified(input DiagnoseInput) *DiagnosticReport {
	p := de.lib.Get(P01_UnderspecifiedQuery)
	questionLen := len([]rune(input.Question))
	if questionLen >= 15 || len(input.RetrievedTexts) < 5 {
		return nil
	}
	confidence := math.Min(1.0, float64(5)/float64(max(len(input.RetrievedTexts), 1)))
	return &DiagnosticReport{
		PatternID:   P01_UnderspecifiedQuery,
		Matched:     true,
		Confidence:  roundDiag(confidence),
		Severity:    p.Severity,
		Evidence:    fmt.Sprintf("query length %d runes, retrieved %d texts", questionLen, len(input.RetrievedTexts)),
		Remediation: p.Remediation,
	}
}

func (de *DiagnosticEngine) detectP02_Ambiguity(input DiagnoseInput) *DiagnosticReport {
	p := de.lib.Get(P02_QueryAmbiguity)
	lowerQ := strings.ToLower(input.Question)
	ambiguityMarkers := []string{"还是", "或者", "哪个", "什么", "怎么", "如何", "and", "or", "which"}
	score := 0
	for _, m := range ambiguityMarkers {
		if strings.Contains(lowerQ, m) {
			score++
		}
	}
	// Check for pronouns without antecedent
	if strings.Contains(lowerQ, "它") || strings.Contains(lowerQ, "这个") || strings.Contains(lowerQ, "that") || strings.Contains(lowerQ, "it") {
		score++
	}
	if score < 2 {
		return nil
	}
	confidence := math.Min(1.0, float64(score)*0.35)
	return &DiagnosticReport{
		PatternID:   P02_QueryAmbiguity,
		Matched:     true,
		Confidence:  roundDiag(confidence),
		Severity:    p.Severity,
		Evidence:    fmt.Sprintf("found %d ambiguity markers", score),
		Remediation: p.Remediation,
	}
}

func (de *DiagnosticEngine) detectP04_IncompleteRetrieval(input DiagnoseInput) *DiagnosticReport {
	p := de.lib.Get(P04_IncompleteRetrieval)
	if len(input.RetrievedTexts) == 0 {
		return &DiagnosticReport{
			PatternID:   P04_IncompleteRetrieval,
			Matched:     true,
			Confidence:  1.0,
			Severity:    p.Severity,
			Evidence:    "no retrieved texts",
			Remediation: p.Remediation,
		}
	}
	lowerAnswer := strings.ToLower(input.Answer)
	insufficiencyMarkers := []string{"无法确认", "不完全", "根据已有信息", "信息不足", "无法确定", "not enough", "insufficient"}
	for _, marker := range insufficiencyMarkers {
		if strings.Contains(lowerAnswer, marker) {
			return &DiagnosticReport{
				PatternID:   P04_IncompleteRetrieval,
				Matched:     true,
				Confidence:  0.8,
				Severity:    p.Severity,
				Evidence:    fmt.Sprintf("answer contains insufficiency marker: %q", marker),
				Remediation: p.Remediation,
			}
		}
	}
	return nil
}

func (de *DiagnosticEngine) detectP05_IrrelevantRetrieval(input DiagnoseInput) *DiagnosticReport {
	p := de.lib.Get(P05_IrrelevantRetrieval)
	if len(input.RetrievedTexts) == 0 {
		return nil
	}
	// Simple keyword overlap check
	qWords := simpleTokenize(input.Question)
	totalOverlap := 0.0
	for _, text := range input.RetrievedTexts {
		tWords := simpleTokenize(text)
		overlap := jaccardOverlap(qWords, tWords)
		totalOverlap += overlap
	}
	avgOverlap := totalOverlap / float64(len(input.RetrievedTexts))
	if avgOverlap > 0.15 {
		return nil
	}
	return &DiagnosticReport{
		PatternID:   P05_IrrelevantRetrieval,
		Matched:     true,
		Confidence:  roundDiag(math.Min(1.0, (0.15-avgOverlap)*5)),
		Severity:    p.Severity,
		Evidence:    fmt.Sprintf("avg keyword overlap %.3f", avgOverlap),
		Remediation: p.Remediation,
	}
}

func (de *DiagnosticEngine) detectP06_ContextOverflow(input DiagnoseInput) *DiagnosticReport {
	p := de.lib.Get(P06_ContextWindowOverflow)
	if input.ContextWindow <= 0 || input.TokenCount <= 0 {
		return nil
	}
	ratio := float64(input.TokenCount) / float64(input.ContextWindow)
	if ratio < 0.85 {
		return nil
	}
	return &DiagnosticReport{
		PatternID:   P06_ContextWindowOverflow,
		Matched:     true,
		Confidence:  roundDiag(math.Min(1.0, (ratio-0.85)*6.67)),
		Severity:    p.Severity,
		Evidence:    fmt.Sprintf("token usage %.1f%% of context window", ratio*100),
		Remediation: p.Remediation,
	}
}

func (de *DiagnosticEngine) detectP09_PoorCitation(input DiagnoseInput) *DiagnosticReport {
	p := de.lib.Get(P09_PoorCitation)
	if input.Answer == "" {
		return nil
	}
	lowerAnswer := strings.ToLower(input.Answer)
	citationMarkers := []string{"[来源", "[source", "根据文档", "引用", "[[", "】"}
	hasCitation := false
	for _, m := range citationMarkers {
		if strings.Contains(lowerAnswer, m) {
			hasCitation = true
			break
		}
	}
	if hasCitation {
		return nil
	}
	if len(input.Sources) > 2 {
		return &DiagnosticReport{
			PatternID:   P09_PoorCitation,
			Matched:     true,
			Confidence:  0.6,
			Severity:    p.Severity,
			Evidence:    fmt.Sprintf("answer has no citation markers despite %d sources", len(input.Sources)),
			Remediation: p.Remediation,
		}
	}
	return nil
}

func (de *DiagnosticEngine) detectP11_SafetyBreach(input DiagnoseInput) *DiagnosticReport {
	if !input.SafetyBlocked {
		return nil
	}
	p := de.lib.Get(P11_SafetyBreach)
	return &DiagnosticReport{
		PatternID:   P11_SafetyBreach,
		Matched:     true,
		Confidence:  1.0,
		Severity:    p.Severity,
		Evidence:    "safety guard blocked the request",
		Remediation: p.Remediation,
	}
}

func (de *DiagnosticEngine) detectP12_TimeoutOrRateLimit(input DiagnoseInput) *DiagnosticReport {
	if input.ErrorMessage == "" {
		return nil
	}
	p := de.lib.Get(P12_TimeoutOrRateLimit)
	lower := strings.ToLower(input.ErrorMessage)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") ||
		strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "too many") {
		return &DiagnosticReport{
			PatternID:   P12_TimeoutOrRateLimit,
			Matched:     true,
			Confidence:  0.9,
			Severity:    p.Severity,
			Evidence:    fmt.Sprintf("error message: %s", utils.TruncateWithEllipsis(input.ErrorMessage, 200)),
			Remediation: p.Remediation,
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// LLM-assisted detection for harder patterns (P03, P07, P08, P10)
// ---------------------------------------------------------------------------

func (de *DiagnosticEngine) detectWithLLM(ctx context.Context, input DiagnoseInput) []DiagnosticReport {
	retrievedJoined := strings.Join(input.RetrievedTexts, "\n---\n")
	if len([]rune(retrievedJoined)) > 3000 {
		retrievedJoined = string([]rune(retrievedJoined)[:3000]) + "..."
	}
	answer := input.Answer
	if len([]rune(answer)) > 2000 {
		answer = string([]rune(answer)[:2000]) + "..."
	}

	prompt := fmt.Sprintf(`Analyze this QA pair and identify which of these failure patterns apply: P03_hallucination, P07_conflicting_evidence, P08_factual_drift, P10_broken_chain_of_thought.

Question: %s
Answer: %s
Retrieved Context: %s

For EACH pattern, output JSON array:
[{"id":"P03_hallucination","matched":true,"confidence":0.8,"evidence":"..."},
 {"id":"P07_conflicting_evidence","matched":false,"confidence":0.1,"evidence":"..."}]

Only output the JSON array.`, input.Question, answer, retrievedJoined)

	resp, err := de.provider.Chat(ctx, "你是一个RAG诊断专家。只输出JSON数组。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		return nil
	}

	var llmResults []struct {
		ID         string  `json:"id"`
		Matched    bool    `json:"matched"`
		Confidence float64 `json:"confidence"`
		Evidence   string  `json:"evidence"`
	}

	jsonStr := utils.ExtractJSONArray(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &llmResults); err != nil {
		return nil
	}

	var reports []DiagnosticReport
	for _, r := range llmResults {
		patternID := FailurePatternID(r.ID)
		p := de.lib.Get(patternID)
		if p == nil || !r.Matched {
			continue
		}
		reports = append(reports, DiagnosticReport{
			PatternID:   patternID,
			Matched:     true,
			Confidence:  roundDiag(r.Confidence),
			Severity:    p.Severity,
			Evidence:    r.Evidence,
			Remediation: p.Remediation,
		})
	}
	return reports
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func roundDiag(f float64) float64 {
	return math.Round(f*100) / 100
}

func simpleTokenize(s string) []string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer(",", " ", ".", " ", "!", " ", "?", " ", ";", " ", ":", " ", "(", " ", ")", " ", "\"", " ", "'", " ", "\n", " ", "\t", " ")
	s = replacer.Replace(s)
	return strings.Fields(s)
}

func jaccardOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, w := range a {
		setA[w] = true
	}
	setB := make(map[string]bool, len(b))
	for _, w := range b {
		setB[w] = true
	}
	inter := 0
	for w := range setA {
		if setB[w] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
