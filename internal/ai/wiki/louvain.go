package wiki

import (
	"sort"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// louvainEdge 无向加权边。
type louvainEdge struct {
	a, b string
	w    float64
}

// louvainPartition 单层 Louvain（局部模块度最大化，反复扫描直到无节点移动）。
//
// 相比并查集连通分量，Louvain 按边的密度分布切分：稠密但只有细桥连接的
// 两个簇会被正确分开，而不是被一条边粘成一个巨大社区。
//
// 判据用教科书简化增益式：gain(i→C) = k_i_in − Σtot(C)·k_i/m2，
// 常数项在移动前后抵消，不影响 argmax。
// 对知识库规模的图（几十~几百页）单层已能可靠捕捉密度划分；
// 多层聚合（社区折叠成超级节点再迭代）是后续升级点，此处刻意不做 ——
// 收益只在十万节点级图上显现，复杂度却翻三倍。
// 固定遍历顺序（不随机化）保证同图结果确定，否则目录缓存的失效判断会抖动。
func louvainPartition(nodes []string, edges []louvainEdge) map[string]int {
	idx := make(map[string]int, len(nodes))
	for i, s := range nodes {
		idx[s] = i
	}
	n := len(nodes)
	if n == 0 {
		return map[string]int{}
	}

	adj := make([]map[int]float64, n)
	for i := range adj {
		adj[i] = make(map[int]float64)
	}
	k := make([]float64, n)
	for _, e := range edges {
		i, ok1 := idx[e.a]
		j, ok2 := idx[e.b]
		if !ok1 || !ok2 || i == j {
			continue
		}
		adj[i][j] += e.w
		adj[j][i] += e.w
		k[i] += e.w
		k[j] += e.w
	}

	m2 := 0.0
	for _, kv := range k {
		m2 += kv
	}

	community := make([]int, n)
	for i := range community {
		community[i] = i
	}
	if m2 <= 0 {
		return toCommunityMap(nodes, community) // 全图无边：单例社区
	}

	// sigmaTot：社区度数和。社区内部边权在简化增益式里
	// 通过"移出后按邻居社区重算 k_i_in"体现，无需单独维护。
	sigmaTot := make([]float64, n)
	copy(sigmaTot, k)

	const eps = 1e-9
	const maxPasses = 8
	for pass := 0; pass < maxPasses; pass++ {
		moved := false
		for i := 0; i < n; i++ {
			ci := community[i]

			// 试移出：暂时从原社区扣除自己的度
			sigmaTot[ci] -= k[i]

			// 计算与各邻居社区的连接权重
			kIn := make(map[int]float64, len(adj[i]))
			for j, w := range adj[i] {
				if j == i {
					continue
				}
				kIn[community[j]] += w
			}

			// 当前社区的增益（留在原地的基线）
			bestC, bestGain := ci, kIn[ci]-sigmaTot[ci]*k[i]/m2
			for c, w := range kIn {
				if c == ci {
					continue
				}
				if gain := w - sigmaTot[c]*k[i]/m2; gain > bestGain+eps {
					bestGain, bestC = gain, c
				}
			}

			// 落位
			sigmaTot[bestC] += k[i]
			if bestC != ci {
				community[i] = bestC
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	return toCommunityMap(nodes, community)
}

func toCommunityMap(nodes []string, community []int) map[string]int {
	// 社区号压缩成 0..k 连续编号，保持输出稳定
	renumber := make(map[int]int)
	out := make(map[string]int, len(nodes))
	for i, s := range nodes {
		c := community[i]
		if _, ok := renumber[c]; !ok {
			renumber[c] = len(renumber)
		}
		out[s] = renumber[c]
	}
	return out
}

// communities 用 Louvain 社区检测做空间内页面的社区划分（升级自并查集连通分量）。
// 返回每个社区的成员 slug 列表（内部排序，社区间按首成员排序，结果确定）。
func communities(pages []*knowledgemodel.WikiPage, links []*knowledgemodel.WikiLink) [][]string {
	nodes := make([]string, 0, len(pages))
	for _, p := range pages {
		nodes = append(nodes, p.Slug)
	}
	sort.Strings(nodes)

	// 同一对页面的多条链接权重累加；weight<=0 的边按 1 计
	pairWeight := make(map[[2]string]float64)
	for _, l := range links {
		a, b := l.FromSlug, l.ToSlug
		if a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		w := l.Weight
		if w <= 0 {
			w = 1
		}
		pairWeight[[2]string{a, b}] += w
	}
	edges := make([]louvainEdge, 0, len(pairWeight))
	for pair, w := range pairWeight {
		edges = append(edges, louvainEdge{a: pair[0], b: pair[1], w: w})
	}

	assignment := louvainPartition(nodes, edges)
	groups := make(map[int][]string)
	for _, s := range nodes {
		c := assignment[s]
		groups[c] = append(groups[c], s)
	}
	out := make([][]string, 0, len(groups))
	for _, g := range groups {
		sort.Strings(g)
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}
