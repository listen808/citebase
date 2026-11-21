package wiki

import (
	"context"
	"sort"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
)

// pprDamping 阻尼系数：每次迭代 (1-α) 的概率跳回种子分布。
const pprDamping = 0.85

// pprIterations 幂迭代轮数。Wiki 页面数量小（单库百页量级），
// 15 轮足以收敛到工程精度，也保证 Retrieve 路径上的耗时可忽略。
const pprIterations = 15

// pprGraphSeedRatio 种子分布在传送向量里的占比。
// 剩余 (1-ratio) 均匀铺给全部页面 —— 全铺给种子会只搜「种子邻域」，
// 完全孤立但被 BM25 边缘命中的页面永远排不进来；
// 完全均匀又退化成普通 PageRank，问题相关性丢了。
const pprGraphSeedRatio = 0.6

// PPR 结果：按分数降序的页面 slug → 得分。
// 只有「至少存在一条边」的页面会出现在结果里；
// 孤立页在图上不可达，分数恒为 0，不参与重排。
type PPRResult struct {
	scores map[string]float64
}

// scoreOf 取某页得分，不存在返回 0。
func (r *PPRResult) scoreOf(slug string) float64 {
	if r == nil || r.scores == nil {
		return 0
	}
	return r.scores[slug]
}

// runPersonalizedPageRank 在 WikiLink 图上做个性化 PageRank。
//
// seeds 是种子页 slug → 种子权重（未归一化，由调用方给原始分，
// 比如目录路给 1、BM25 路给其归一化分数）。
// 幂迭代收敛后返回每页得分。图是双向语义的（A 引用 B 意味着 B 与 A 相关），
// 邻接矩阵对称，直接迭代而不必处理有向图的入边/出边归一化差异。
//
// 返回 nil 表示图不可用（没有任何链接），调用方应直接跳过这一路。
func runPersonalizedPageRank(ctx context.Context, spaceID int, pages []*knowledgemodel.WikiPage, seeds map[string]float64) *PPRResult {
	links, err := knowledgerepo.NewWikiLinkRepository().ListBySpace(spaceID)
	if err != nil || len(links) == 0 {
		return nil
	}

	// 节点 = 当前有效页 ∩ 出现在边里的页。
	// 不在任何边里的页分数恒为 0，不进迭代。
	pageSet := make(map[string]bool, len(pages))
	for _, p := range pages {
		pageSet[p.Slug] = true
	}
	nodeSet := make(map[string]bool)
	for _, l := range links {
		if pageSet[l.FromSlug] && pageSet[l.ToSlug] {
			nodeSet[l.FromSlug] = true
			nodeSet[l.ToSlug] = true
		}
	}
	if len(nodeSet) == 0 {
		return nil
	}
	nodeIDs := make([]string, 0, len(nodeSet))
	for s := range nodeSet {
		nodeIDs = append(nodeIDs, s)
	}
	sort.Strings(nodeIDs)
	idxOf := make(map[string]int, len(nodeIDs))
	for i, s := range nodeIDs {
		idxOf[s] = i
	}

	// 邻接：对称累加边权（A→B 与 B→A 都算）。
	n := len(nodeIDs)
	adj := make([][]float64, n)
	for i := range adj {
		adj[i] = make([]float64, n)
	}
	deg := make([]float64, n)
	for _, l := range links {
		i, okI := idxOf[l.FromSlug]
		j, okJ := idxOf[l.ToSlug]
		if !okI || !okJ {
			continue
		}
		w := l.Weight
		if w < 0 {
			w = 0
		}
		adj[i][j] += w
		adj[j][i] += w
		deg[i] += w
		deg[j] += w
	}

	// 传送向量：pprGraphSeedRatio 给种子，其余均匀铺给全部页。
	// 种子只认节点集合里的 slug；种子分归一化到占比内。
	v := make([]float64, n)
	seedTotal := 0.0
	for _, s := range nodeIDs {
		if w, ok := seeds[s]; ok && w > 0 {
			seedTotal += w
		}
	}
	if seedTotal <= 0 {
		return nil
	}
	uniform := (1 - pprGraphSeedRatio) / float64(n)
	for i, s := range nodeIDs {
		v[i] = uniform
		if w, ok := seeds[s]; ok && w > 0 {
			v[i] += pprGraphSeedRatio * (w / seedTotal)
		}
	}

	// 幂迭代：score ← α · M · score + (1-α) · v
	// 行随机化 M[i][j] = adj[i][j] / deg[i]（无出边的页保留自己的分数，
	// 等价于 M[i][i] = 1，避免分数在孤点处蒸发）。
	score := make([]float64, n)
	for i := range score {
		score[i] = v[i]
	}
	next := make([]float64, n)
	for it := 0; it < pprIterations; it++ {
		if ctx.Err() != nil {
			return nil
		}
		for i := 0; i < n; i++ {
			var m float64
			if deg[i] > 0 {
				for j := 0; j < n; j++ {
					if adj[i][j] > 0 {
						m += (adj[i][j] / deg[i]) * score[j]
					}
				}
			} else {
				m = score[i]
			}
			next[i] = pprDamping*m + (1-pprDamping)*v[i]
		}
		score, next = next, score
	}

	out := &PPRResult{scores: make(map[string]float64, n)}
	var maxScore float64
	for i := range nodeIDs {
		if score[i] > maxScore {
			maxScore = score[i]
		}
	}
	if maxScore <= 0 {
		return nil
	}
	// 归一化到 0–1，直接可当排序加成。
	for i, s := range nodeIDs {
		if score[i] > 0 {
			out.scores[s] = score[i] / maxScore
		}
	}
	return out
}

// graphCandidate 一条图扩展候选：slug + 归一化 PPR 分。
type graphCandidate struct {
	Slug  string
	Score float64
}

// graphExpand 第三路召回：个性化 PageRank 在链接图上扩展。
//
// 前两路（目录 ∪ 全文）各自只看「这一页」，图路回答的是另一个问题：
// 「和已命中页紧挨着的页里，哪些值得一起看」。HippoRAG-2 的实测结论是
// 单跳图扩展（PPR 即其最小实现）对多跳问题增益最显著，且召回集合
// 与文本检索近乎互补。
//
// 返回的候选分数已归一化到 0–1，量级小于 BM25 分，
// 在 Retrieve 的合并里只够把高关联页顶进候选窗口，
// 不会压过真正文本命中的页（那是刻意设计：图路是补充，不是改写）。
func (r *Retriever) graphExpand(ctx context.Context, spaceID int, pages []*knowledgemodel.WikiPage, seeds map[string]float64) []graphCandidate {
	res := runPersonalizedPageRank(ctx, spaceID, pages, seeds)
	if res == nil || len(res.scores) == 0 {
		return nil
	}
	all := make([]graphCandidate, 0, len(res.scores))
	for slug, s := range res.scores {
		all = append(all, graphCandidate{Slug: slug, Score: s})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > maxCandidates {
		all = all[:maxCandidates]
	}
	return all
}

// normalizeForPPR 把任意正的原始分集合归一化（最大分 → 1），
// 作为 PPR 种子权重。空集合返回 nil。
func normalizeForPPR(in map[string]float64) map[string]float64 {
	var max float64
	for _, w := range in {
		if w > max {
			max = w
		}
	}
	if max <= 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for s, w := range in {
		if w > 0 {
			out[s] = w / max
		}
	}
	return out
}
