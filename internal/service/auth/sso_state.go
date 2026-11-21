package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// ssoStateTTL 授权跳转 state 的有效期。回调必须在有效期内完成，且一次性消费。
const ssoStateTTL = 10 * time.Minute

type ssoStateEntry struct {
	ssoID     int
	createdAt time.Time
}

var (
	ssoStates   = make(map[string]ssoStateEntry)
	ssoStatesMu sync.Mutex
)

// IssueSSOState 生成并登记一次性 state，绑定到指定 SSO 配置。
func IssueSSOState(ssoID int) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	state := hex.EncodeToString(buf)

	ssoStatesMu.Lock()
	defer ssoStatesMu.Unlock()
	pruneSSOStatesLocked()
	// 容量兜底：极端情况下（大量未完成跳转）丢弃最旧的一半，避免无界增长
	if len(ssoStates) > 10000 {
		for k := range ssoStates {
			delete(ssoStates, k)
			if len(ssoStates) <= 5000 {
				break
			}
		}
	}
	ssoStates[state] = ssoStateEntry{ssoID: ssoID, createdAt: time.Now()}
	return state, nil
}

// ConsumeSSOState 校验并消费 state：必须存在、未过期、且属于同一 SSO 配置。
// 返回 false 表示应拒绝本次回调（伪造、重放或已过期）。
func ConsumeSSOState(state string, ssoID int) bool {
	if state == "" {
		return false
	}
	ssoStatesMu.Lock()
	defer ssoStatesMu.Unlock()
	entry, ok := ssoStates[state]
	if !ok {
		return false
	}
	// 一次性：无论后续校验是否通过都消费掉，防止重放
	delete(ssoStates, state)
	if time.Since(entry.createdAt) > ssoStateTTL {
		return false
	}
	return entry.ssoID == ssoID
}

func pruneSSOStatesLocked() {
	cutoff := time.Now().Add(-ssoStateTTL)
	for k, v := range ssoStates {
		if v.createdAt.Before(cutoff) {
			delete(ssoStates, k)
		}
	}
}
