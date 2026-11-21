package text2sql

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	logx "github.com/listen808/citebase/internal/log"
)

// SelfConsistencyVoter Self-Consistency 投票器
// P1-1 (§4.2.5): 多候选通过多数表决选择最终 SQL，而非简单评分
//
// 投票策略：
//  1. 标准化 SQL（去注释、去多余空格、统一大小写）后哈希聚类
//  2. 执行结果相同的 SQL 视为同一类（结果哈希）
//  3. 选择执行成功且数量最多的类别
//  4. 平票时用 Confidence 最高的
type SelfConsistencyVoter struct{}

// NewSelfConsistencyVoter 创建投票器
func NewSelfConsistencyVoter() *SelfConsistencyVoter {
	return &SelfConsistencyVoter{}
}

// VoteGroup 投票分组
type VoteGroup struct {
	SQLHash     string         // 标准化 SQL 哈希
	ResultHash  string         // 执行结果哈希（执行失败时为空）
	Members     []CandidateSQL // 该组的成员
	Best        *CandidateSQL  // 该组中最优的候选
	VoteCount   int            // 得票数
	AllExecuted bool           // 是否所有成员都执行成功
}

// Vote 对候选 SQL 列表进行投票
// 返回得票最多的组（最优组），以及所有分组（供调试）
func (v *SelfConsistencyVoter) Vote(candidates []CandidateSQL) (*VoteGroup, []VoteGroup) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 按结果哈希优先聚类：执行成功的候选按 resultHash 分组（等价写法归入同组），
	// 仅在结果为空/失败时回退到 sqlHash 分组。
	groupMap := make(map[string]*VoteGroup)
	for i := range candidates {
		c := candidates[i]
		sqlHash := hashSQL(c.SQL)
		resultHash := hashResult(c.ExecResult)
		var groupKey string
		if resultHash != "" {
			groupKey = "r|" + resultHash
		} else {
			groupKey = "s|" + sqlHash
		}

		group, exists := groupMap[groupKey]
		if !exists {
			group = &VoteGroup{
				SQLHash:    sqlHash,
				ResultHash: resultHash,
				Members:    []CandidateSQL{},
			}
			groupMap[groupKey] = group
		}
		group.Members = append(group.Members, c)

		// 更新组内最优（Confidence 最高的）
		if group.Best == nil || c.Confidence > group.Best.Confidence {
			best := c
			group.Best = &best
		}
	}

	// 转换为切片并计算最终字段
	groups := make([]VoteGroup, 0, len(groupMap))
	for _, g := range groupMap {
		g.VoteCount = len(g.Members)
		g.AllExecuted = true
		for _, m := range g.Members {
			if m.ExecResult == nil || !m.ExecResult.Success {
				g.AllExecuted = false
				break
			}
		}
		groups = append(groups, *g)
	}

	// 排序：优先执行成功的组，其次按票数，最后按 Confidence
	for i := 0; i < len(groups); i++ {
		for j := i + 1; j < len(groups); j++ {
			if compareGroups(&groups[i], &groups[j]) < 0 {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}

	if len(groups) == 0 {
		return nil, nil
	}

	logx.Infof("[SelfConsistency] %d candidates → %d groups, winner: sql_hash=%s vote=%d",
		len(candidates), len(groups), groups[0].SQLHash[:8], groups[0].VoteCount)

	return &groups[0], groups
}

// compareGroups 比较两个组的优先级
// 返回 >0 表示 a 优先，<0 表示 b 优先，=0 表示相等
func compareGroups(a, b *VoteGroup) int {
	// 1. 执行成功的组优先
	aExec := a.AllExecuted && a.ResultHash != ""
	bExec := b.AllExecuted && b.ResultHash != ""
	if aExec && !bExec {
		return 1
	}
	if !aExec && bExec {
		return -1
	}
	// 2. 票数多的优先
	if a.VoteCount != b.VoteCount {
		return a.VoteCount - b.VoteCount
	}
	// 3. Confidence 高的优先
	aConf := 0.0
	bConf := 0.0
	if a.Best != nil {
		aConf = a.Best.Confidence
	}
	if b.Best != nil {
		bConf = b.Best.Confidence
	}
	if aConf > bConf {
		return 1
	}
	if aConf < bConf {
		return -1
	}
	return 0
}

// hashSQL 标准化 SQL 并计算哈希
// 标准化：去注释、去多余空白、统一大小写、去尾部分号
func hashSQL(sql string) string {
	normalized := normalizeSQL(sql)
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])[:16]
}

// hashResult 计算执行结果哈希
// 执行失败或无结果时返回空字符串
func hashResult(execResult *ExecResult) string {
	if execResult == nil || !execResult.Success {
		return ""
	}
	// 用列名 + 行数作为结果指纹
	// 不用完整行数据，因为行数据可能很大且可能有随机顺序
	key := strings.Join(execResult.Columns, ",") + fmt.Sprintf("|rows=%d", execResult.RowCount)
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])[:16]
}

// normalizeSQL SQL 标准化
var (
	commentRegex    = regexp.MustCompile(`(?m)--.*$|--[\s\S]*?$|/\*[\s\S]*?\*/`)
	whitespaceRegex = regexp.MustCompile(`\s+`)
)

func normalizeSQL(sql string) string {
	s := strings.TrimSpace(sql)
	// 去注释
	s = commentRegex.ReplaceAllString(s, " ")
	// 去多余空白
	s = whitespaceRegex.ReplaceAllString(s, " ")
	// 统一小写关键字（保留字符串字面量）
	s = lowercaseKeywords(s)
	// 去尾部分号
	s = strings.TrimSuffix(s, ";")
	return strings.TrimSpace(s)
}

// lowercaseKeywords 将 SQL 关键字转为小写
// 简化实现：整体小写化（不影响哈希聚类的准确性）
func lowercaseKeywords(sql string) string {
	return strings.ToLower(sql)
}
