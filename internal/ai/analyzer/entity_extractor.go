package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

type EntityExtractor struct {
	provider provider.Provider
}

func NewEntityExtractor(p provider.Provider) *EntityExtractor {
	return &EntityExtractor{
		provider: p,
	}
}

type ExtractedEntities struct {
	URLs          []URLInfo      `json:"urls"`
	ConfigItems   []ConfigItem   `json:"config_items"`
	Aliases       []string       `json:"aliases"`
	ActionItems   []string       `json:"action_items"`
	URLGroups     []URLGroup     `json:"url_groups"`
	Technologies  []Technology   `json:"technologies"`
	Concepts      []Concept      `json:"concepts"`
	People        []Person       `json:"people"`
	Organizations []Organization `json:"organizations"`
	Metrics       []Metric       `json:"metrics"`
}

type Technology struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

type Concept struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Aliases    []string `json:"aliases"`
	Confidence float64  `json:"confidence"`
}

type Person struct {
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

type Organization struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

type Metric struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Unit  string `json:"unit"`
}

type URLGroup struct {
	Service string   `json:"service"`
	URLs    []string `json:"urls"`
	Note    string   `json:"note"`
}

func (e *EntityExtractor) ExtractFromContent(ctx context.Context, content string) (*ExtractedEntities, error) {
	if e.provider == nil {
		return e.regexExtract(content), nil
	}

	if len(content) > 12000 {
		content = utils.TruncateRunes(content, 12000)
	}

	prompt := `你是一个信息提取专家。请从以下文档中提取所有结构化信息。
重点关注：

1. URL/链接：所有门户地址、API 端点、文档链接
   - 识别同一服务的多个访问地址（如内网地址、外网地址、测试地址）
2. 配置项：所有 key=value 或配置参数
3. 别名/别称：同一事物的不同名称
4. 行动项：待办事项、操作步骤
5. 技术栈：编程语言、框架、工具、平台
6. 概念定义：专业术语、抽象概念及其定义
7. 人物角色：提到的人名及其角色/职责
8. 组织机构：提到的公司、部门、团队
9. 指标数据：性能指标、统计数据、度量值

文档内容：
%s

输出 JSON：
{
  "urls": [{"url": "...", "label": "...", "type": "portal|api|doc|other", "is_primary": false}],
  "config_items": [{"key": "...", "value": "...", "description": "..."}],
  "aliases": ["别名 1", "别名 2"],
  "action_items": ["待办 1", "待办 2"],
  "url_groups": [{"service": "服务名", "urls": ["地址 1", "地址 2"], "note": "说明"}],
  "technologies": [{"name": "...", "category": "语言|框架|工具|平台", "confidence": 0.9}],
  "concepts": [{"name": "...", "definition": "...", "aliases": ["别名"], "confidence": 0.85}],
  "people": [{"name": "...", "role": "...", "description": "...", "confidence": 0.8}],
  "organizations": [{"name": "...", "type": "公司|部门|团队", "description": "...", "confidence": 0.8}],
  "metrics": [{"name": "...", "value": "...", "unit": "..."}]
}`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := e.provider.Chat(ctx, "你是一个信息提取专家，只输出 JSON，不要解释。", []provider.Message{
		provider.NewTextMessage("user", fmt.Sprintf(prompt, content)),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[EntityExtractor] LLM extract error: %v, fallback to regex", err)
		return e.regexExtract(content), nil
	}

	entities := &ExtractedEntities{}
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), entities); err != nil {
		log.Errorf("[EntityExtractor] JSON parse error: %v, fallback to regex", err)
		return e.regexExtract(content), nil
	}

	return entities, nil
}

func (e *EntityExtractor) regexExtract(content string) *ExtractedEntities {
	entities := &ExtractedEntities{
		URLs:          []URLInfo{},
		ConfigItems:   []ConfigItem{},
		Aliases:       []string{},
		ActionItems:   []string{},
		URLGroups:     []URLGroup{},
		Technologies:  []Technology{},
		Concepts:      []Concept{},
		People:        []Person{},
		Organizations: []Organization{},
		Metrics:       []Metric{},
	}

	urlRe := regexp.MustCompile(`https?://[^\s<>"{}|\\^\[\]]+`)
	urlMatches := urlRe.FindAllString(content, -1)
	for _, url := range urlMatches {
		label := extractURLLabel(url, content)
		urlType := classifyURL(url, label)
		isPrimary := strings.Contains(url, "www.") || strings.Contains(url, "api.")

		entities.URLs = append(entities.URLs, URLInfo{
			URL:       url,
			Label:     label,
			Type:      urlType,
			IsPrimary: isPrimary,
		})
	}

	configRe := regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*[=:]\s*(.+)$`)
	configMatches := configRe.FindAllStringSubmatch(content, -1)
	for _, match := range configMatches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])
			if len(key) > 0 && len(value) > 0 && len(key) < 100 && len(value) < 500 {
				entities.ConfigItems = append(entities.ConfigItems, ConfigItem{
					Key:         key,
					Value:       value,
					Description: "",
				})
			}
		}
	}

	actionKeywords := []string{"待办", "TODO", "需要", "必须", "请", "应该", "步骤", "首先", "然后", "最后"}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 10 && len(trimmed) < 200 {
			for _, keyword := range actionKeywords {
				if strings.Contains(trimmed, keyword) {
					entities.ActionItems = append(entities.ActionItems, trimmed)
					break
				}
			}
		}
	}

	if len(entities.URLs) > 1 {
		domainMap := make(map[string][]string)
		for _, urlInfo := range entities.URLs {
			domain := extractDomain(urlInfo.URL)
			if domain != "" {
				domainMap[domain] = append(domainMap[domain], urlInfo.URL)
			}
		}

		for domain, urls := range domainMap {
			if len(urls) > 1 {
				entities.URLGroups = append(entities.URLGroups, URLGroup{
					Service: domain,
					URLs:    urls,
					Note:    fmt.Sprintf("同一服务的多个访问地址：%d 个", len(urls)),
				})
			}
		}
	}

	return entities
}

func extractURLLabel(url string, content string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if lastPart != "" && len(lastPart) < 50 {
			return lastPart
		}
	}

	for i := 0; i < len(content)-50; i++ {
		if i > 0 && content[i-1] == '>' && content[i:i+len(url)] == url {
			start := i - 50
			if start < 0 {
				start = 0
			}
			label := strings.TrimSpace(content[start:i])
			label = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(label, "")
			if len(label) > 0 && len(label) < 50 {
				return label
			}
		}
	}

	domain := extractDomain(url)
	if domain != "" {
		return domain
	}

	return "链接"
}

func classifyURL(url string, label string) string {
	if strings.Contains(url, "/api/") || strings.Contains(url, "api.") || strings.Contains(url, "/v1/") || strings.Contains(url, "/v2/") {
		return "api"
	}

	if strings.Contains(label, "文档") || strings.Contains(label, "doc") || strings.Contains(url, "/doc/") || strings.Contains(url, "/docs/") {
		return "doc"
	}

	if strings.Contains(url, "portal") || strings.Contains(url, "console") || strings.Contains(url, "admin") {
		return "portal"
	}

	return "other"
}

func extractDomain(url string) string {
	if !strings.HasPrefix(url, "http") {
		return ""
	}

	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		domain := parts[0]
		domain = strings.Split(domain, ":")[0]
		return domain
	}

	return ""
}
