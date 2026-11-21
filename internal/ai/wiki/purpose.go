package wiki

import (
	"fmt"
	"strings"
	"sync"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
)

// Purpose 层（对齐 Karpathy LLM Wiki 的 purpose.md）：
// workspace 级"研究目标 / 关键问题 / 研究范围"声明。
//
// schema 是"怎么运作"，purpose 是"往哪里走"。存 aw_config（KV，键按空间区分），
// 不加表不加列；每次 ingest 的页面生成与查询期的目录选页都会读它作为上下文。

const purposeKeyPrefix = "wiki.purpose."
const purposeMaxLen = 2000

func purposeKey(spaceID int) string {
	return fmt.Sprintf("%s%d", purposeKeyPrefix, spaceID)
}

// purposeCache purpose 在每次页面生成与检索选页时都要读，
// 但它几乎不变 —— 5 分钟 TTL 缓存，避免同一轮编译产生几十次重复查询。
var purposeCache = struct {
	mu sync.RWMutex
	m  map[int]purposeCacheEntry
}{m: make(map[int]purposeCacheEntry)}

type purposeCacheEntry struct {
	builtAt time.Time
	value   string
}

const purposeCacheTTL = 5 * time.Minute

func invalidatePurposeCache(spaceID int) {
	purposeCache.mu.Lock()
	delete(purposeCache.m, spaceID)
	purposeCache.mu.Unlock()
}

// GetPurpose 读取空间研究目标；未设置或读取失败返回空串（调用方按"无 purpose"降级）。
func GetPurpose(spaceID int) string {
	purposeCache.mu.RLock()
	if e, ok := purposeCache.m[spaceID]; ok && time.Since(e.builtAt) < purposeCacheTTL {
		purposeCache.mu.RUnlock()
		return e.value
	}
	purposeCache.mu.RUnlock()

	v, err := adminrepo.NewConfigRepository().GetValueByKey(purposeKey(spaceID))
	if err != nil {
		v = ""
	}
	purposeCache.mu.Lock()
	purposeCache.m[spaceID] = purposeCacheEntry{builtAt: time.Now(), value: v}
	purposeCache.mu.Unlock()
	return v
}

// SetPurpose 写入空间研究目标。空串 = 清除。
func SetPurpose(spaceID int, purpose string) error {
	purpose = strings.TrimSpace(purpose)
	if len([]rune(purpose)) > purposeMaxLen {
		purpose = string([]rune(purpose)[:purposeMaxLen])
	}

	repo := adminrepo.NewConfigRepository()
	key := purposeKey(spaceID)
	var err error
	if _, findErr := repo.FindByKey(key); findErr == nil {
		err = repo.UpdateValueByKey(key, purpose)
	} else {
		err = repo.Create(&adminmodel.Config{
			Key: key, Value: purpose,
			Label: "Wiki 研究目标", GroupKey: "wiki", Type: "textarea",
		})
	}
	if err == nil {
		invalidatePurposeCache(spaceID)
	}
	return err
}

// purposePromptBlock 把 purpose 渲染成可注入 prompt 的段落；无 purpose 返回空串。
func purposePromptBlock(spaceID int) string {
	p := GetPurpose(spaceID)
	if p == "" {
		return ""
	}
	return "\n\n本知识库的研究目标（写作/选页时必须服务于它）：\n" + p
}
