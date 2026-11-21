package search

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	modelsearch "github.com/listen808/citebase/internal/model/search"
)

// 全局索引实例
var index bleve.Index

// indexMu 保护 index 的初始化/重建/关闭，防止并发操作同一索引目录
// （Windows 下并发 Open/RemoveAll 会因文件锁互相失败）。
// 用读写锁而非互斥锁：Bleve Index 本身支持 Search 与写入并发，
// 搜索/统计等只读路径走 RLock，避免所有搜索请求在进程内串行化；重建/关闭/写入走 Lock。
var indexMu sync.RWMutex

// indexPath 存储索引路径，用于懒初始化
var savedIndexPath string

// IndexRebuilt 标记索引是否被重建（用于通知调用方需要重新索引数据）
var IndexRebuilt = false

// 预编译 AND 关键字分割正则
var andKeywordRegex = regexp.MustCompile(`[\s&]+`)

// 搜索类型常量
const (
	SearchTypeAll     = "all"     // 全部搜索
	SearchTypeTitle   = "title"   // 仅标题搜索
	SearchTypeContent = "content" // 仅内容搜索
)

// removeIndexDir 带重试地删除索引目录。
// Windows 上索引段文件可能被杀毒扫描/句柄延迟释放短暂锁定，RemoveAll 会暂时失败，
// 静默忽略会导致 bleve.New 在残留目录上失败，形成"索引永远打不开"的死局。
func removeIndexDir(indexPath string) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		if err := os.RemoveAll(indexPath); err == nil {
			return nil
		} else {
			lastErr = err
			logx.Infof("[Search] RemoveAll attempt %d failed: %v, retrying...", i+1, err)
			time.Sleep(300 * time.Millisecond)
		}
	}
	return fmt.Errorf("failed to remove index dir %s: %w", indexPath, lastErr)
}

// closeIndexLocked 关闭并置空全局索引（调用方需持有 indexMu）
func closeIndexLocked() {
	if index != nil {
		if err := index.Close(); err != nil {
			logx.Infof("[Search] index close error: %v", err)
		}
		index = nil
	}
}

// InitIndex 初始化搜索索引
// 注意：本函数不加锁，由调用方（ensureIndex/ResetIndex/ForceInitIndex/启动流程）保证串行。
func InitIndex(indexPath string) error {
	// 确保索引路径是绝对路径，避免相对路径问题
	absIndexPath, err := filepath.Abs(indexPath)
	if err == nil {
		indexPath = absIndexPath
	}

	// 确保索引目录存在
	dir := filepath.Dir(indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create index directory %s: %w", dir, err)
	}

	logx.Infof("[Search] Initializing index at: %s", indexPath)

	// 保存路径用于懒初始化
	savedIndexPath = indexPath

	var bleveErr error
	index, bleveErr = bleve.Open(indexPath)
	if bleveErr == nil {
		// 索引已存在，检查分析器版本是否匹配
		if needsReindex(index) {
			logx.Infof("[Search] Index analyzer outdated, rebuilding with jieba tokenizer...")
			if err := index.Close(); err != nil {
				logx.Infof("[Search] index close error: %v", err)
			}
			index = nil // 防止 Close 后 index 仍被引用
			if err := removeIndexDir(indexPath); err != nil {
				index = nil
				return fmt.Errorf("failed to clean outdated index: %w", err)
			}
			indexMapping := buildIndexMapping()
			index, bleveErr = bleve.New(indexPath, indexMapping)
			if bleveErr != nil {
				index = nil
				return fmt.Errorf("failed to create new index after reindex: %w", bleveErr)
			}
			markAnalyzerVersion(index)
			IndexRebuilt = true
		}
		logx.Infof("[Search] Index opened successfully at: %s", indexPath)
		return nil
	}

	// 索引不存在或损坏，清理后创建新索引
	logx.Infof("[Search] Cannot open index (%v), cleaning up and creating new at: %s", bleveErr, indexPath)
	if err := removeIndexDir(indexPath); err != nil {
		index = nil
		return err
	}
	indexMapping := buildIndexMapping()
	index, bleveErr = bleve.New(indexPath, indexMapping)
	if bleveErr != nil {
		index = nil
		return fmt.Errorf("failed to create index: %w", bleveErr)
	}
	markAnalyzerVersion(index)
	logx.Infof("[Search] New index created successfully at: %s", indexPath)
	return nil
}

// needsReindex 检查现有索引是否需要重建（分析器版本检测）
func needsReindex(idx bleve.Index) bool {
	if idx == nil {
		return true
	}
	docCount, err := idx.DocCount()
	if err != nil {
		logx.Infof("[Search] DocCount error, will reindex: %v", err)
		return true
	}
	if docCount == 0 {
		return false
	}
	markerQuery := bleve.NewTermQuery("jieba_v5")
	markerQuery.SetField("_analyzer")
	markerReq := bleve.NewSearchRequest(markerQuery)
	markerReq.Size = 1
	markerResult, err := idx.Search(markerReq)
	if err != nil || markerResult.Total == 0 {
		logx.Infof("[Search] Analyzer version mismatch (jieba_v5 tokenizer required), will reindex")
		return true
	}
	return false
}

// markAnalyzerVersion 在索引中写入分析器版本标记。
// v4：词典改为「随包分发 + 显式路径加载」，词典缺失时的降级分词也从「整段一个 token」
// 变为「单字 + 二元组」。旧索引里的词项与新分词结果不兼容，必须整体重建才能恢复召回。
func markAnalyzerVersion(idx bleve.Index) {
	marker := map[string]interface{}{
		"_analyzer": "jieba_v5",
	}
	if err := idx.Index("_analyzer_marker", marker); err != nil {
		logx.Infof("[Search] Warning: failed to write analyzer marker: %v", err)
	}
}

func buildIndexMapping() mapping.IndexMapping {
	docMapping := bleve.NewDocumentMapping()
	docMapping.AddFieldMappingsAt("id", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("space_id", bleve.NewNumericFieldMapping())

	indexMapping := bleve.NewIndexMapping()

	analyzerName := "standard"
	if err := indexMapping.AddCustomAnalyzer("jieba", map[string]interface{}{
		"type":          "custom",
		"tokenizer":     JiebaTokenizerName,
		"token_filters": []interface{}{"to_lower"},
	}); err != nil {
		logx.Infof("[Search] Warning: failed to add jieba analyzer: %v, falling back to standard", err)
	} else {
		analyzerName = "jieba"
		logx.Infof("[Search] Using jieba tokenizer for Chinese word segmentation")
	}

	nameFieldMapping := bleve.NewTextFieldMapping()
	nameFieldMapping.Analyzer = analyzerName
	docMapping.AddFieldMappingsAt("name", nameFieldMapping)

	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = analyzerName
	docMapping.AddFieldMappingsAt("content", contentFieldMapping)

	// 拼音字段：索引文本已预切为空格分隔的单字母/音节（见 PinyinIndexText），
	// 主分析器按空格切分即可实现首字母单 token 匹配，无需自定义 analyzer
	pinyinFieldMapping := bleve.NewTextFieldMapping()
	pinyinFieldMapping.Analyzer = analyzerName
	docMapping.AddFieldMappingsAt("name_pinyin", pinyinFieldMapping)

	docMapping.AddFieldMappingsAt("type", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("path", bleve.NewTextFieldMapping())

	indexMapping.DefaultMapping = docMapping

	return indexMapping
}

func GetIndex() bleve.Index {
	return index
}

func CloseIndex() error {
	if index != nil {
		return index.Close()
	}
	return nil
}

// ForceInitIndex 强制初始化索引（API 诊断用）
// 关闭现有索引（如果有），删除并重建
func ForceInitIndex(indexPath string) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	closeIndexLocked()
	if err := removeIndexDir(indexPath); err != nil {
		return err
	}
	return InitIndex(indexPath)
}

// ResetIndex 清空并重建索引
func ResetIndex(indexPath string) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	closeIndexLocked()
	if err := removeIndexDir(indexPath); err != nil {
		return err
	}
	return InitIndex(indexPath)
}

// IndexDocument 索引文档
// 持锁执行：防止写入过程中 index 被 ResetIndex close/替换（曾导致写入数据丢失、索引损坏）
func IndexDocument(docID string, doc *modelsearch.SearchDocument) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	ensureIndexLocked()
	if index == nil {
		return fmt.Errorf("search index is not initialized")
	}
	return index.Index(docID, doc)
}

// IndexDocumentByID 根据 ID + 内容索引
// 持锁执行，理由同 IndexDocument
func IndexDocumentByID(id int, spaceID int, name string, content string, docType int, path string) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	ensureIndexLocked()
	if index == nil {
		return fmt.Errorf("search index is not initialized")
	}
	doc := modelsearch.SearchDocument{
		ID:         id,
		SpaceID:    spaceID,
		Name:       name,
		Content:    content,
		NamePinyin: PinyinIndexText(name),
		Type:       docType,
		Path:       path,
	}
	return index.Index(strconv.Itoa(id), doc)
}

// DeleteDocument 删除索引
// 持锁执行，理由同 IndexDocument
func DeleteDocument(docID string) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	ensureIndexLocked()
	if index == nil {
		return nil
	}
	return index.Delete(docID)
}

// SearchOptions 搜索选项
type SearchOptions struct {
	Query    string // 搜索关键字
	Type     string // 搜索类型: all, title, content
	SpaceID  int    // 空间ID过滤，0表示不限
	SpaceIDs []int  // 多空间过滤（权限前置），nil 表示不限
	Limit    int    // 返回数量限制
	Offset   int    // 偏移量
}

// SearchDocuments 搜索文档（按空间过滤）
func SearchDocuments(searchQuery string, spaceID int, limit int) (*bleve.SearchResult, error) {
	return SearchDocumentsWithOptions(&SearchOptions{
		Query:   searchQuery,
		Type:    SearchTypeAll,
		SpaceID: spaceID,
		Limit:   limit,
	})
}

// SearchDocumentsGlobal 全局搜索（不限空间）
func SearchDocumentsGlobal(searchQuery string, limit int) (*bleve.SearchResult, error) {
	return SearchDocumentsWithOptions(&SearchOptions{
		Query:   searchQuery,
		Type:    SearchTypeAll,
		SpaceID: 0,
		Limit:   limit,
	})
}

// ensureIndex 懒初始化索引（加锁版本）
func ensureIndex() {
	indexMu.Lock()
	defer indexMu.Unlock()
	ensureIndexLocked()
}

// ensureIndexLocked 懒初始化索引（调用方需持有 indexMu）
func ensureIndexLocked() {
	if index != nil {
		return
	}
	if savedIndexPath == "" {
		return
	}
	logx.Infof("[Search] Lazy-initializing index at: %s", savedIndexPath)
	if err := InitIndex(savedIndexPath); err != nil {
		logx.Infof("[Search] Lazy-init failed: %v", err)
	}
}

// SearchDocumentsWithOptions 带选项的搜索
// 只读路径持 RLock：防止搜索过程中 index 被 ResetIndex close（曾返回 "index is closed" 错误），
// 同时允许多个搜索并发执行（旧实现用互斥锁把全部搜索串行化，延迟随并发线性上升）。
func SearchDocumentsWithOptions(opts *SearchOptions) (*bleve.SearchResult, error) {
	indexMu.RLock()
	if index == nil {
		indexMu.RUnlock()
		ensureIndex() // 懒初始化需要写锁，不能在 RLock 下升级
		indexMu.RLock()
	}
	defer indexMu.RUnlock()
	if index == nil {
		return nil, fmt.Errorf("index not initialized")
	}

	if opts.Limit <= 0 {
		opts.Limit = 20
	}

	req := buildAdvancedSearchRequest(opts)
	if req == nil {
		return nil, fmt.Errorf("invalid search query")
	}

	return index.Search(req)
}

// buildAdvancedSearchRequest 构建高级搜索请求
// 支持高级搜索语法：
//   - "A|B|C" 表示 OR 搜索（包含任一关键字）
//   - "A B C" 或 "A&B&C" 表示 AND 搜索（同时包含所有关键字）
//   - "-keyword" 表示排除（不包含该关键字）
//   - "title:keyword" 限定标题搜索
//   - "content:keyword" 限定内容搜索
func buildAdvancedSearchRequest(opts *SearchOptions) *bleve.SearchRequest {
	if opts == nil || opts.Query == "" {
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Size = opts.Limit
		return req
	}

	trimmed := strings.TrimSpace(opts.Query)
	if trimmed == "" {
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Size = opts.Limit
		return req
	}

	// 解析高级语法：排除词和字段限定
	parsed := parseAdvancedQuery(trimmed)

	var finalQuery query.Query

	// 根据解析后的关键字构建查询
	if parsed.HasOR {
		finalQuery = buildORQueryWithField(parsed.Include, parsed.AndGroups, parsed.FieldType)
	} else if len(parsed.AndGroups) > 0 || len(parsed.Include) > 1 {
		allAnd := append([]string{}, parsed.Include...)
		allAnd = append(allAnd, parsed.AndGroups...)
		finalQuery = buildANDQuery(allAnd, parsed.FieldType)
	} else if len(parsed.Include) == 1 {
		finalQuery = buildSingleQuery(parsed.Include[0], parsed.FieldType)
	} else {
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Size = opts.Limit
		return req
	}

	if finalQuery == nil {
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Size = opts.Limit
		return req
	}

	// 添加排除查询
	if len(parsed.Exclude) > 0 {
		excludeQueries := make([]query.Query, 0, len(parsed.Exclude))
		for _, ex := range parsed.Exclude {
			excludeQueries = append(excludeQueries, buildFieldQuery(ex, parsed.FieldType))
		}
		if len(excludeQueries) > 0 {
			excludeUnion := bleve.NewDisjunctionQuery(excludeQueries...)
			boolQuery := bleve.NewBooleanQuery()
			boolQuery.AddMust(finalQuery)
			boolQuery.AddMustNot(excludeUnion)
			finalQuery = boolQuery
		}
	}

	// 构建空间过滤
	// 注意：不能使用 bleve.NewNumericRangeQuery(&v,&v)——其 max 默认 exclusive，
	// 等值过滤 [v,v) 是空区间恒 0 命中，曾导致所有带空间过滤的搜索静默返回空。
	if opts.SpaceID > 0 {
		minVal := float64(opts.SpaceID)
		maxVal := float64(opts.SpaceID)
		minInc := true
		maxInc := true
		spaceQuery := bleve.NewNumericRangeInclusiveQuery(&minVal, &maxVal, &minInc, &maxInc)
		spaceQuery.SetField("space_id")
		finalQuery = bleve.NewConjunctionQuery(finalQuery, spaceQuery)
	} else if len(opts.SpaceIDs) > 0 {
		// 多空间过滤（权限前置）：构造 DisjunctionQuery 注入 Bleve 查询，避免搜后过滤
		spaceQueries := make([]query.Query, 0, len(opts.SpaceIDs))
		for _, sid := range opts.SpaceIDs {
			if sid <= 0 {
				continue
			}
			minVal := float64(sid)
			maxVal := float64(sid)
			minInc := true
			maxInc := true
			sq := bleve.NewNumericRangeInclusiveQuery(&minVal, &maxVal, &minInc, &maxInc)
			sq.SetField("space_id")
			spaceQueries = append(spaceQueries, sq)
		}
		if len(spaceQueries) > 0 {
			spaceFilter := bleve.NewDisjunctionQuery(spaceQueries...)
			finalQuery = bleve.NewConjunctionQuery(finalQuery, spaceFilter)
		}
	}

	req := bleve.NewSearchRequest(finalQuery)
	req.Highlight = bleve.NewHighlight()
	req.Highlight.AddField("name")
	req.Highlight.AddField("content")
	req.IncludeLocations = true
	req.Fields = []string{"id", "space_id", "name", "type", "path"}
	req.Size = opts.Limit
	if opts.Offset > 0 {
		req.From = opts.Offset
	}

	return req
}

// parsedQuery 解析后的查询结构
type parsedQuery struct {
	Include   []string
	Exclude   []string
	AndGroups []string
	FieldType string // "all", "title", "content" - 从语法解析的字段限定
	HasOR     bool
}

// parseAdvancedQuery 解析高级搜索语法
func parseAdvancedQuery(query string) parsedQuery {
	result := parsedQuery{
		Include:   make([]string, 0),
		Exclude:   make([]string, 0),
		AndGroups: make([]string, 0),
		FieldType: SearchTypeAll,
		HasOR:     false,
	}

	// 检测 OR 语法
	result.HasOR = strings.Contains(query, "|")

	// 分词处理（按空格、|、& 分割，但保留引号内的内容）
	parts := tokenizeQuery(query)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "|" {
			continue
		}

		// 排除语法: -keyword
		if strings.HasPrefix(part, "-") && len(part) > 1 {
			exclude := strings.TrimPrefix(part, "-")
			// 检查字段限定
			exclude, _ = parseFieldQualifier(exclude, &result)
			if exclude != "" {
				result.Exclude = append(result.Exclude, exclude)
			}
			continue
		}

		// 字段限定语法: title:keyword, content:keyword
		clean, fieldType := parseFieldQualifier(part, &result)
		if fieldType != SearchTypeAll {
			result.FieldType = fieldType
		}

		if clean != "" {
			// 检查是否包含 AND 连接符
			if strings.Contains(clean, "&") {
				subParts := strings.Split(clean, "&")
				for _, sp := range subParts {
					sp = strings.TrimSpace(sp)
					if sp != "" {
						result.AndGroups = append(result.AndGroups, sp)
					}
				}
			} else {
				result.Include = append(result.Include, clean)
			}
		}
	}

	return result
}

// parseFieldQualifier 解析字段限定符 title: / content:
// 返回清理后的关键字和字段类型
func parseFieldQualifier(keyword string, result *parsedQuery) (string, string) {
	if strings.HasPrefix(keyword, "title:") && len(keyword) > 6 {
		return strings.TrimPrefix(keyword, "title:"), SearchTypeTitle
	}
	if strings.HasPrefix(keyword, "content:") && len(keyword) > 8 {
		return strings.TrimPrefix(keyword, "content:"), SearchTypeContent
	}
	return keyword, SearchTypeAll
}

// tokenizeQuery 分词查询字符串，保留 OR 和 AND 语义
func tokenizeQuery(query string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			current.WriteByte(ch)
			continue
		}
		if ch == '|' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, "|")
		} else if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// buildORQueryWithField 构建带字段限定的 OR 查询
func buildORQueryWithField(keywords []string, andGroups []string, fieldType string) query.Query {
	var queries []query.Query

	for _, kw := range keywords {
		queries = append(queries, buildFieldQuery(kw, fieldType))
	}

	for _, andKw := range andGroups {
		andQueries := buildANDQuery([]string{andKw}, fieldType)
		if andQueries != nil {
			queries = append(queries, andQueries)
		}
	}

	if len(queries) == 0 {
		return nil
	}
	if len(queries) == 1 {
		return queries[0]
	}

	return bleve.NewDisjunctionQuery(queries...)
}

// parseAndKeywords 解析 AND 关键字
func parseAndKeywords(input string) []string {
	// 用预编译的正则分割
	parts := andKeywordRegex.Split(input, -1)
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// buildSingleQuery 构建单关键字查询
func buildSingleQuery(keyword string, searchType string) query.Query {
	return buildFieldQuery(keyword, searchType)
}

// buildANDQuery 构建 AND 查询
// 使用 MatchQuery + ConjunctionQuery：要求每个关键字都出现，但不要求连续
// 修复：原实现用 MatchPhraseQuery 要求词项连续出现，导致"知识 管理"匹配不到"知识库的管理"
func buildANDQuery(keywords []string, searchType string) query.Query {
	if len(keywords) == 0 {
		return nil
	}

	var queries []query.Query
	for _, kw := range keywords {
		// AND 查询使用 MatchQuery（分词匹配），仅要求关键字出现而非连续出现
		queries = append(queries, buildFieldQuery(kw, searchType))
	}

	if len(queries) == 0 {
		return nil
	}
	if len(queries) == 1 {
		return queries[0]
	}

	return bleve.NewConjunctionQuery(queries...)
}

// buildPhraseFieldQuery 构建短语匹配查询（不分词）
// 用于 AND 查询，确保关键词作为整体匹配
func buildPhraseFieldQuery(keyword string, searchType string) query.Query {
	escapedKeyword := escapeSpecialChars(keyword)

	switch searchType {
	case SearchTypeTitle:
		titleQuery := bleve.NewMatchPhraseQuery(escapedKeyword)
		titleQuery.SetField("name")
		titleQuery.SetBoost(2.0)

		pinyinQuery := bleve.NewMatchPhraseQuery(escapedKeyword)
		pinyinQuery.SetField("name_pinyin")
		pinyinQuery.SetBoost(0.5)

		return bleve.NewDisjunctionQuery(titleQuery, pinyinQuery)

	case SearchTypeContent:
		contentQuery := bleve.NewMatchPhraseQuery(escapedKeyword)
		contentQuery.SetField("content")
		return contentQuery

	default:
		nameQuery := bleve.NewMatchPhraseQuery(escapedKeyword)
		nameQuery.SetField("name")
		nameQuery.SetBoost(2.0)

		pinyinQuery := bleve.NewMatchPhraseQuery(escapedKeyword)
		pinyinQuery.SetField("name_pinyin")
		pinyinQuery.SetBoost(0.5)

		contentQuery := bleve.NewMatchPhraseQuery(escapedKeyword)
		contentQuery.SetField("content")

		return bleve.NewDisjunctionQuery(nameQuery, pinyinQuery, contentQuery)
	}
}

// buildFieldQuery 根据搜索类型构建字段查询
// 对 ASCII 短查询追加 PrefixQuery（低 boost），支持前缀匹配如"wik"→"wiki"
func buildFieldQuery(keyword string, searchType string) query.Query {
	escapedKeyword := escapeSpecialChars(keyword)

	// 判断是否为 ASCII 短查询（启用前缀匹配）
	enablePrefix := shouldEnablePrefix(escapedKeyword)

	switch searchType {
	case SearchTypeTitle:
		titleQuery := bleve.NewMatchQuery(escapedKeyword)
		titleQuery.SetField("name")
		titleQuery.SetBoost(2.0)

		pinyinQuery := bleve.NewMatchQuery(escapedKeyword)
		pinyinQuery.SetField("name_pinyin")
		pinyinQuery.SetBoost(0.5)

		queries := []query.Query{titleQuery, pinyinQuery}
		if enablePrefix {
			pq := bleve.NewPrefixQuery(strings.ToLower(escapedKeyword))
			pq.SetField("name")
			pq.SetBoost(1.0)
			queries = append(queries, pq)
		}
		return bleve.NewDisjunctionQuery(queries...)

	case SearchTypeContent:
		contentQuery := bleve.NewMatchQuery(escapedKeyword)
		contentQuery.SetField("content")
		if enablePrefix {
			pq := bleve.NewPrefixQuery(strings.ToLower(escapedKeyword))
			pq.SetField("content")
			pq.SetBoost(0.3)
			return bleve.NewDisjunctionQuery(contentQuery, pq)
		}
		return contentQuery

	default:
		nameQuery := bleve.NewMatchQuery(escapedKeyword)
		nameQuery.SetField("name")
		nameQuery.SetBoost(2.0)

		pinyinQuery := bleve.NewMatchQuery(escapedKeyword)
		pinyinQuery.SetField("name_pinyin")
		pinyinQuery.SetBoost(0.5)

		contentQuery := bleve.NewMatchQuery(escapedKeyword)
		contentQuery.SetField("content")

		queries := []query.Query{nameQuery, pinyinQuery, contentQuery}
		if enablePrefix {
			npq := bleve.NewPrefixQuery(strings.ToLower(escapedKeyword))
			npq.SetField("name")
			npq.SetBoost(1.0)
			cpq := bleve.NewPrefixQuery(strings.ToLower(escapedKeyword))
			cpq.SetField("content")
			cpq.SetBoost(0.3)
			queries = append(queries, npq, cpq)
		}
		return bleve.NewDisjunctionQuery(queries...)
	}
}

// shouldEnablePrefix 判断是否启用前缀匹配
// 仅对纯 ASCII 字母/数字、长度 2-6 的查询启用（避免中文和长查询的性能开销）
func shouldEnablePrefix(keyword string) bool {
	if len(keyword) < 2 || len(keyword) > 6 {
		return false
	}
	for _, r := range keyword {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// escapeSpecialChars 清理搜索关键词中的特殊字符
// Bleve MatchQuery 会对关键词进行分词处理，不需要手动转义 Lucene 语法字符
// 但需要移除或替换可能导致问题的控制字符
func escapeSpecialChars(s string) string {
	// 移除控制字符
	result := strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1 // 移除控制字符
		}
		return r
	}, s)
	return strings.TrimSpace(result)
}

// ReindexAll 重新索引所有文档
func ReindexAll(docs []modelsearch.SearchDocument) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	ensureIndexLocked()
	if index == nil {
		return fmt.Errorf("index not initialized")
	}
	batch := index.NewBatch()
	for _, doc := range docs {
		_ = batch.Index(strconv.Itoa(doc.ID), doc)
	}
	return index.Batch(batch)
}

// BatchIndex 批量索引文档
func BatchIndex(documents []modelsearch.IndexableDocument) error {
	indexMu.Lock()
	defer indexMu.Unlock()
	if index == nil {
		return nil
	}
	batch := index.NewBatch()
	for _, d := range documents {
		doc := modelsearch.SearchDocument{
			ID:         d.ID,
			SpaceID:    d.SpaceID,
			Name:       d.Name,
			Content:    d.Content,
			NamePinyin: PinyinIndexText(d.Name),
			Type:       d.Type,
			Path:       d.Path,
		}
		_ = batch.Index(strconv.Itoa(d.ID), doc)
	}
	return index.Batch(batch)
}

func ToSearchDocument(id, spaceID int, name, content string, docType int, path string) modelsearch.SearchDocument {
	return modelsearch.SearchDocument{
		ID:         id,
		SpaceID:    spaceID,
		Name:       name,
		Content:    content,
		NamePinyin: PinyinIndexText(name),
		Type:       docType,
		Path:       path,
	}
}

// GetIndexStats 只读统计：走 RLock，避免在持写锁期间再阻塞写入路径
func GetIndexStats() (map[string]interface{}, error) {
	indexMu.RLock()
	defer indexMu.RUnlock()
	if index == nil {
		return map[string]interface{}{
			"is_open":   false,
			"doc_count": int64(0),
		}, nil
	}
	docCount, err := index.DocCount()
	if err != nil {
		logx.Infof("[Search] GetIndexStats error: %v", err)
		return map[string]interface{}{
			"is_open":   false,
			"doc_count": int64(0),
			"error":     err.Error(),
		}, nil
	}
	// DocCount() 包含内部 analyzer marker 文档，需要排除
	actualCount := int64(docCount)
	if actualCount > 0 {
		markerQuery := bleve.NewTermQuery("jieba_v5")
		markerQuery.SetField("_analyzer")
		markerReq := bleve.NewSearchRequest(markerQuery)
		markerReq.Size = 0
		if markerResult, err := index.Search(markerReq); err == nil && markerResult.Total > 0 {
			actualCount--
		}
	}
	return map[string]interface{}{
		"is_open":   true,
		"doc_count": actualCount,
	}, nil
}

// IndexAppearsCorrupt 索引完整性自检
// 场景：索引 doc_count>0 但全量查询返回 0，说明段数据损坏（并发重建竞态曾导致此问题）。
// 返回 true 时调用方应强制重建索引，避免沿用损坏索引。
func IndexAppearsCorrupt() bool {
	stats, err := GetIndexStats()
	if err != nil {
		return true
	}
	dc, ok := stats["doc_count"].(int64)
	if !ok || dc == 0 {
		return false
	}
	res, err := SearchDocumentsWithOptions(&SearchOptions{Query: "", Limit: 1})
	if err != nil {
		return true
	}
	return res.Total == 0
}
