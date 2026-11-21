package text2sql

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ConversationState 对话状态机，维护完整对话上下文
type ConversationState struct {
	ConversationID int
	SpaceID        int

	// 累积的 Schema 信息
	UsedTables map[string]*TableSchemaInfo // 表名 -> 结构
	UsedFields map[string][]string         // 表名 -> 字段列表
	UsedJoins  []JoinRelation              // 已建立的 JOIN 关系

	// 累积的 SQL 结构
	SQLHistory  []SQLTurn // 每轮的 SQL 及修改
	CurrentSQL  string    // 当前最新 SQL
	currentTurn int       // 当前轮次计数

	// 用户意图追踪
	IntentHistory []UserIntent      // 每轮的用户意图
	ActiveFilters []FilterCondition // 当前生效的筛选条件

	// 解析的上一轮SQL结构（用于多轮修改模式）
	LastParsedSQL *ParsedSQLStructure
}

// ParsedSQLStructure 解析后的SQL结构，用于多轮修改时精确指导LLM
type ParsedSQLStructure struct {
	SelectClause string   // SELECT 子句
	FromClause   string   // FROM 子句
	JoinClauses  []string // JOIN 子句列表
	WhereClause  string   // WHERE 子句
	GroupClause  string   // GROUP BY 子句
	OrderClause  string   // ORDER BY 子句
	LimitClause  string   // LIMIT 子句
	RawSQL       string   // 原始SQL
}

// TableSchemaInfo 缓存的表结构信息
type TableSchemaInfo struct {
	Name      string
	Columns   []ColumnInfo
	Confirmed bool // 用户是否确认过
}

// ColumnInfo 列信息
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Comment  string
}

// JoinRelation JOIN 关系
type JoinRelation struct {
	LeftTable  string
	LeftField  string
	RightTable string
	RightField string
	JoinType   string // "INNER", "LEFT", "RIGHT"
}

// SQLTurn 单轮对话的 SQL 记录
type SQLTurn struct {
	TurnID        int
	UserQuery     string
	GeneratedSQL  string
	Modifications []SQLModification
	Timestamp     time.Time
}

// SQLModification SQL 修改记录
type SQLModification struct {
	Type     string // "add_filter", "change_order", "add_join", "change_field", "add_limit"
	Target   string // 修改目标
	OldValue string
	NewValue string
}

// UserIntent 用户意图
type UserIntent struct {
	TurnID     int
	Query      string
	IntentType string   // "new_query", "modify", "follow_up", "clarify"
	Entities   []string // 提取的实体
}

// FilterCondition 筛选条件
type FilterCondition struct {
	Field    string
	Operator string // "=", ">", "<", "LIKE", "IN", "BETWEEN"
	Value    string
	Source   string // 来自哪一轮（如 "turn:1"）
}

// NewConversationState 创建对话状态机
func NewConversationState(convID, spaceID int) *ConversationState {
	return &ConversationState{
		ConversationID: convID,
		SpaceID:        spaceID,
		UsedTables:     make(map[string]*TableSchemaInfo),
		UsedFields:     make(map[string][]string),
		UsedJoins:      make([]JoinRelation, 0),
		SQLHistory:     make([]SQLTurn, 0),
		IntentHistory:  make([]UserIntent, 0),
		ActiveFilters:  make([]FilterCondition, 0),
	}
}

// RecordTurn 记录一轮对话
func (cs *ConversationState) RecordTurn(query, sql string, mods []SQLModification) {
	cs.currentTurn++
	turn := SQLTurn{
		TurnID:        cs.currentTurn,
		UserQuery:     query,
		GeneratedSQL:  sql,
		Modifications: mods,
		Timestamp:     time.Now(),
	}
	cs.SQLHistory = append(cs.SQLHistory, turn)
	cs.CurrentSQL = sql
}

// RecordTableUsage 记录使用的表
func (cs *ConversationState) RecordTableUsage(tableName string, columns []ColumnInfo) {
	if _, exists := cs.UsedTables[tableName]; !exists {
		cs.UsedTables[tableName] = &TableSchemaInfo{
			Name:    tableName,
			Columns: columns,
		}
	}
	fieldNames := make([]string, len(columns))
	for i, col := range columns {
		fieldNames[i] = col.Name
	}
	cs.UsedFields[tableName] = fieldNames
}

// RecordJoin 记录 JOIN 关系
func (cs *ConversationState) RecordJoin(join JoinRelation) {
	for _, j := range cs.UsedJoins {
		if j.LeftTable == join.LeftTable && j.RightTable == join.RightTable &&
			j.LeftField == join.LeftField && j.RightField == join.RightField {
			return
		}
	}
	cs.UsedJoins = append(cs.UsedJoins, join)
}

// RecordIntent 记录用户意图
func (cs *ConversationState) RecordIntent(intent UserIntent) {
	cs.IntentHistory = append(cs.IntentHistory, intent)
}

// GetTurnCount 获取轮次数
func (cs *ConversationState) GetTurnCount() int {
	return len(cs.SQLHistory)
}

// GetLastTurn 获取最后一轮
func (cs *ConversationState) GetLastTurn() *SQLTurn {
	if len(cs.SQLHistory) == 0 {
		return nil
	}
	return &cs.SQLHistory[len(cs.SQLHistory)-1]
}

// DetectIntent 检测当前轮的用户意图
func (cs *ConversationState) DetectIntent(query string) UserIntent {
	intent := UserIntent{
		TurnID: cs.currentTurn + 1,
		Query:  query,
	}

	queryLower := strings.ToLower(query)

	// 检测引用词 → follow_up
	referenceWords := []string{"刚才", "上一轮", "那个", "这个", "上面的", "之前的", "刚才那个", "前一轮"}
	for _, word := range referenceWords {
		if strings.Contains(queryLower, word) {
			intent.IntentType = "follow_up"
			cs.RecordIntent(intent)
			return intent
		}
	}

	// 检测修改指示词 → modify
	modifyWords := []string{"改成", "改为", "修改", "换成", "变成", "把", "将", "不要", "去掉", "删除"}
	for _, word := range modifyWords {
		if strings.Contains(queryLower, word) {
			intent.IntentType = "modify"
			cs.RecordIntent(intent)
			return intent
		}
	}

	// 检测补充说明 → clarify
	clarifyWords := []string{"我说的是", "我的意思是", "补充", "其实", "准确来说"}
	for _, word := range clarifyWords {
		if strings.Contains(queryLower, word) {
			intent.IntentType = "clarify"
			cs.RecordIntent(intent)
			return intent
		}
	}

	// 省略主语的追问（无明确主语 + 有动作词）→ follow_up
	// 增强：如果对话历史存在且查询很短（<20字）或无表名提及，也判定为 follow_up
	actionWords := []string{"排序", "筛选", "限制", "分组", "统计", "加上", "增加", "去掉"}
	hasAction := false
	for _, word := range actionWords {
		if strings.Contains(queryLower, word) {
			hasAction = true
			break
		}
	}
	if hasAction && len(cs.SQLHistory) > 0 {
		// 如果查询很短（<20字），大概率是追问
		if len([]rune(query)) < 20 {
			intent.IntentType = "follow_up"
			cs.RecordIntent(intent)
			return intent
		}
		// 如果没有引入新主题（没有提到新的表名），也判定为 follow_up
		if !cs.hasNewTopic(query) {
			intent.IntentType = "follow_up"
			cs.RecordIntent(intent)
			return intent
		}
	}

	// 默认 new_query
	intent.IntentType = "new_query"
	cs.RecordIntent(intent)
	return intent
}

// hasNewTopic 判断查询是否引入了新主题（即是否提到了之前未使用的新表）
func (cs *ConversationState) hasNewTopic(query string) bool {
	// 如果对话历史为空，说明是新对话，不算引入新主题
	if len(cs.SQLHistory) == 0 {
		return false
	}

	queryLower := strings.ToLower(query)

	// 检查是否提到了**已使用**的表名 → 延续当前话题
	for tableName := range cs.UsedTables {
		if strings.Contains(queryLower, strings.ToLower(tableName)) {
			return false
		}
	}

	// 没有提到任何已使用的表，但查询中包含中文"表"字 → 可能引入新主题
	if strings.Contains(query, "表") {
		return true
	}

	// 检测下划线分隔的英文标识符（如 user_orders, profile_data）→ 可能引入新表名
	for _, w := range strings.Fields(queryLower) {
		if len(w) > 3 && strings.Contains(w, "_") {
			if _, used := cs.UsedTables[w]; !used {
				return true
			}
		}
	}

	// 否则视为延续当前话题（如纯动作性追问"按价格排序"）
	return false
}

// ResolveReference 解析查询中的引用词，补全上下文
// 修复：增强引用词检测，包括"现有""当前""已有的"等隐性引用
func (cs *ConversationState) ResolveReference(query string) string {
	if len(cs.SQLHistory) == 0 {
		return query
	}

	lastTurn := cs.GetLastTurn()
	if lastTurn == nil {
		return query
	}

	queryLower := strings.ToLower(query)

	// 显性引用词
	referenceWords := []string{"刚才", "上一轮", "那个", "这个", "上面的", "之前的", "刚才那个", "前一轮"}
	// 隐性引用词（用户可能不说"上一轮"，而是说"现有""当前""已有的"）
	implicitReferenceWords := []string{"现有", "当前", "已有的", "已生成", "上一个", "保留", "保持", "在刚才", "沿用", "继续用", "用刚才"}

	allReferenceWords := append(referenceWords, implicitReferenceWords...)

	hasReference := false
	for _, word := range allReferenceWords {
		if strings.Contains(queryLower, word) {
			hasReference = true
			break
		}
	}

	if !hasReference {
		return query
	}

	// 解析上一轮SQL结构，供后续精确修改
	cs.LastParsedSQL = parseSQLStructure(lastTurn.GeneratedSQL)

	context := lastTurn.GeneratedSQL
	return query + "\n\n[上一轮SQL]: " + context + "\n\n[重要提示] 基于上述SQL进行修改，不要改变不需要修改的部分。"
}

// GetContextForLLM 生成供 LLM 使用的完整对话上下文
// 增强版：当用户意图是修改/跟进时，明确标注SQL各部分（保留/修改）
func (cs *ConversationState) GetContextForLLM(intentType string) string {
	if len(cs.SQLHistory) == 0 {
		return ""
	}

	var sb strings.Builder

	// 如果是修改或跟进意图，生成修改模式的上下文
	if intentType == "follow_up" || intentType == "modify" {
		lastTurn := cs.GetLastTurn()
		if lastTurn != nil && lastTurn.GeneratedSQL != "" {
			// 解析上一轮SQL结构
			parsed := parseSQLStructure(lastTurn.GeneratedSQL)
			cs.LastParsedSQL = parsed

			sb.WriteString("## 修改模式指令\n\n")
			sb.WriteString("用户正在基于上一轮SQL进行修改。请遵循以下规则：\n")
			sb.WriteString("1. **严格基于以下SQL框架**，只修改用户明确要求修改的部分\n")
			sb.WriteString("2. 用户没有提到的部分（如FROM、JOIN、WHERE），**必须完全保留原样**\n")
			sb.WriteString("3. 不要添加额外的JOIN条件、不要修改已有的JOIN类型\n")
			sb.WriteString("4. 如果用户要求\"列出所有数据\"或\"所有字段\"，则SELECT部分使用 `*` 或列出所有表的所有字段\n")
			sb.WriteString("5. 如果用户要求\"保持现有where\"，则WHERE子句完全保留原样\n\n")

			sb.WriteString("### 上一轮SQL框架（各部分独立标注）\n\n")
			if parsed.SelectClause != "" {
				sb.WriteString(fmt.Sprintf("**SELECT**（当前）: %s\n", parsed.SelectClause))
			}
			if parsed.FromClause != "" {
				sb.WriteString(fmt.Sprintf("**FROM**（保留）: %s\n", parsed.FromClause))
			}
			for _, join := range parsed.JoinClauses {
				sb.WriteString(fmt.Sprintf("**JOIN**（保留）: %s\n", join))
			}
			if parsed.WhereClause != "" {
				sb.WriteString(fmt.Sprintf("**WHERE**（保留）: %s\n", parsed.WhereClause))
			}
			if parsed.GroupClause != "" {
				sb.WriteString(fmt.Sprintf("**GROUP BY**（保留）: %s\n", parsed.GroupClause))
			}
			if parsed.OrderClause != "" {
				sb.WriteString(fmt.Sprintf("**ORDER BY**（保留）: %s\n", parsed.OrderClause))
			}
			if parsed.LimitClause != "" {
				sb.WriteString(fmt.Sprintf("**LIMIT**（保留）: %s\n", parsed.LimitClause))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("## 对话历史\n\n")

	for _, turn := range cs.SQLHistory {
		sb.WriteString(fmt.Sprintf("### 第%d轮\n", turn.TurnID))
		sb.WriteString(fmt.Sprintf("用户: %s\n", turn.UserQuery))
		if turn.GeneratedSQL != "" {
			sb.WriteString(fmt.Sprintf("生成SQL: %s\n\n", turn.GeneratedSQL))
		} else {
			sb.WriteString("生成SQL: （未生成）\n\n")
		}
	}

	if len(cs.UsedTables) > 0 {
		sb.WriteString("## 已使用的表\n")
		for name := range cs.UsedTables {
			sb.WriteString(fmt.Sprintf("- %s\n", name))
		}
		sb.WriteString("\n")
	}

	if len(cs.ActiveFilters) > 0 {
		sb.WriteString("## 当前筛选条件\n")
		for _, f := range cs.ActiveFilters {
			sb.WriteString(fmt.Sprintf("- %s %s %s\n", f.Field, f.Operator, f.Value))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// parseSQLStructure 解析SQL字符串，提取各部分子句
// 使用正则表达式进行简单解析，支持标准SELECT语句
func parseSQLStructure(sql string) *ParsedSQLStructure {
	result := &ParsedSQLStructure{RawSQL: sql}
	if sql == "" {
		return result
	}

	upper := strings.ToUpper(sql)

	// 提取SELECT
	if idx := strings.Index(upper, "SELECT"); idx != -1 {
		rest := sql[idx+6:]
		upperRest := strings.ToUpper(rest)
		// 找到下一个关键词（FROM、WHERE等）
		nextIdx := len(rest)
		for _, keyword := range []string{" FROM", "\tFROM", "\nFROM", " WHERE", "\tWHERE", "\nWHERE", " GROUP BY", " ORDER BY", " LIMIT", " HAVING"} {
			if kidx := strings.Index(upperRest, keyword); kidx != -1 && kidx < nextIdx {
				nextIdx = kidx
			}
		}
		result.SelectClause = strings.TrimSpace(rest[:nextIdx])
	}

	// 提取FROM
	if idx := strings.Index(upper, " FROM "); idx != -1 {
		rest := sql[idx+6:]
		upperRest := strings.ToUpper(rest)
		// 找到JOIN或WHERE
		nextIdx := len(rest)
		for _, keyword := range []string{" JOIN", " WHERE", "\tWHERE", "\nWHERE", " GROUP BY", " ORDER BY", " LIMIT"} {
			if kidx := strings.Index(upperRest, keyword); kidx != -1 && kidx < nextIdx {
				nextIdx = kidx
			}
		}
		result.FromClause = strings.TrimSpace(rest[:nextIdx])
	}

	// 提取JOIN子句（支持多个JOIN）
	joinRe := regexp.MustCompile(`(?i)(LEFT\s+JOIN|RIGHT\s+JOIN|INNER\s+JOIN|OUTER\s+JOIN|JOIN)\s+([\w_]+\s+[\w_]+\s+ON\s+.+?)(?:\s+(?:LEFT\s+JOIN|RIGHT\s+JOIN|INNER\s+JOIN|OUTER\s+JOIN|JOIN|WHERE|GROUP\s+BY|ORDER\s+BY|LIMIT|HAVING)|$)`)
	matches := joinRe.FindAllStringSubmatch(sql, -1)
	for _, m := range matches {
		if len(m) > 0 {
			joinSQL := strings.TrimSpace(m[0])
			// 去掉开头的JOIN类型前缀，统一加上JOIN
			joinSQL = regexp.MustCompile(`(?i)^(?:LEFT\s+|RIGHT\s+|INNER\s+|OUTER\s+)?JOIN\s*`).ReplaceAllString(joinSQL, "")
			result.JoinClauses = append(result.JoinClauses, joinSQL)
		}
	}

	// 如果正则没匹配到，使用简单解析
	if len(result.JoinClauses) == 0 {
		// 手动扫描JOIN
		start := 0
		for {
			if jidx := strings.Index(upper[start:], " JOIN "); jidx != -1 {
				actualIdx := start + jidx + 1 // +1 指向J
				rest := sql[actualIdx:]
				upperRest := strings.ToUpper(rest)
				nextIdx := len(rest)
				for _, keyword := range []string{" LEFT JOIN", " RIGHT JOIN", " INNER JOIN", " OUTER JOIN", " JOIN", " WHERE", " GROUP BY", " ORDER BY", " LIMIT"} {
					if kidx := strings.Index(upperRest, keyword); kidx != -1 && kidx > 0 && kidx < nextIdx {
						nextIdx = kidx
					}
				}
				joinClause := strings.TrimSpace(rest[:nextIdx])
				result.JoinClauses = append(result.JoinClauses, joinClause)
				start = actualIdx + nextIdx
			} else {
				break
			}
		}
	}

	// 提取WHERE
	if idx := strings.Index(upper, " WHERE "); idx != -1 {
		rest := sql[idx+7:]
		upperRest := strings.ToUpper(rest)
		nextIdx := len(rest)
		for _, keyword := range []string{" GROUP BY", " ORDER BY", " LIMIT", " HAVING"} {
			if kidx := strings.Index(upperRest, keyword); kidx != -1 && kidx < nextIdx {
				nextIdx = kidx
			}
		}
		result.WhereClause = strings.TrimSpace(rest[:nextIdx])
	}

	// 提取GROUP BY
	if idx := strings.Index(upper, " GROUP BY "); idx != -1 {
		rest := sql[idx+10:]
		upperRest := strings.ToUpper(rest)
		nextIdx := len(rest)
		for _, keyword := range []string{" ORDER BY", " LIMIT", " HAVING"} {
			if kidx := strings.Index(upperRest, keyword); kidx != -1 && kidx < nextIdx {
				nextIdx = kidx
			}
		}
		result.GroupClause = strings.TrimSpace(rest[:nextIdx])
	}

	// 提取ORDER BY
	if idx := strings.Index(upper, " ORDER BY "); idx != -1 {
		rest := sql[idx+10:]
		upperRest := strings.ToUpper(rest)
		nextIdx := len(rest)
		for _, keyword := range []string{" LIMIT"} {
			if kidx := strings.Index(upperRest, keyword); kidx != -1 && kidx < nextIdx {
				nextIdx = kidx
			}
		}
		result.OrderClause = strings.TrimSpace(rest[:nextIdx])
	}

	// 提取LIMIT
	if idx := strings.Index(upper, " LIMIT "); idx != -1 {
		result.LimitClause = strings.TrimSpace(sql[idx+7:])
	}

	return result
}

// BuildModificationContext 根据用户查询和上一轮SQL，构建精确的修改指令
// 识别用户想修改的部分（如"保持现有where"=保留WHERE，"列出所有数据"=修改SELECT）
func (cs *ConversationState) BuildModificationContext(query string) string {
	if cs.LastParsedSQL == nil || cs.LastParsedSQL.RawSQL == "" {
		return ""
	}

	queryLower := strings.ToLower(query)

	var keepParts []string
	var modifyParts []string

	// 识别保留部分
	keepKeywords := map[string][]string{
		"SELECT": {"select", "select部分", "select子句", "select字段", "选中的字段", "选中的列", "现有select", "已有select"},
		"FROM":   {"from", "from部分", "from子句", "表来源", "现有from", "已有from"},
		"JOIN":   {"join", "join条件", "关联", "关联条件", "表关联", "现有join", "已有join"},
		"WHERE":  {"where", "where条件", "where子句", "筛选条件", "过滤条件", "现有where", "已有where", "保持where", "保留where"},
		"GROUP":  {"group by", "groupby", "分组", "分组条件", "现有group", "已有group"},
		"ORDER":  {"order by", "orderby", "排序", "排序条件", "现有order", "已有order"},
		"LIMIT":  {"limit", "limit子句", "限制条数", "top", "前几条", "现有limit", "已有limit"},
	}

	modifyKeywords := map[string][]string{
		"SELECT": {"列出所有数据", "列出所有", "所有字段", "所有数据", "全部字段", "全部数据", "select所有", "select *", "列出所有列", "所有列"},
		"FROM":   {"换个表", "换表", "from换成"},
		"JOIN":   {"换个关联", "换join", "改关联"},
		"WHERE":  {"改条件", "换条件", "换个条件", "where改成", "取消条件", "去掉条件", "删除条件"},
		"GROUP":  {"换个分组", "改分组"},
		"ORDER":  {"换个排序", "改排序"},
		"LIMIT":  {"改条数", "换条数"},
	}

	for part, keywords := range keepKeywords {
		for _, kw := range keywords {
			if strings.Contains(queryLower, kw) {
				keepParts = append(keepParts, part)
				break
			}
		}
	}

	for part, keywords := range modifyKeywords {
		for _, kw := range keywords {
			if strings.Contains(queryLower, kw) {
				modifyParts = append(modifyParts, part)
				break
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("## 用户修改意图分析\n\n")

	if len(keepParts) > 0 {
		sb.WriteString(fmt.Sprintf("**明确要求保留的部分**: %s\n", strings.Join(keepParts, ", ")))
	}
	if len(modifyParts) > 0 {
		sb.WriteString(fmt.Sprintf("**明确要求修改的部分**: %s\n", strings.Join(modifyParts, ", ")))
	}

	// 特殊处理：如果用户说"列出所有数据"但没有明确保留SELECT，说明要修改SELECT
	if strings.Contains(queryLower, "所有数据") || strings.Contains(queryLower, "所有字段") || strings.Contains(queryLower, "全部数据") || strings.Contains(queryLower, "全部字段") || strings.Contains(queryLower, "所有列") || strings.Contains(queryLower, "列出所有") {
		if !strSliceContains(keepParts, "SELECT") {
			modifyParts = append(modifyParts, "SELECT")
			sb.WriteString("\n**推断修改**: SELECT部分 → 改为列出所有字段（SELECT * 或列出所有表的所有字段）\n")
		}
	}

	// 如果用户明确说了"保持"某个部分，但没说保持其他部分，默认其他部分也保留
	if len(keepParts) > 0 && len(modifyParts) == 0 {
		sb.WriteString("\n**推断**: 用户只要求保留部分子句，未明确要求修改的部分也建议保留。\n")
	}

	// 如果用户完全没提任何保留或修改，只是追问（如"列出所有数据"），推断为修改SELECT
	if len(keepParts) == 0 && len(modifyParts) == 0 {
		if strings.Contains(queryLower, "所有") || strings.Contains(queryLower, "全部") || strings.Contains(queryLower, "列") || strings.Contains(queryLower, "字段") || strings.Contains(queryLower, "列出") || strings.Contains(queryLower, "显示") || strings.Contains(queryLower, "展示") {
			sb.WriteString("\n**推断修改**: SELECT部分 → 列出所有字段\n")
		}
	}

	sb.WriteString("\n**修改规则**:\n")
	sb.WriteString("1. 保留的部分必须完全沿用原SQL的语法和逻辑，一字不差\n")
	sb.WriteString("2. 修改的部分必须基于原SQL的上下文进行合理调整\n")
	sb.WriteString("3. 未明确提到的部分，优先保持原样\n")
	sb.WriteString("4. 如果修改SELECT为\"所有数据\"，请使用 SELECT * 或列出所有表的完整字段列表\n")
	sb.WriteString("5. 最终SQL必须包含上一轮SQL的所有部分，除非用户明确要求删除某部分\n")

	return sb.String()
}

func strSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
