package text2sql

import (
	"fmt"
	"sync"
	"testing"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// stubKnowledgeRepo 只实现 kb 用到的那五个方法，让并发与累加两条断言不必连库也能测。
type stubKnowledgeRepo struct {
	mu       sync.Mutex
	rules    []modelai.BusinessRule
	caches   []modelai.TableCache
	listCall int
}

func (s *stubKnowledgeRepo) ListRules(_, _ int, _, _ string) ([]modelai.BusinessRule, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCall++
	out := make([]modelai.BusinessRule, len(s.rules))
	copy(out, s.rules)
	return out, int64(len(out)), nil
}

func (s *stubKnowledgeRepo) UpsertTableCache(*modelai.TableCache) error { return nil }

func (s *stubKnowledgeRepo) GetAllTableCaches() ([]modelai.TableCache, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]modelai.TableCache, len(s.caches))
	copy(out, s.caches)
	return out, nil
}

func (s *stubKnowledgeRepo) GetRelationsBySpace(uint64) ([]modelai.TableRelation, error) {
	return nil, nil
}

func (s *stubKnowledgeRepo) GetTableCachesBySpace(uint64) ([]modelai.TableCache, error) {
	return s.GetAllTableCaches()
}

// TestLoadRulesReplacesInsteadOfAccumulating 钉住规则翻倍：
// 同一批规则装载 N 次，每张表名下仍只能是 N 条里的那一份，不能变成 N×条数。
// 这条 bug 的用户可见形态是 prompt 里同一条业务规则被重复贴很多遍。
func TestLoadRulesReplacesInsteadOfAccumulating(t *testing.T) {
	repo := &stubKnowledgeRepo{rules: []modelai.BusinessRule{
		{TableNam: "users", Content: "软删过滤"},
		{TableNam: "users", Content: "时间用秒"},
		{TableNam: "orders", Content: "金额单位是分"},
	}}
	kb := NewKnowledgeBase(repo)

	for i := 0; i < 5; i++ {
		if err := kb.LoadRules(); err != nil {
			t.Fatalf("第 %d 次 LoadRules 失败: %v", i+1, err)
		}
		if got := len(kb.GetRules("users")); got != 2 {
			t.Fatalf("装载 %d 次后 users 名下有 %d 条规则（应为 2 —— 多出来的是重复累加，会全部拼进 prompt）", i+1, got)
		}
		if got := len(kb.GetRules("orders")); got != 1 {
			t.Fatalf("装载 %d 次后 orders 名下有 %d 条规则（应为 1）", i+1, got)
		}
	}
}

// TestGetRulesReturnsCopy 交出底层数组就等于把锁让出去：
// 调用方排序或 append 时，写侧可能正在改同一个数组。
func TestGetRulesReturnsCopy(t *testing.T) {
	repo := &stubKnowledgeRepo{rules: []modelai.BusinessRule{
		{TableNam: "users", Content: "a"},
		{TableNam: "users", Content: "b"},
	}}
	kb := NewKnowledgeBase(repo)
	if err := kb.LoadRules(); err != nil {
		t.Fatal(err)
	}

	first := kb.GetRules("users")
	if len(first) != 2 {
		t.Fatalf("前置条件：应取到 2 条，实得 %d", len(first))
	}
	first[0].Content = "被调用方改写了"

	if kb.GetRules("users")[0].Content != "a" {
		t.Fatal("GetRules 返回的是内部切片：调用方的一次就地改写污染了知识库")
	}
}

// TestReloadTablesFromCachesReplacesWholeMap 换表必须是整张替换：
// 上一轮的表名不该残留（残留会让 SchemaLinker 在已删掉的表上继续找列）。
func TestReloadTablesFromCachesReplacesWholeMap(t *testing.T) {
	kb := NewKnowledgeBase(&stubKnowledgeRepo{})
	kb.ReloadTablesFromCaches([]modelai.TableCache{{TableNam: "aw_user"}, {TableNam: "aw_doc"}})
	kb.ReloadTablesFromCaches([]modelai.TableCache{{TableNam: "aw_user"}})

	if len(kb.GetAllTables()) != 1 {
		t.Fatalf("替换后应只剩 1 张表，实得 %d —— 说明是往旧 map 里塞而不是换掉整张", len(kb.GetAllTables()))
	}
	if kb.GetTable("aw_doc") != nil {
		t.Fatal("旧表 aw_doc 仍在表集合里")
	}
}

// TestGetAllTablesIsDetachedFromInternalMap 取到内部 map 的调用方（agent_tools.go
// 连着 range 好几轮）在刷新进行时会撞上 concurrent map iteration and map write。
func TestGetAllTablesIsDetachedFromInternalMap(t *testing.T) {
	kb := NewKnowledgeBase(&stubKnowledgeRepo{})
	kb.ReloadTablesFromCaches([]modelai.TableCache{{TableNam: "a"}})

	snapshot := kb.GetAllTables()
	delete(snapshot, "a")
	snapshot["injected"] = &modelai.TableCache{TableNam: "injected"}

	if kb.GetTable("a") == nil {
		t.Fatal("对 GetAllTables 返回值的 delete 影响了内部状态 —— 交出的是内部 map 本身")
	}
	if kb.GetTable("injected") != nil {
		t.Fatal("对 GetAllTables 返回值的写入泄漏进了内部状态")
	}
}

// TestConcurrentRefreshAndReads 是这条修复的本体：
// 读侧走请求路径上的那几个 getter，写侧走 RefreshKnowledge 实际调用的那两个方法。
// 没有锁时 -race 会直接报 concurrent map read and map write（运行时 fatal，recover 接不住）。
func TestConcurrentRefreshAndReads(t *testing.T) {
	repo := &stubKnowledgeRepo{
		rules: []modelai.BusinessRule{{TableNam: "users", Content: "r"}, {TableNam: "orders", Content: "s"}},
	}
	kb := NewKnowledgeBase(repo)
	for i := 0; i < 40; i++ {
		repo.caches = append(repo.caches, modelai.TableCache{TableNam: fmt.Sprintf("t%d", i)})
	}
	if err := kb.LoadRules(); err != nil {
		t.Fatal(err)
	}
	kb.ReloadTablesFromCaches(repo.caches)

	var wg sync.WaitGroup
	const rounds = 300

	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				tables := kb.GetAllTables()
				for name := range tables {
					_ = kb.GetTable(name)
					_ = kb.GetRules(name)
					_ = kb.GetRelations(name)
				}
				_ = kb.SearchTables("t")
				_ = kb.GetAllRelations()
			}
		}()
	}

	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				if err := kb.LoadRules(); err != nil {
					t.Error(err)
					return
				}
				caches, err := kb.tableCachesForRefresh()
				if err != nil {
					t.Error(err)
					return
				}
				kb.ReloadTablesFromCaches(caches)
			}
		}()
	}

	wg.Wait()
}
