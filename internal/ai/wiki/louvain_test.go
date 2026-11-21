package wiki

import (
	"sort"
	"testing"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

func mkEdges(pairs [][2]string, weights []float64) []louvainEdge {
	edges := make([]louvainEdge, 0, len(pairs))
	for i, p := range pairs {
		edges = append(edges, louvainEdge{a: p[0], b: p[1], w: weights[i]})
	}
	return edges
}

// 两个稠密三角形靠一条细桥连接：Louvain 应按密度切成两个社区，
// 而并查集连通分量会错误地把六个节点并成一个。
func TestLouvainPartitionSplitsBridgeCommunities(t *testing.T) {
	nodes := []string{"a1", "a2", "a3", "b1", "b2", "b3"}
	edges := mkEdges(
		[][2]string{{"a1", "a2"}, {"a2", "a3"}, {"a1", "a3"},
			{"b1", "b2"}, {"b2", "b3"}, {"b1", "b3"},
			{"a3", "b1"}},
		[]float64{3, 3, 3, 3, 3, 3, 0.1},
	)

	got := louvainPartition(nodes, edges)
	commA := got["a1"]
	for _, s := range []string{"a2", "a3"} {
		if got[s] != commA {
			t.Fatalf("a-clique should share one community, got %v", got)
		}
	}
	commB := got["b1"]
	for _, s := range []string{"b2", "b3"} {
		if got[s] != commB {
			t.Fatalf("b-clique should share one community, got %v", got)
		}
	}
	if commA == commB {
		t.Fatalf("bridge-connected cliques should split into two communities, got %v", got)
	}
}

// 确定性：同图两次划分结果必须一致（目录缓存失效判断依赖这一点）。
func TestLouvainPartitionDeterministic(t *testing.T) {
	nodes := []string{"x", "y", "z"}
	edges := mkEdges([][2]string{{"x", "y"}, {"y", "z"}}, []float64{2, 1})
	first := louvainPartition(nodes, edges)
	second := louvainPartition(nodes, edges)
	for _, n := range nodes {
		if first[n] != second[n] {
			t.Fatalf("partition must be deterministic: %v vs %v", first, second)
		}
	}
}

// communities 的输出形状：排序稳定、覆盖所有页面（含孤立单例）。
func TestCommunitiesCoversIsolatedPages(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		{Slug: "p1", Status: "published"},
		{Slug: "p2", Status: "published"},
		{Slug: "lonely", Status: "published"},
	}
	links := []*knowledgemodel.WikiLink{
		{FromSlug: "p1", ToSlug: "p2", Weight: 1},
	}
	got := communities(pages, links)
	var slugs []string
	for _, g := range got {
		slugs = append(slugs, g...)
	}
	sort.Strings(slugs)
	want := []string{"lonely", "p1", "p2"}
	if len(slugs) != len(want) {
		t.Fatalf("communities must cover every page exactly once, got %v", got)
	}
	for i := range want {
		if slugs[i] != want[i] {
			t.Fatalf("missing page %s in %v", want[i], slugs)
		}
	}
}
