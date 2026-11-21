package enhanced

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TestCase 评估测试用例
type TestCase struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Question string          `json:"question"`
	SpaceID  int             `json:"space_id"`
	Expected ExpectedOutcome `json:"expected"`
	Tags     []string        `json:"tags,omitempty"`
}

// ExpectedOutcome 预期结果
type ExpectedOutcome struct {
	ContainsAny    []string `json:"contains_any,omitempty"`
	ContainsAll    []string `json:"contains_all,omitempty"`
	ExpectedDocIDs []int    `json:"expected_doc_ids,omitempty"`
	ExpectedTools  []string `json:"expected_tools,omitempty"`
	ExpectedTables []string `json:"expected_tables,omitempty"`
	NotContains    []string `json:"not_contains,omitempty"`
	MaxLatencyMs   int64    `json:"max_latency_ms,omitempty"`
}

// Source 评估用来源
type Source struct {
	DocumentID int `json:"document_id"`
}

// TestResult 测试结果
type TestResult struct {
	TestCaseID    string        `json:"test_case_id"`
	Passed        bool          `json:"passed"`
	ActualAnswer  string        `json:"actual_answer"`
	ActualSources []Source      `json:"actual_sources,omitempty"`
	ActualTools   []string      `json:"actual_tools,omitempty"`
	ActualTables  []string      `json:"actual_tables,omitempty"`
	DurationMs    int64         `json:"duration_ms"`
	Failures      []TestFailure `json:"failures,omitempty"`
}

// TestFailure 测试失败详情
type TestFailure struct {
	Type     string `json:"type"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

// RagRetriever RAG 检索接口
type RagRetriever interface {
	Ask(ctx context.Context, question string, spaceID int) (answer string, sources []Source, err error)
}

// SQLGenerator Text2SQL 接口
type SQLGenerator interface {
	Generate(ctx context.Context, question string, spaceID uint64) (sql string, tables []string, err error)
}

// TestRunner 测试运行器
type TestRunner struct {
	ragRetriever RagRetriever
	sqlGenerator SQLGenerator
}

// NewTestRunner 创建测试运行器
func NewTestRunner(rag RagRetriever, sql SQLGenerator) *TestRunner {
	return &TestRunner{ragRetriever: rag, sqlGenerator: sql}
}

// RunSuite 运行测试套件
func (r *TestRunner) RunSuite(ctx context.Context, cases []TestCase) []TestResult {
	results := make([]TestResult, 0, len(cases))
	for _, tc := range cases {
		results = append(results, r.RunSingle(ctx, tc))
	}
	return results
}

// RunSingle 运行单个测试用例
func (r *TestRunner) RunSingle(ctx context.Context, tc TestCase) TestResult {
	start := time.Now()
	result := TestResult{TestCaseID: tc.ID}

	// 判断是 RAG 还是 Text2SQL 用例
	if len(tc.Expected.ExpectedTables) > 0 && r.sqlGenerator != nil {
		sql, tables, err := r.sqlGenerator.Generate(ctx, tc.Question, uint64(tc.SpaceID))
		if err != nil {
			result.Failures = append(result.Failures, TestFailure{
				Type: "sql_generation_error", Expected: "success", Actual: err.Error(),
			})
		} else {
			result.ActualAnswer = sql
			result.ActualTables = tables
		}
	} else if r.ragRetriever != nil {
		answer, sources, err := r.ragRetriever.Ask(ctx, tc.Question, tc.SpaceID)
		if err != nil {
			result.Failures = append(result.Failures, TestFailure{
				Type: "rag_error", Expected: "success", Actual: err.Error(),
			})
		} else {
			result.ActualAnswer = answer
			for _, s := range sources {
				result.ActualSources = append(result.ActualSources, Source{DocumentID: s.DocumentID})
			}
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	result.Passed = len(result.Failures) == 0 && r.validateOutcome(tc, &result)

	return result
}

// validateOutcome 校验预期结果
func (r *TestRunner) validateOutcome(tc TestCase, result *TestResult) bool {
	answer := strings.ToLower(result.ActualAnswer)

	// contains_any：答案应至少包含其一
	if len(tc.Expected.ContainsAny) > 0 {
		found := false
		for _, s := range tc.Expected.ContainsAny {
			if strings.Contains(answer, strings.ToLower(s)) {
				found = true
				break
			}
		}
		if !found {
			result.Failures = append(result.Failures, TestFailure{
				Type: "contains_any", Expected: fmt.Sprint(tc.Expected.ContainsAny), Actual: result.ActualAnswer[:min(100, len(result.ActualAnswer))],
			})
		}
	}

	// contains_all：答案应全部包含
	for _, s := range tc.Expected.ContainsAll {
		if !strings.Contains(answer, strings.ToLower(s)) {
			result.Failures = append(result.Failures, TestFailure{
				Type: "contains_all", Expected: s, Actual: "missing",
			})
		}
	}

	// not_contains：答案不应包含
	for _, s := range tc.Expected.NotContains {
		if strings.Contains(answer, strings.ToLower(s)) {
			result.Failures = append(result.Failures, TestFailure{
				Type: "not_contains", Expected: "absent", Actual: s,
			})
		}
	}

	// expected_doc_ids：来源应包含指定文档
	if len(tc.Expected.ExpectedDocIDs) > 0 {
		actualIDs := make(map[int]bool)
		for _, s := range result.ActualSources {
			actualIDs[s.DocumentID] = true
		}
		for _, docID := range tc.Expected.ExpectedDocIDs {
			if !actualIDs[docID] {
				result.Failures = append(result.Failures, TestFailure{
					Type: "expected_doc_ids", Expected: fmt.Sprintf("doc_%d", docID), Actual: "not in sources",
				})
			}
		}
	}

	// expected_tables：SQL 应包含指定表名
	if len(tc.Expected.ExpectedTables) > 0 {
		actualTables := make(map[string]bool)
		for _, t := range result.ActualTables {
			actualTables[strings.ToLower(t)] = true
		}
		for _, table := range tc.Expected.ExpectedTables {
			if !actualTables[strings.ToLower(table)] {
				result.Failures = append(result.Failures, TestFailure{
					Type: "expected_tables", Expected: table, Actual: "not in SQL",
				})
			}
		}
	}

	// max_latency_ms：响应时间限制
	if tc.Expected.MaxLatencyMs > 0 && result.DurationMs > tc.Expected.MaxLatencyMs {
		result.Failures = append(result.Failures, TestFailure{
			Type: "latency", Expected: fmt.Sprintf("%dms", tc.Expected.MaxLatencyMs), Actual: fmt.Sprintf("%dms", result.DurationMs),
		})
	}

	return len(result.Failures) == 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ===== A/B Comparison =====

// AgentVariant Agent 变体配置
type AgentVariant struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Config VariantConfig `json:"config"`
}

// VariantConfig 变体配置
type VariantConfig struct {
	Provider    string  `json:"provider,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

// ABResult A/B 测试结果
type ABResult struct {
	TestCaseID string                `json:"test_case_id"`
	Variants   map[string]TestResult `json:"variants"`
	Winner     string                `json:"winner"`
	Summary    string                `json:"summary"`
}

// ABComparison A/B 对比器
type ABComparison struct {
	runner *TestRunner
}

// NewABComparison 创建 A/B 对比器
func NewABComparison(runner *TestRunner) *ABComparison {
	return &ABComparison{runner: runner}
}

// Compare 对比多个变体（当前实现：用同一 runner 跑同一套用例，实际使用时可切换 provider）
func (a *ABComparison) Compare(ctx context.Context, cases []TestCase, variants []AgentVariant) []ABResult {
	results := make([]ABResult, 0, len(cases))
	for _, tc := range cases {
		ab := ABResult{
			TestCaseID: tc.ID,
			Variants:   make(map[string]TestResult),
		}
		for _, v := range variants {
			// 实际场景：这里根据 v.Config 切换 provider/model
			// 当前骨架：用默认 runner 执行
			r := a.runner.RunSingle(ctx, tc)
			ab.Variants[v.ID] = r
		}
		// 选出通过且延迟最低的
		winner := ""
		bestLatency := int64(1 << 62)
		for vID, r := range ab.Variants {
			if r.Passed && r.DurationMs < bestLatency {
				winner = vID
				bestLatency = r.DurationMs
			}
		}
		ab.Winner = winner
		if winner == "" {
			ab.Summary = "all variants failed"
		} else {
			ab.Summary = fmt.Sprintf("winner: %s (%dms)", winner, bestLatency)
		}
		results = append(results, ab)
	}
	return results
}
