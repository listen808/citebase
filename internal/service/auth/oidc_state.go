package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// oidcStateTTL 授权跳转参数有效期。
const oidcStateTTL = 10 * time.Minute

type oidcStateEntry struct {
	providerID  uint
	redirectURI string
	nonce       string
	createdAt   time.Time
}

var (
	oidcStates   = make(map[string]oidcStateEntry)
	oidcStatesMu sync.Mutex
)

// IssueOIDCState 服务端签发一次性 state + nonce（防 CSRF、防授权码注入、支持 ID Token nonce 校验）。
func IssueOIDCState(providerID uint, redirectURI string) (state, nonce string, err error) {
	stateBytes := make([]byte, 16)
	if _, err = rand.Read(stateBytes); err != nil {
		return "", "", err
	}
	nonceBytes := make([]byte, 16)
	if _, err = rand.Read(nonceBytes); err != nil {
		return "", "", err
	}
	state = hex.EncodeToString(stateBytes)
	nonce = hex.EncodeToString(nonceBytes)

	oidcStatesMu.Lock()
	defer oidcStatesMu.Unlock()
	pruneOIDCStatesLocked()
	if len(oidcStates) > 10000 {
		for k := range oidcStates {
			delete(oidcStates, k)
			if len(oidcStates) <= 5000 {
				break
			}
		}
	}
	oidcStates[state] = oidcStateEntry{providerID: providerID, redirectURI: redirectURI, nonce: nonce, createdAt: time.Now()}
	return state, nonce, nil
}

// ConsumeOIDCState 校验并消费 state；必须存在、未过期且属于同一 provider。
func ConsumeOIDCState(state string, providerID uint) (oidcStateEntry, bool) {
	if state == "" {
		return oidcStateEntry{}, false
	}
	oidcStatesMu.Lock()
	defer oidcStatesMu.Unlock()
	entry, ok := oidcStates[state]
	if !ok {
		return oidcStateEntry{}, false
	}
	delete(oidcStates, state) // 一次性消费，防重放
	if time.Since(entry.createdAt) > oidcStateTTL {
		return oidcStateEntry{}, false
	}
	if entry.providerID != providerID {
		return oidcStateEntry{}, false
	}
	return entry, true
}

func pruneOIDCStatesLocked() {
	cutoff := time.Now().Add(-oidcStateTTL)
	for k, v := range oidcStates {
		if v.createdAt.Before(cutoff) {
			delete(oidcStates, k)
		}
	}
}

// RedirectURI 返回发起授权时登记的回调地址。
func (e oidcStateEntry) RedirectURI() string { return e.redirectURI }

// Nonce 返回发起授权时签发的 nonce。
func (e oidcStateEntry) Nonce() string { return e.nonce }
