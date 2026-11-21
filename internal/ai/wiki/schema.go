package wiki

import (
	"fmt"
	"strings"
	"sync"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
)

// Schema 层（对齐 Karpathy LLM Wiki 的 SCHEMA.md）：
// workspace 级"行为约束"声明 —— 命名规则、建页策略、冲突处理等。
//
// purpose 是"往哪里走"，schema 是"怎么运作"：它约束编译器怎么给页面命名、
// 什么情况下该建新页 vs 更新现有页、遇到矛盾时怎么处理。存 aw_config（KV，
// 键按空间区分），不加表不加列；每次页面生成的 system prompt 都会注入它。

const schemaKeyPrefix = "wiki.schema."
const schemaMaxLen = 2000

func schemaKey(spaceID int) string {
	return fmt.Sprintf("%s%d", schemaKeyPrefix, spaceID)
}

// schemaCache schema 在每次页面生成时都要读，但它几乎不变 —— 5 分钟 TTL 缓存，
// 避免同一轮编译产生几十次重复查询。
var schemaCache = struct {
	mu sync.RWMutex
	m  map[int]schemaCacheEntry
}{m: make(map[int]schemaCacheEntry)}

type schemaCacheEntry struct {
	builtAt time.Time
	value   string
}

const schemaCacheTTL = 5 * time.Minute

func invalidateSchemaCache(spaceID int) {
	schemaCache.mu.Lock()
	delete(schemaCache.m, spaceID)
	schemaCache.mu.Unlock()
}

// GetSchema 读取空间行为约束；未设置或读取失败返回空串（调用方按"无 schema"降级）。
func GetSchema(spaceID int) string {
	schemaCache.mu.RLock()
	if e, ok := schemaCache.m[spaceID]; ok && time.Since(e.builtAt) < schemaCacheTTL {
		schemaCache.mu.RUnlock()
		return e.value
	}
	schemaCache.mu.RUnlock()

	v, err := adminrepo.NewConfigRepository().GetValueByKey(schemaKey(spaceID))
	if err != nil {
		v = ""
	}
	schemaCache.mu.Lock()
	schemaCache.m[spaceID] = schemaCacheEntry{builtAt: time.Now(), value: v}
	schemaCache.mu.Unlock()
	return v
}

// SetSchema 写入空间行为约束。空串 = 清除。
func SetSchema(spaceID int, schema string) error {
	schema = strings.TrimSpace(schema)
	if len([]rune(schema)) > schemaMaxLen {
		schema = string([]rune(schema)[:schemaMaxLen])
	}

	repo := adminrepo.NewConfigRepository()
	key := schemaKey(spaceID)
	var err error
	if _, findErr := repo.FindByKey(key); findErr == nil {
		err = repo.UpdateValueByKey(key, schema)
	} else {
		err = repo.Create(&adminmodel.Config{
			Key: key, Value: schema,
			Label: "Wiki 行为约束", GroupKey: "wiki", Type: "textarea",
		})
	}
	if err == nil {
		invalidateSchemaCache(spaceID)
	}
	return err
}

// schemaPromptBlock 把 schema 渲染成可注入 prompt 的段落；无 schema 返回空串。
func schemaPromptBlock(spaceID int) string {
	s := GetSchema(spaceID)
	if s == "" {
		return ""
	}
	return "\n\n本知识库的行为约束（命名规则、建页策略、冲突处理，必须遵守）：\n" + s
}
