package wiki

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContentHash 源内容哈希，用于增量编译：未变更的源直接跳过重新生成，
// 省掉一次模型调用（对应 nashsu/llm_wiki 的 SHA256 增量缓存）。
func ContentHash(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
