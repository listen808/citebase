package text2sql

import (
	"strconv"
	"sync"
	"time"
)

// SchemaCachePool Schema 缓存池，避免每轮重复检索
// P0-3: 所有方法对 map/slice 的访问通过 mu 互斥保护，支持并发调用
// P1(w7): Tables/FieldIndex 以 (spaceID, 表名) 复合 key 分片。旧实现只用裸表名做 key，
// 全局唯一 pool 会把不同空间同名表的字段互相覆盖，并把跨空间表名/字段塞进 SQL 模板。
type SchemaCachePool struct {
	ConversationID int
	SpaceID        int
	Tables         map[string]*CachedTable    // 复合 key -> 缓存
	Relations      []SchemaRelation           // 已发现的表关系
	FieldIndex     map[string]map[string]bool // 复合 key -> 字段集合
	LastAccessed   time.Time

	mu sync.RWMutex
}

// 缓存必须可淘汰：旧实现无 TTL/容量上限（LastAccessed 写而不读），常驻进程单调增长。
const (
	schemaCacheTTL      = 15 * time.Minute
	schemaCacheMaxTable = 500
)

// schemaCacheKey 复合分片键：同名表在不同空间互不覆盖、互不可见。
func schemaCacheKey(spaceID int, tableName string) string {
	return strconv.Itoa(spaceID) + "|" + tableName
}

// tableRowSpaceID 取表缓存行的空间归属；无空间（全局）记录落到 0 分片。
func tableRowSpaceID(spaceID *uint64) int {
	if spaceID != nil {
		return int(*spaceID)
	}
	return 0
}

// CachedTable 缓存的表信息
type CachedTable struct {
	Name      string
	Columns   []ColumnInfo
	Samples   []string // 字段样本值（辅助语义匹配）
	Source    string   // "retrieved", "user_confirmed"
	Confirmed bool

	spaceID    int       // 所属空间（分片归属，便于调试与过滤）
	accessedAt time.Time // 最近访问时间（写锁内更新），供 TTL/容量淘汰使用
}

// SchemaRelation 表关系
type SchemaRelation struct {
	SourceTable  string
	SourceField  string
	TargetTable  string
	TargetField  string
	RelationType string // "fk", "inferred"
}

// NewSchemaCachePool 创建 Schema 缓存池
func NewSchemaCachePool(convID, spaceID int) *SchemaCachePool {
	return &SchemaCachePool{
		ConversationID: convID,
		SpaceID:        spaceID,
		Tables:         make(map[string]*CachedTable),
		Relations:      make([]SchemaRelation, 0),
		FieldIndex:     make(map[string]map[string]bool),
		LastAccessed:   time.Now(),
	}
}

// CacheTable 缓存表结构。spaceID 为分片键：调用方按表所属空间写入，读取按请求空间过滤，
// 保证同名表跨空间不互相覆盖/泄露。
func (p *SchemaCachePool) CacheTable(spaceID int, tableName string, columns []ColumnInfo, confirmed bool) {
	source := "retrieved"
	if confirmed {
		source = "user_confirmed"
	}
	key := schemaCacheKey(spaceID, tableName)
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Tables[key] = &CachedTable{
		Name:       tableName,
		Columns:    columns,
		Source:     source,
		Confirmed:  confirmed,
		spaceID:    spaceID,
		accessedAt: now,
	}

	// 更新字段索引
	fieldSet := make(map[string]bool)
	for _, col := range columns {
		fieldSet[col.Name] = true
	}
	p.FieldIndex[key] = fieldSet
	p.LastAccessed = now
	p.evictLocked()
}

// evictLocked 按 TTL 清理过期条目，并在超过容量上限时逐出最久未访问的条目（须持有写锁）。
func (p *SchemaCachePool) evictLocked() {
	now := time.Now()
	for k, t := range p.Tables {
		if now.Sub(t.accessedAt) > schemaCacheTTL {
			delete(p.Tables, k)
			delete(p.FieldIndex, k)
		}
	}
	for len(p.Tables) > schemaCacheMaxTable {
		var oldestKey string
		var oldest time.Time
		for k, t := range p.Tables {
			if oldestKey == "" || t.accessedAt.Before(oldest) {
				oldestKey, oldest = k, t.accessedAt
			}
		}
		delete(p.Tables, oldestKey)
		delete(p.FieldIndex, oldestKey)
	}
}

// GetCachedTable 获取缓存的表
func (p *SchemaCachePool) GetCachedTable(spaceID int, tableName string) (*CachedTable, bool) {
	key := schemaCacheKey(spaceID, tableName)
	// 需要更新 accessedAt，必须用写锁：旧实现在 RLock 下写 LastAccessed，本身就是数据竞争
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ok := p.Tables[key]
	if ok {
		now := time.Now()
		t.accessedAt = now
		p.LastAccessed = now
	}
	return t, ok
}

// HasField 检查字段是否存在于缓存中
func (p *SchemaCachePool) HasField(spaceID int, tableName, fieldName string) bool {
	key := schemaCacheKey(spaceID, tableName)
	p.mu.RLock()
	defer p.mu.RUnlock()
	fieldSet, ok := p.FieldIndex[key]
	if !ok {
		return false
	}
	return fieldSet[fieldName]
}

// ConfirmTable 标记表为已确认（用户修正过的信息）
func (p *SchemaCachePool) ConfirmTable(spaceID int, tableName string) {
	key := schemaCacheKey(spaceID, tableName)
	p.mu.Lock()
	defer p.mu.Unlock()
	if t, ok := p.Tables[key]; ok {
		t.Confirmed = true
		t.Source = "user_confirmed"
		t.accessedAt = time.Now()
	}
}

// GetMissingFields 获取缓存中缺失的字段
func (p *SchemaCachePool) GetMissingFields(spaceID int, tableName string, fields []string) []string {
	key := schemaCacheKey(spaceID, tableName)
	p.mu.RLock()
	defer p.mu.RUnlock()
	fieldSet, ok := p.FieldIndex[key]
	if !ok {
		// 表未缓存：全部视为缺失
		missing := make([]string, len(fields))
		copy(missing, fields)
		return missing
	}
	var missing []string
	for _, f := range fields {
		if !fieldSet[f] {
			missing = append(missing, f)
		}
	}
	return missing
}

// GetCachedTables 获取指定空间分片内所有缓存的表名
func (p *SchemaCachePool) GetCachedTables(spaceID int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.Tables))
	for _, t := range p.Tables {
		// 只返回本空间分片的表：跨空间表名不得进入本空间的模板生成上下文
		if t.spaceID == spaceID {
			names = append(names, t.Name)
		}
	}
	return names
}

// IsTableConfirmed 检查表是否已被用户确认
func (p *SchemaCachePool) IsTableConfirmed(spaceID int, tableName string) bool {
	key := schemaCacheKey(spaceID, tableName)
	p.mu.RLock()
	defer p.mu.RUnlock()
	if t, ok := p.Tables[key]; ok {
		return t.Confirmed
	}
	return false
}

// AddRelation 添加表关系
func (p *SchemaCachePool) AddRelation(rel SchemaRelation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 去重
	for _, r := range p.Relations {
		if r.SourceTable == rel.SourceTable && r.TargetTable == rel.TargetTable &&
			r.SourceField == rel.SourceField && r.TargetField == rel.TargetField {
			return
		}
	}
	p.Relations = append(p.Relations, rel)
}
