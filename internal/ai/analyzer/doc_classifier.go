package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
)

// DocClassifier 智能文档分类器
// 基于LLM的文档类型分类，支持技术文档、API文档、业务文档、会议纪要等多类别
type DocClassifier struct {
	provider provider.Provider
}

// ClassificationResult 文档分类结果
type ClassificationResult struct {
	Category    string   `json:"category"`     // 主分类
	SubCategory string   `json:"sub_category"` // 子分类
	Confidence  float64  `json:"confidence"`   // 置信度
	Domain      string   `json:"domain"`       // 业务领域
	DocType     string   `json:"doc_type"`     // 文档类型
	Tags        []string `json:"tags"`         // 自动标签
	Audience    string   `json:"audience"`     // 目标受众
	Complexity  string   `json:"complexity"`   // 复杂度
	Language    string   `json:"language"`     // 语言
	Summary     string   `json:"summary"`      // 一句话摘要
}

// DocCategory 预定义文档类别
var DocCategories = []string{
	"技术架构文档",
	"API接口文档",
	"运维部署手册",
	"产品需求文档",
	"项目管理文档",
	"测试用例文档",
	"用户手册",
	"会议纪要",
	"技术方案设计",
	"数据库设计文档",
	"代码规范文档",
	"故障排查手册",
	"培训材料",
	"行业研究报告",
	"周报/月报",
	"变更记录",
	"安全合规文档",
	"通用知识文档",
}

func NewDocClassifier(p provider.Provider) *DocClassifier {
	return &DocClassifier{provider: p}
}

// Classify 对文档内容进行分类
func (dc *DocClassifier) Classify(ctx context.Context, title, content string) (*ClassificationResult, error) {
	if dc.provider == nil {
		return dc.fastClassify(title, content), nil
	}

	// 截取内容避免过长
	sample := content
	if len(sample) > 3000 {
		sample = sample[:3000]
	}

	categories := strings.Join(DocCategories, "、")
	prompt := fmt.Sprintf(`请分析以下文档，给出详细的分类信息。输出JSON格式。

文档标题: %s

文档内容(前3000字):
%s

可选的分类列表: %s

输出JSON格式:
{
  "category": "主分类",
  "sub_category": "子分类",
  "confidence": 0.9,
  "domain": "业务领域（如：金融、医疗、电商、教育等）",
  "doc_type": "文档类型（如：设计文档、操作手册、API文档等）",
  "tags": ["标签1", "标签2", "标签3", "标签4", "标签5"],
  "audience": "目标受众（开发者、运维人员、产品经理、全员等）",
  "complexity": "复杂度（low/medium/high）",
  "language": "语言（zh/en/mixed）",
  "summary": "一句话摘要（20字以内）"
}

要求:
- category从候选列表中选取最匹配的
- 生成5-10个有区分度的标签
- 评估文档的技术复杂度`, title, sample, categories)

	resp, err := dc.provider.Chat(ctx, "你是一个专业的文档分类助手。只输出JSON，不要其他内容。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(512), provider.WithTemperature(0.2), provider.WithJSONMode(),
	)
	if err != nil {
		log.Errorf("[DocClassifier] LLM classify error: %v, using fast classify", err)
		return dc.fastClassify(title, content), nil
	}

	result := parseClassifyResponse(resp.Content, title)
	if result == nil {
		return dc.fastClassify(title, content), nil
	}

	return result, nil
}

// fastClassify 快速分类（基于规则，不依赖LLM）
func (dc *DocClassifier) fastClassify(title, content string) *ClassificationResult {
	lowerTitle := strings.ToLower(title)
	lowerContent := strings.ToLower(content[:min(len(content), 500)])

	result := &ClassificationResult{
		Language: detectLanguage(content),
		Tags:     extractSimpleKeywords(content, 8),
	}

	// 基于关键词规则分类
	switch {
	case matchAny(lowerTitle, "api", "接口", "rest", "graphql", "grpc", "openapi"):
		result.Category = "API接口文档"
		result.DocType = "API文档"
		result.Audience = "开发者"

	case matchAny(lowerTitle, "部署", "运维", "docker", "kubernetes", "k8s", "部署", "监控", "告警"):
		result.Category = "运维部署手册"
		result.DocType = "操作手册"
		result.Audience = "运维人员"

	case matchAny(lowerTitle+lowerContent, "数据库", "表结构", "ddl", "sql", "schema", "索引", "数据模型"):
		result.Category = "数据库设计文档"
		result.DocType = "设计文档"
		result.Audience = "开发者"

	case matchAny(lowerTitle, "方案", "设计", "架构", "技术选型", "系统设计"):
		result.Category = "技术方案设计"
		result.DocType = "设计文档"
		result.Audience = "技术团队"

	case matchAny(lowerTitle, "需求", "prd", "产品", "功能", "迭代"):
		result.Category = "产品需求文档"
		result.DocType = "需求文档"
		result.Audience = "产品团队"

	case matchAny(lowerTitle, "会议", "纪要", "周会", "例会", "review"):
		result.Category = "会议纪要"
		result.DocType = "会议纪要"
		result.Audience = "全员"

	case matchAny(lowerTitle, "测试", "用例", "qa", "bug", "缺陷"):
		result.Category = "测试用例文档"
		result.DocType = "测试文档"
		result.Audience = "测试团队"

	case matchAny(lowerTitle, "规范", "标准", "编码", "code style", "最佳实践"):
		result.Category = "代码规范文档"
		result.DocType = "规范文档"
		result.Audience = "开发者"

	case matchAny(lowerTitle, "故障", "排查", "troubleshoot", "修复", "bug fix"):
		result.Category = "故障排查手册"
		result.DocType = "操作手册"
		result.Audience = "运维人员"

	case matchAny(lowerTitle, "安全", "合规", "审计", "权限", "认证"):
		result.Category = "安全合规文档"
		result.DocType = "规范文档"
		result.Audience = "安全团队"

	case matchAny(lowerTitle, "周报", "月报", "日报", "总结", "汇报"):
		result.Category = "周报/月报"
		result.DocType = "报告"
		result.Audience = "全员"

	case matchAny(lowerTitle, "培训", "教程", "入门", "指南", "tutorial", "guide"):
		result.Category = "培训材料"
		result.DocType = "教程"
		result.Audience = "全员"

	default:
		result.Category = "通用知识文档"
		result.DocType = "通用文档"
		result.Audience = "全员"
	}

	// 评估复杂度
	result.Complexity = assessComplexity(content)

	// 推断子分类
	result.SubCategory = inferSubCategory(lowerTitle, lowerContent, result.Category)

	// 置信度：基于规则的置信度
	result.Confidence = 0.75

	// 生成摘要
	result.Summary = generateSimpleSummary(title, content)

	return result
}

// matchAny 检查是否包含任一关键词
func matchAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// assessComplexity 评估文档复杂度
func assessComplexity(content string) string {
	lines := strings.Count(content, "\n")
	codeBlocks := strings.Count(content, "```")
	tables := strings.Count(content, "| ---")
	techTerms := countTechTerms(content)

	score := lines/200 + codeBlocks/4 + tables + techTerms/5

	if score >= 10 {
		return "high"
	} else if score >= 4 {
		return "medium"
	}
	return "low"
}

// countTechTerms 统计技术术语出现次数
func countTechTerms(content string) int {
	techTerms := []string{
		"config", "config", "deploy", "deploy", "server", "client", "database", "数据库",
		"cache", "redis", "queue", "异步", "async", "cluster", "集群", "docker",
		"kubernetes", "k8s", "pipeline", "ci/cd", "微服务", "microservice",
		"api", "rest", "graphql", "grpc", "websocket", "负载均衡", "load balancer",
	}
	count := 0
	lower := strings.ToLower(content)
	for _, term := range techTerms {
		count += strings.Count(lower, term)
	}
	return count
}

// inferSubCategory 根据主分类推断子分类
func inferSubCategory(title, content, category string) string {
	switch category {
	case "技术架构文档":
		if matchAny(title+content, "微服务", "microservice") {
			return "微服务架构"
		} else if matchAny(title+content, "单体", "monolith") {
			return "单体架构"
		}
		return "系统架构"
	case "API接口文档":
		if matchAny(title+content, "rest") {
			return "REST API"
		} else if matchAny(title+content, "graphql") {
			return "GraphQL API"
		} else if matchAny(title+content, "grpc") {
			return "gRPC API"
		}
		return "通用API"
	case "数据库设计文档":
		if matchAny(title+content, "mysql") {
			return "MySQL设计"
		} else if matchAny(title+content, "postgresql") {
			return "PostgreSQL设计"
		} else if matchAny(title+content, "mongodb") {
			return "MongoDB设计"
		}
		return "数据库设计"
	default:
		return ""
	}
}

// extractJSONFieldSimple 从JSON字符串中提取字段值（简化版，用于doc_classifier）
func extractJSONFieldSimple(jsonStr, fieldName string) string {
	key := fmt.Sprintf(`"%s"`, fieldName)
	idx := strings.Index(jsonStr, key)
	if idx < 0 {
		return ""
	}
	rest := jsonStr[idx+len(key):]
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[colonIdx+1:])

	if strings.HasPrefix(rest, `"`) {
		rest = rest[1:]
		endIdx := strings.Index(rest, `"`)
		if endIdx < 0 {
			return rest
		}
		return rest[:endIdx]
	}

	endIdx := strings.IndexAny(rest, ",}\n")
	if endIdx < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:endIdx])
}

// generateSimpleSummary 生成简单摘要
func generateSimpleSummary(title, content string) string {
	// 取第一段非空内容作为摘要
	lines := strings.Split(content, "\n")
	summary := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") {
			continue
		}
		summary = trimmed
		break
	}
	if len(summary) > 80 {
		summary = summary[:80] + "..."
	}
	if summary == "" {
		summary = title
	}
	return summary
}

// parseClassifyResponse 解析LLM返回的分类JSON
func parseClassifyResponse(content, fallbackTitle string) *ClassificationResult {
	jsonStr := content
	if idx := strings.Index(content, "{"); idx >= 0 {
		jsonStr = content[idx:]
		if last := strings.LastIndex(jsonStr, "}"); last >= 0 {
			jsonStr = jsonStr[:last+1]
		}
	}

	result := &ClassificationResult{
		Category:    extractJSONFieldSimple(jsonStr, "category"),
		SubCategory: extractJSONFieldSimple(jsonStr, "sub_category"),
		DocType:     extractJSONFieldSimple(jsonStr, "doc_type"),
		Domain:      extractJSONFieldSimple(jsonStr, "domain"),
		Audience:    extractJSONFieldSimple(jsonStr, "audience"),
		Complexity:  extractJSONFieldSimple(jsonStr, "complexity"),
		Language:    extractJSONFieldSimple(jsonStr, "language"),
		Summary:     extractJSONFieldSimple(jsonStr, "summary"),
	}

	// 解析tags数组
	if tagsIdx := strings.Index(jsonStr, `"tags"`); tagsIdx >= 0 {
		arrStart := strings.Index(jsonStr[tagsIdx:], "[")
		arrEnd := strings.Index(jsonStr[tagsIdx:], "]")
		if arrStart >= 0 && arrEnd > arrStart {
			arrStr := jsonStr[tagsIdx+arrStart+1 : tagsIdx+arrEnd]
			for _, tag := range strings.Split(arrStr, ",") {
				tag = strings.Trim(tag, ` "`)
				if tag != "" {
					result.Tags = append(result.Tags, tag)
				}
			}
		}
	}

	// 解析confidence
	if confIdx := strings.Index(jsonStr, `"confidence"`); confIdx >= 0 {
		fmt.Sscanf(jsonStr[confIdx:], `"confidence": %f`, &result.Confidence)
	}

	if result.Category == "" {
		return nil
	}

	return result
}
