package text2sql

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ConversationCompressor 对话历史压缩器
type ConversationCompressor struct {
	maxTokens   int // 最大 token 数（估算）
	maxMessages int // 最大消息数
	keepRecentN int // 保留最近 N 条消息不压缩
}

// NewConversationCompressor 创建对话压缩器
func NewConversationCompressor(maxTokens, maxMessages, keepRecentN int) *ConversationCompressor {
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	if maxMessages <= 0 {
		maxMessages = 10
	}
	if keepRecentN <= 0 {
		keepRecentN = 3
	}
	return &ConversationCompressor{
		maxTokens:   maxTokens,
		maxMessages: maxMessages,
		keepRecentN: keepRecentN,
	}
}

// CompressedMessage 压缩后的消息
type CompressedMessage struct {
	Role         string
	Content      string
	SQL          string
	IsCompressed bool
	KeyInfo      *ConversationKeyInfo // 提取的关键信息
}

// ConversationKeyInfo 对话关键信息
type ConversationKeyInfo struct {
	Tables       []string // 涉及的表
	Fields       []string // 涉及的字段
	Filters      []string // 过滤条件
	JoinTypes    []string // JOIN 类型
	Aggregations []string // 聚合函数
	OrderBy      []string // 排序字段
	Limit        int      // LIMIT 值
}

// CompressHistory 压缩对话历史
func (c *ConversationCompressor) CompressHistory(history []ConversationMessage) []CompressedMessage {
	if len(history) <= c.maxMessages {
		// 不需要压缩，只提取关键信息
		result := make([]CompressedMessage, len(history))
		for i, msg := range history {
			result[i] = CompressedMessage{
				Role:         msg.Role,
				Content:      msg.Content,
				SQL:          msg.SQL,
				IsCompressed: false,
				KeyInfo:      c.extractKeyInfo(msg),
			}
		}
		return result
	}

	// 需要压缩：保留最近 N 条，压缩前面的
	toCompress := history[:len(history)-c.keepRecentN]
	toKeep := history[len(history)-c.keepRecentN:]

	var result []CompressedMessage

	// 压缩前面的消息
	if len(toCompress) > 0 {
		compressed := c.compressMessages(toCompress)
		result = append(result, compressed)
	}

	// 保留最近的消息
	for _, msg := range toKeep {
		result = append(result, CompressedMessage{
			Role:         msg.Role,
			Content:      msg.Content,
			SQL:          msg.SQL,
			IsCompressed: false,
			KeyInfo:      c.extractKeyInfo(msg),
		})
	}

	return result
}

// compressMessages 压缩多条消息为一条摘要
func (c *ConversationCompressor) compressMessages(messages []ConversationMessage) CompressedMessage {
	var allTables, allFields, allFilters []string
	var sqlCount int

	for _, msg := range messages {
		info := c.extractKeyInfo(msg)
		if info != nil {
			allTables = append(allTables, info.Tables...)
			allFields = append(allFields, info.Fields...)
			allFilters = append(allFilters, info.Filters...)
			if msg.SQL != "" {
				sqlCount++
			}
		}
	}

	// 去重
	allTables = uniqueStrings(allTables)
	allFields = uniqueStrings(allFields)
	allFilters = uniqueStrings(allFilters)

	// 构建压缩摘要
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("[对话摘要: 共 %d 轮交互, %d 次SQL生成]\n", len(messages), sqlCount))

	if len(allTables) > 0 {
		summary.WriteString(fmt.Sprintf("涉及表: %s\n", strings.Join(allTables, ", ")))
	}
	if len(allFields) > 0 {
		summary.WriteString(fmt.Sprintf("涉及字段: %s\n", strings.Join(allFields, ", ")))
	}
	if len(allFilters) > 0 {
		summary.WriteString(fmt.Sprintf("过滤条件: %s\n", strings.Join(allFilters, "; ")))
	}

	return CompressedMessage{
		Role:         "system",
		Content:      summary.String(),
		IsCompressed: true,
		KeyInfo: &ConversationKeyInfo{
			Tables:  allTables,
			Fields:  allFields,
			Filters: allFilters,
		},
	}
}

// extractKeyInfo 从消息中提取关键信息
func (c *ConversationCompressor) extractKeyInfo(msg ConversationMessage) *ConversationKeyInfo {
	info := &ConversationKeyInfo{}

	if msg.SQL != "" {
		// 从 SQL 中提取表名
		info.Tables = extractTableNamesFromSQL(msg.SQL)

		// 提取字段名（简化版：提取 SELECT 和 WHERE 中的标识符）
		info.Fields = extractFieldNames(msg.SQL)

		// 提取过滤条件
		info.Filters = extractFilterConditions(msg.SQL)

		// 提取 JOIN 类型
		info.JoinTypes = extractJoinTypes(msg.SQL)

		// 提取聚合函数
		info.Aggregations = extractAggregations(msg.SQL)

		// 提取排序字段
		info.OrderBy = extractOrderBy(msg.SQL)

		// 提取 LIMIT
		info.Limit = extractLimit(msg.SQL)
	}

	return info
}

// extractFieldNames 从 SQL 中提取字段名
func extractFieldNames(sql string) []string {
	var fields []string

	// 提取 SELECT 子句中的字段
	selectRe := regexp.MustCompile(`(?i)SELECT\s+(.*?)\s+FROM`)
	if m := selectRe.FindStringSubmatch(sql); len(m) > 1 {
		selectClause := m[1]
		// 简单的字段提取（按逗号分割）
		parts := strings.Split(selectClause, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			// 提取字段名（去掉表别名和 AS）
			if idx := strings.LastIndex(part, "."); idx >= 0 {
				part = part[idx+1:]
			}
			if idx := strings.Index(strings.ToUpper(part), " AS "); idx >= 0 {
				part = part[:idx]
			}
			part = strings.TrimSpace(part)
			if part != "" && part != "*" {
				fields = append(fields, part)
			}
		}
	}

	return uniqueStrings(fields)
}

// extractFilterConditions 从 SQL 中提取过滤条件
func extractFilterConditions(sql string) []string {
	var filters []string

	// 提取 WHERE 子句
	whereRe := regexp.MustCompile(`(?i)WHERE\s+(.*?)(?:GROUP BY|ORDER BY|LIMIT|$)`)
	if m := whereRe.FindStringSubmatch(sql); len(m) > 1 {
		whereClause := m[1]
		// 按 AND/OR 分割条件
		conditions := regexp.MustCompile(`(?i)\s+(AND|OR)\s+`).Split(whereClause, -1)
		for _, cond := range conditions {
			cond = strings.TrimSpace(cond)
			if cond != "" && len(cond) < 100 { // 过滤过长的条件
				filters = append(filters, cond)
			}
		}
	}

	return uniqueStrings(filters)
}

// extractJoinTypes 从 SQL 中提取 JOIN 类型
func extractJoinTypes(sql string) []string {
	var joins []string

	joinRe := regexp.MustCompile(`(?i)(INNER|LEFT|RIGHT|FULL|CROSS)?\s*JOIN`)
	matches := joinRe.FindAllStringSubmatch(sql, -1)
	for _, m := range matches {
		joinType := "JOIN"
		if len(m) > 1 && m[1] != "" {
			joinType = strings.ToUpper(m[1]) + " JOIN"
		}
		joins = append(joins, joinType)
	}

	return uniqueStrings(joins)
}

// extractAggregations 从 SQL 中提取聚合函数
func extractAggregations(sql string) []string {
	var aggs []string

	aggRe := regexp.MustCompile(`(?i)(COUNT|SUM|AVG|MAX|MIN)\s*\(`)
	matches := aggRe.FindAllStringSubmatch(sql, -1)
	for _, m := range matches {
		if len(m) > 1 {
			aggs = append(aggs, strings.ToUpper(m[1]))
		}
	}

	return uniqueStrings(aggs)
}

// extractOrderBy 从 SQL 中提取排序字段
func extractOrderBy(sql string) []string {
	var orders []string

	orderRe := regexp.MustCompile(`(?i)ORDER\s+BY\s+(.*?)(?:LIMIT|$)`)
	if m := orderRe.FindStringSubmatch(sql); len(m) > 1 {
		orderClause := m[1]
		parts := strings.Split(orderClause, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				orders = append(orders, part)
			}
		}
	}

	return uniqueStrings(orders)
}

// extractLimit 从 SQL 中提取 LIMIT 值
func extractLimit(sql string) int {
	limitRe := regexp.MustCompile(`(?i)LIMIT\s+(\d+)`)
	if m := limitRe.FindStringSubmatch(sql); len(m) > 1 {
		var limit int
		fmt.Sscanf(m[1], "%d", &limit)
		return limit
	}
	return 0
}

// uniqueStrings 去重字符串切片
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// FormatCompressedHistory 格式化压缩后的对话历史
func FormatCompressedHistory(compressed []CompressedMessage) string {
	var parts []string

	for _, msg := range compressed {
		if msg.IsCompressed {
			parts = append(parts, msg.Content)
		} else {
			role := "用户"
			if msg.Role == "assistant" {
				role = "助手"
			}
			line := fmt.Sprintf("%s: %s", role, msg.Content)
			if msg.SQL != "" {
				line += fmt.Sprintf("\n[生成的SQL: %s]", msg.SQL)
			}
			parts = append(parts, line)
		}
	}

	return strings.Join(parts, "\n")
}

// CompressConversationContext 压缩对话上下文（集成到 Generator）
func (g *Generator) CompressConversationContext(history []ConversationMessage) string {
	if len(history) == 0 {
		return ""
	}

	compressor := NewConversationCompressor(2000, 10, 3)
	compressed := compressor.CompressHistory(history)

	return FormatCompressedHistory(compressed)
}

// CompressConversationContextWithLLM 使用 LLM 压缩对话上下文（高级版）
func (g *Generator) CompressConversationContextWithLLM(ctx context.Context, history []ConversationMessage) (string, error) {
	if len(history) == 0 {
		return "", nil
	}

	// 如果历史较短，直接格式化
	if len(history) <= 5 {
		return buildConversationContext(history), nil
	}

	// 构建压缩提示词
	var conversationText strings.Builder
	for i, msg := range history {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		conversationText.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, role, msg.Content))
		if msg.SQL != "" {
			conversationText.WriteString(fmt.Sprintf("   SQL: %s\n", msg.SQL))
		}
	}

	prompt := fmt.Sprintf(`请压缩以下对话历史，保留关键信息（涉及的表、字段、过滤条件、查询意图）。

对话历史：
%s

请输出压缩后的摘要（不超过 200 字）：`, conversationText.String())

	messages := []Message{NewTextMessage("user", prompt)}
	resp, err := g.llmProvider.Chat(ctx, "你是对话压缩专家。", messages)
	if err != nil {
		// 回退到规则压缩
		return g.CompressConversationContext(history), nil
	}

	return resp.Content, nil
}
