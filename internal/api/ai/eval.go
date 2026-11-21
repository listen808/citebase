package ai

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	evalpkg "github.com/listen808/citebase/internal/ai/eval"
	"github.com/listen808/citebase/internal/ai/eval/enhanced"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/response"
)

// EvalAPI 统一评估 API
// 集成 RAGAS 数据集评估 + 端到端 TestCase 评估 + A/B 对比
type EvalAPI struct {
	repo *evalpkg.DatasetRepository

	// TestCase 评估（进程内存储）
	mu        sync.RWMutex
	testCases map[string]enhanced.TestCase
}

// NewEvalAPI 创建评估 API 实例
func NewEvalAPI() *EvalAPI {
	return &EvalAPI{
		repo:      evalpkg.NewDatasetRepository(),
		testCases: make(map[string]enhanced.TestCase),
	}
}

// GetAutoEvalStatus P0-3：暴露自动化评测的运行态。
// 评测接上调度之后必须能看到"它到底跑没跑、上一次几分"，否则等于没接。
func (api *EvalAPI) GetAutoEvalStatus(c *gin.Context) {
	ae := aimod.GetModule().AutoEvaluator()
	if ae == nil {
		response.Success(c, gin.H{"enabled": false, "reason": "auto evaluator not assembled"})
		return
	}
	response.Success(c, gin.H{
		"enabled":  true,
		"stats":    ae.GetStats(),
		"last_run": ae.GetLastResult(),
	})
}

// ListDatasets 列出评估数据集
func (api *EvalAPI) ListDatasets(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	datasets, total, err := api.repo.ListDatasets(limit, offset)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "failed to list datasets", err)
		return
	}

	response.Success(c, gin.H{
		"datasets": datasets,
		"total":    total,
	})
}

// CreateDataset 创建评估数据集
func (api *EvalAPI) CreateDataset(c *gin.Context) {
	var req struct {
		Name      string                 `json:"name" binding:"required"`
		SpaceID   int                    `json:"space_id"`
		Questions []evalpkg.EvalQuestion `json:"questions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name and questions are required")
		return
	}

	ds, err := api.repo.CreateDataset(req.Name, req.SpaceID, req.Questions)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "failed to create dataset", err)
		return
	}

	response.Success(c, gin.H{"dataset": ds})
}

// GetDataset 获取数据集详情
func (api *EvalAPI) GetDataset(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid dataset id")
		return
	}

	ds, err := api.repo.GetDataset(id)
	if err != nil {
		response.NotFound(c, "dataset not found")
		return
	}

	response.Success(c, gin.H{"dataset": ds})
}

// DeleteDataset 删除数据集
func (api *EvalAPI) DeleteDataset(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid dataset id")
		return
	}

	if err := api.repo.DeleteDataset(id); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "failed to delete dataset", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// RunEval 运行评估
func (api *EvalAPI) RunEval(c *gin.Context) {
	var req struct {
		DatasetID int64 `json:"dataset_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "dataset_id is required")
		return
	}

	ds, err := api.repo.GetDataset(req.DatasetID)
	if err != nil {
		response.NotFound(c, "dataset not found")
		return
	}

	m := aimod.GetModule()
	if m == nil || !m.Enabled() {
		response.ServerError(c, "AI module not enabled")
		return
	}

	prov := m.PrimaryProvider()
	retriever := m.Retriever()

	evaluator := evalpkg.NewEvaluator(prov, retriever, api.repo)

	ctx := context.Background()
	result, err := evaluator.Evaluate(ctx, ds.Questions, ds.SpaceID, ds.ID, ds.Name)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "evaluation failed", err)
		return
	}

	response.Success(c, gin.H{"result": result})
}

// ListResults 列出评估结果
func (api *EvalAPI) ListResults(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	results, total, err := api.repo.ListResults(limit, offset)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "failed to list results", err)
		return
	}

	response.Success(c, gin.H{
		"results": results,
		"total":   total,
	})
}

// GetResult 获取评估结果详情
func (api *EvalAPI) GetResult(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid result id")
		return
	}

	result, err := api.repo.GetResult(id)
	if err != nil {
		response.NotFound(c, "result not found")
		return
	}

	response.Success(c, gin.H{"result": result})
}

// DeleteResult 删除评估结果
func (api *EvalAPI) DeleteResult(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid result id")
		return
	}

	if err := api.repo.DeleteResult(id); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "failed to delete result", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

// --- TestCase 评估（端到端） ---

// CreateTestCase 创建测试用例
func (api *EvalAPI) CreateTestCase(c *gin.Context) {
	var tc enhanced.TestCase
	if err := c.ShouldBindJSON(&tc); err != nil {
		response.BadRequest(c, "invalid test case payload")
		return
	}
	if tc.ID == "" {
		tc.ID = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	if tc.Name == "" {
		tc.Name = tc.ID
	}

	api.mu.Lock()
	api.testCases[tc.ID] = tc
	api.mu.Unlock()

	response.Success(c, gin.H{"test_case": tc})
}

// ListTestCases 列出所有测试用例
func (api *EvalAPI) ListTestCases(c *gin.Context) {
	api.mu.RLock()
	defer api.mu.RUnlock()

	cases := make([]enhanced.TestCase, 0, len(api.testCases))
	for _, tc := range api.testCases {
		cases = append(cases, tc)
	}
	response.Success(c, gin.H{"test_cases": cases, "total": len(cases)})
}

// RunTestCases 运行测试套件
func (api *EvalAPI) RunTestCases(c *gin.Context) {
	var req struct {
		TestCaseIDs []string `json:"test_case_ids"`
	}
	_ = c.ShouldBindJSON(&req)

	api.mu.RLock()
	allCases := make([]enhanced.TestCase, 0, len(api.testCases))
	if len(req.TestCaseIDs) == 0 {
		for _, tc := range api.testCases {
			allCases = append(allCases, tc)
		}
	} else {
		wanted := make(map[string]bool, len(req.TestCaseIDs))
		for _, id := range req.TestCaseIDs {
			wanted[id] = true
		}
		for id, tc := range api.testCases {
			if wanted[id] {
				allCases = append(allCases, tc)
			}
		}
	}
	api.mu.RUnlock()

	if len(allCases) == 0 {
		response.BadRequest(c, "no test cases to run")
		return
	}

	m := aimod.GetModule()
	if m == nil || !m.Enabled() {
		response.ServerError(c, "AI module not enabled")
		return
	}

	runner := buildTestRunner(m)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	results := runner.RunSuite(ctx, allCases)

	passed := 0
	totalLatency := int64(0)
	for _, r := range results {
		if r.Passed {
			passed++
		}
		totalLatency += r.DurationMs
	}

	response.Success(c, gin.H{
		"results":       results,
		"total":         len(results),
		"passed":        passed,
		"failed":        len(results) - passed,
		"pass_rate":     float64(passed) / float64(len(results)),
		"total_latency": totalLatency,
	})
}

// ABCompare A/B 对比测试
func (api *EvalAPI) ABCompare(c *gin.Context) {
	var req struct {
		TestCaseIDs []string                `json:"test_case_ids"`
		Variants    []enhanced.AgentVariant `json:"variants"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid payload")
		return
	}
	if len(req.Variants) < 2 {
		response.BadRequest(c, "at least 2 variants required")
		return
	}

	api.mu.RLock()
	allCases := make([]enhanced.TestCase, 0, len(api.testCases))
	if len(req.TestCaseIDs) == 0 {
		for _, tc := range api.testCases {
			allCases = append(allCases, tc)
		}
	} else {
		wanted := make(map[string]bool, len(req.TestCaseIDs))
		for _, id := range req.TestCaseIDs {
			wanted[id] = true
		}
		for id, tc := range api.testCases {
			if wanted[id] {
				allCases = append(allCases, tc)
			}
		}
	}
	api.mu.RUnlock()

	if len(allCases) == 0 {
		response.BadRequest(c, "no test cases to compare")
		return
	}

	m := aimod.GetModule()
	if m == nil || !m.Enabled() {
		response.ServerError(c, "AI module not enabled")
		return
	}

	runner := buildTestRunner(m)
	comparison := enhanced.NewABComparison(runner)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Minute)
	defer cancel()

	results := comparison.Compare(ctx, allCases, req.Variants)
	response.Success(c, gin.H{"results": results, "total": len(results)})
}

// buildTestRunner 从 AI 模块构建 TestRunner
func buildTestRunner(m *aimod.Module) *enhanced.TestRunner {
	var ragAdapter enhanced.RagRetriever
	if r := m.Retriever(); r != nil {
		ragAdapter = &ragRetrieverAdapter{retriever: r}
	}
	return enhanced.NewTestRunner(ragAdapter, nil)
}

// ragRetrieverAdapter 将 rag.Retriever 适配为 enhanced.RagRetriever
type ragRetrieverAdapter struct {
	retriever *rag.Retriever
}

func (a *ragRetrieverAdapter) Ask(ctx context.Context, question string, spaceID int) (string, []enhanced.Source, error) {
	ans, err := a.retriever.Ask(ctx, question, spaceID, 5)
	if err != nil {
		return "", nil, err
	}
	if ans == nil {
		return "", nil, nil
	}
	sources := make([]enhanced.Source, 0, len(ans.Sources))
	for _, s := range ans.Sources {
		sources = append(sources, enhanced.Source{DocumentID: s.DocumentID})
	}
	return ans.Answer, sources, nil
}

var _ enhanced.RagRetriever = (*ragRetrieverAdapter)(nil)
