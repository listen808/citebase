package text2sql

import (
	"fmt"
	"strings"
	"sync"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// knowledgeRepo 是 kb 用到的那五个仓储方法。
// 提成接口的理由很具体：KnowledgeBase 现在要能被单测钉住（并发刷新 + 规则翻倍），
// 而按具体类型 *airepo.TextToSQLRepo 写字段就只能连库测 —— 那等于这两条断言写不出来。
type knowledgeRepo interface {
	ListRules(page, pageSize int, ruleType, tableName string) ([]modelai.BusinessRule, int64, error)
	UpsertTableCache(cache *modelai.TableCache) error
	GetAllTableCaches() ([]modelai.TableCache, error)
	GetRelationsBySpace(spaceID uint64) ([]modelai.TableRelation, error)
	GetTableCachesBySpace(spaceID uint64) ([]modelai.TableCache, error)
}

// KnowledgeBase 是三张 map 的容器，而这两种生命周期同时存在：
//   - Generator 初始化时建一份**长期共享**的 kb（g.kb），请求路径上只读它
//     （generator.go 的 tryTemplateGenerate / generateWithLLM 一路传 g.kb，
//     agent_tools.go 通过 GetKnowledgeBase()/request_scope.go 也拿它）；
//   - POST /ai/data-query/refresh 会**整张换掉** g.kb.tables 并往 g.kb.rules 里追加。
//
// 没有锁的时候这两件事并发就是 `fatal error: concurrent map read and map write`
// —— 那是 runtime 级 fatal，recover() 接不住，整个进程直接没了。
//
// 约定：导出的方法自己上锁，**非导出的 load* / merge* 系列假设调用方已经持锁**
// （它们总是成串地被导出方法调用，逐个上锁会变成重入）。
// 读侧一律返回**拷贝**（GetAllTables）或"发布后不再改动"的切片，
// 所以下面的写侧全部是"先构造新 map，再一次性替换"，不对已发布的 map 原地 append。
type KnowledgeBase struct {
	repo       knowledgeRepo
	parser     *DDLParser
	schemaRepo SchemaRepoInterface

	mu     sync.RWMutex
	tables map[string]*modelai.TableCache
	rules  map[string][]modelai.BusinessRule
	rels   map[string][]modelai.TableRelation
}

func NewKnowledgeBase(repo knowledgeRepo) *KnowledgeBase {
	return &KnowledgeBase{
		repo:   repo,
		parser: NewDDLParser(),
		tables: make(map[string]*modelai.TableCache),
		rules:  make(map[string][]modelai.BusinessRule),
		rels:   make(map[string][]modelai.TableRelation),
	}
}

// SetSchemaRepo 设置 Schema 仓库（统一数据源：aw_schema_*）。
// 它写在共享实例上、被 load 系列读，所以和 map 用同一把锁 —— 不是"顺手加的"。
func (kb *KnowledgeBase) SetSchemaRepo(repo SchemaRepoInterface) {
	kb.mu.Lock()
	kb.schemaRepo = repo
	kb.mu.Unlock()
}

// LoadFromSpace 从 aw_schema_* 加载指定空间的表结构（统一数据源）
// 回退：如果 schemaRepo 不可用，从 aw_ai_table_cache 加载（兼容旧数据）
func (kb *KnowledgeBase) LoadFromSpace(spaceID uint64) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()

	kb.tables = make(map[string]*modelai.TableCache)
	kb.rels = make(map[string][]modelai.TableRelation)

	// 优先从 Schema 管理模块加载（aw_schema_table + aw_schema_column + aw_schema_relation）
	if kb.schemaRepo != nil {
		schemaSpace, err := kb.schemaRepo.FindSpaceByWikiSpaceID(int(spaceID))
		if err != nil {
			allSpaces, _, spaceErr := kb.schemaRepo.ListSpaces(1, 100)
			if spaceErr == nil && len(allSpaces) > 0 {
				schemaSpace = allSpaces[0]
			}
		}
		if schemaSpace != nil {
			kb.loadFromSchemaSpace(schemaSpace.ID)
			// 同时合并 aw_ai_table_cache 中可能存在的补充数据（过渡期）
			kb.mergeFromLegacyCache(spaceID)
			return nil
		}
	}

	// 回退：从 aw_ai_table_cache 加载（过渡期兼容）
	return kb.loadFromLegacyCache(spaceID)
}

// loadFromSchemaSpace 从 aw_schema_table/column/relation 加载数据
func (kb *KnowledgeBase) loadFromSchemaSpace(schemaSpaceID int) {
	if kb.schemaRepo == nil {
		return
	}
	tables, err := kb.schemaRepo.ListAllTables(schemaSpaceID)
	if err != nil {
		return
	}
	for _, t := range tables {
		cleanName := cleanTableName(t.Name)
		columns, colErr := kb.schemaRepo.ListColumnsByTableName(schemaSpaceID, t.Name)
		if colErr != nil {
			continue
		}
		colInfos := make(modelai.ColumnInfoList, 0, len(columns))
		for _, col := range columns {
			colInfos = append(colInfos, modelai.ColumnInfo{
				Name:       cleanColumnName(col.ColumnName),
				Type:       col.DataType,
				Comment:    col.ColumnComment,
				IsPK:       col.IsPrimaryKey == 1,
				IsNullable: col.Nullable == 1,
			})
		}
		cache := &modelai.TableCache{
			TableNam:     cleanName,
			TableComment: t.TableComment,
			DDLContent:   t.RawDDL,
			ColumnsJSON:  colInfos,
		}
		kb.tables[cleanName] = cache
	}

	// 加载表关系
	relations, relErr := kb.schemaRepo.ListRelations(schemaSpaceID)
	if relErr == nil {
		for _, rel := range relations {
			srcTable := cleanTableName(rel.SourceTable)
			kb.rels[srcTable] = append(kb.rels[srcTable], modelai.TableRelation{
				SourceTable:  srcTable,
				SourceField:  cleanColumnName(rel.SourceColumn),
				TargetTable:  cleanTableName(rel.TargetTable),
				TargetField:  cleanColumnName(rel.TargetColumn),
				RelationType: rel.RelationType,
			})
		}
	}
}

// loadFromLegacyCache 从 aw_ai_table_cache 加载（过渡期回退）
func (kb *KnowledgeBase) loadFromLegacyCache(spaceID uint64) error {
	caches, err := kb.repo.GetTableCachesBySpace(spaceID)
	if err != nil {
		return fmt.Errorf("load table caches by space: %w", err)
	}
	for i := range caches {
		kb.tables[caches[i].TableNam] = &caches[i]
	}
	relations, err := kb.repo.GetRelationsBySpace(spaceID)
	if err != nil {
		return fmt.Errorf("load relations by space: %w", err)
	}
	for _, rel := range relations {
		kb.rels[rel.SourceTable] = append(kb.rels[rel.SourceTable], rel)
	}
	return nil
}

// mergeFromLegacyCache 合并 aw_ai_table_cache 中当前空间未覆盖的表（过渡期）
func (kb *KnowledgeBase) mergeFromLegacyCache(spaceID uint64) {
	caches, err := kb.repo.GetAllTableCaches()
	if err != nil {
		return
	}
	for i := range caches {
		if _, exists := kb.tables[caches[i].TableNam]; !exists {
			kb.tables[caches[i].TableNam] = &caches[i]
		}
	}
}

func (kb *KnowledgeBase) LoadFromDDL(ddlContents []string) error {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	for _, ddl := range ddlContents {
		cache, err := kb.parser.ParseDDL(ddl)
		if err != nil {
			continue
		}
		if cache.TableNam != "" {
			kb.tables[cache.TableNam] = cache
		}
	}

	caches := make([]modelai.TableCache, 0, len(kb.tables))
	for _, c := range kb.tables {
		caches = append(caches, *c)
	}
	relations := kb.parser.InferRelations(caches)
	for _, rel := range relations {
		kb.rels[rel.SourceTable] = append(kb.rels[rel.SourceTable], rel)
	}

	return nil
}

// LoadRules 重新装载业务规则。
// 必须**整张换掉**而不是往现有 map 里 append：LoadRules 会被每次生成
// （generate_core.go 的 prepareKB、generator.go 三处）与 RefreshKnowledge 调用，
// 而 append 的目标是上一轮留下的切片 —— 于是同一批规则每刷一次就多叠一份，
// 并全部拼进 prompt。实测跑满刷新次数后规则数按轮次线性增长。
func (kb *KnowledgeBase) LoadRules() error {
	rules, _, err := kb.repo.ListRules(1, 10000, "", "")
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}

	fresh := make(map[string][]modelai.BusinessRule, len(rules))
	for _, rule := range rules {
		fresh[rule.TableNam] = append(fresh[rule.TableNam], rule)
	}

	kb.mu.Lock()
	kb.rules = fresh
	kb.mu.Unlock()
	return nil
}

// ReloadTablesFromCaches 用给定缓存**整张替换**表结构（RefreshKnowledge 的写侧入口）。
// 之前调用方是直接 `g.kb.tables = make(...)` 再逐条塞 —— 那就是并发 map 崩溃的写侧。
func (kb *KnowledgeBase) ReloadTablesFromCaches(caches []modelai.TableCache) {
	replacement := make(map[string]*modelai.TableCache, len(caches))
	for i := range caches {
		replacement[caches[i].TableNam] = &caches[i]
	}
	kb.mu.Lock()
	kb.tables = replacement
	kb.mu.Unlock()
}

// tableCachesForRefresh 把仓储读挪进 kb，免得调用方为了取缓存而直接抓 kb.repo。
func (kb *KnowledgeBase) tableCachesForRefresh() ([]modelai.TableCache, error) {
	return kb.repo.GetAllTableCaches()
}

func (kb *KnowledgeBase) SearchTables(keyword string) []*modelai.TableCache {
	keyword = strings.ToLower(keyword)

	kb.mu.RLock()
	defer kb.mu.RUnlock()
	var results []*modelai.TableCache
	for name, cache := range kb.tables {
		if strings.Contains(strings.ToLower(name), keyword) {
			results = append(results, cache)
			continue
		}
		if strings.Contains(strings.ToLower(cache.TableComment), keyword) {
			results = append(results, cache)
			continue
		}
		for _, col := range cache.ColumnsJSON {
			if strings.Contains(strings.ToLower(col.Comment), keyword) {
				results = append(results, cache)
				break
			}
		}
	}

	return results
}

func (kb *KnowledgeBase) GetTable(name string) *modelai.TableCache {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	return kb.tables[name]
}

// GetRules 返回切片副本：调用方会把它拼进 prompt 甚至排序，
// 而写侧此刻可能正在 append —— 交出底层数组就等于把锁让出去。
func (kb *KnowledgeBase) GetRules(tableName string) []modelai.BusinessRule {
	kb.mu.RLock()
	src := kb.rules[tableName]
	kb.mu.RUnlock()
	if len(src) == 0 {
		return nil
	}
	out := make([]modelai.BusinessRule, len(src))
	copy(out, src)
	return out
}

func (kb *KnowledgeBase) GetRelations(tableName string) []modelai.TableRelation {
	kb.mu.RLock()
	src := kb.rels[tableName]
	kb.mu.RUnlock()
	if len(src) == 0 {
		return nil
	}
	out := make([]modelai.TableRelation, len(src))
	copy(out, src)
	return out
}

// GetAllTables 返回浅拷贝。原先直接把内部 map 交出去，
// 而 agent_tools.go 那类调用方会 `for range` 它好几轮 —— 期间 RefreshKnowledge 就能换掉整张。
func (kb *KnowledgeBase) GetAllTables() map[string]*modelai.TableCache {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	out := make(map[string]*modelai.TableCache, len(kb.tables))
	for name, cache := range kb.tables {
		out[name] = cache
	}
	return out
}

// GetAllRelations 获取所有表关系
func (kb *KnowledgeBase) GetAllRelations() []modelai.TableRelation {
	kb.mu.RLock()
	defer kb.mu.RUnlock()
	var allRelations []modelai.TableRelation
	for _, rels := range kb.rels {
		allRelations = append(allRelations, rels...)
	}
	return allRelations
}

func (kb *KnowledgeBase) SaveToDB() error {
	kb.mu.RLock()
	pending := make([]*modelai.TableCache, 0, len(kb.tables))
	for _, cache := range kb.tables {
		pending = append(pending, cache)
	}
	kb.mu.RUnlock()

	for _, cache := range pending {
		if err := kb.repo.UpsertTableCache(cache); err != nil {
			return fmt.Errorf("save table %s: %w", cache.TableNam, err)
		}
	}

	return nil
}

// LoadAll 加载所有 Schema 空间的表（优先 Schema，回退 Cache）
func (kb *KnowledgeBase) LoadAll() error {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	// 优先从 Schema 管理模块加载所有空间
	if kb.schemaRepo != nil {
		allSpaces, _, err := kb.schemaRepo.ListSpaces(1, 1000)
		if err == nil {
			for _, s := range allSpaces {
				kb.loadFromSchemaSpace(s.ID)
			}
			if len(kb.tables) > 0 {
				kb.mergeFromLegacyCache(0) // 补充 aw_ai_table_cache 中无空间归属的表
				return nil
			}
		}
	}
	// 回退
	return kb.loadFromLegacyCache(0)
}

// MergeAllTables 合并所有空间的表（优先 Schema，回退 Cache）
func (kb *KnowledgeBase) MergeAllTables() error {
	kb.mu.Lock()
	defer kb.mu.Unlock()
	// 优先从 Schema 管理模块合并
	if kb.schemaRepo != nil {
		allSpaces, _, err := kb.schemaRepo.ListSpaces(1, 1000)
		if err == nil {
			for _, s := range allSpaces {
				kb.loadFromSchemaSpace(s.ID)
			}
		}
	}
	// 同时合并 aw_ai_table_cache 中未覆盖的表（过渡期）
	kb.mergeFromLegacyCache(0)
	return nil
}
