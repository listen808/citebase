package wiki

import (
	"reflect"
	"testing"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

func pageForTest(slug string, sources []int, provenance string) *knowledgemodel.WikiPage {
	return &knowledgemodel.WikiPage{Slug: slug, Sources: sources, Provenance: provenance}
}

func TestCascadePlanExclusivePageRemoved(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		pageForTest("only-here", []int{7}, knowledgemodel.WikiProvenanceLLM),
	}
	remove, prune := CascadePlan(pages, 7)
	if !reflect.DeepEqual(remove, []string{"only-here"}) {
		t.Fatalf("remove = %v, want [only-here]", remove)
	}
	if len(prune) != 0 {
		t.Fatalf("prune = %v, want empty", prune)
	}
}

func TestCascadePlanSharedPagePrunedNotRemoved(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		pageForTest("shared", []int{7, 9}, knowledgemodel.WikiProvenanceLLM),
	}
	remove, prune := CascadePlan(pages, 7)
	if len(remove) != 0 {
		t.Fatalf("remove = %v, want empty（共享页不能整页删）", remove)
	}
	if !reflect.DeepEqual(prune, []string{"shared"}) {
		t.Fatalf("prune = %v, want [shared]", prune)
	}
}

func TestCascadePlanHumanPageNeverTouched(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		pageForTest("human-only", []int{7}, knowledgemodel.WikiProvenanceHuman),
	}
	remove, prune := CascadePlan(pages, 7)
	if len(remove) != 0 || len(prune) != 0 {
		t.Fatalf("human 页不应被级联：remove=%v prune=%v", remove, prune)
	}
}

func TestCascadePlanUnrelatedPageIgnored(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		pageForTest("other-source", []int{8}, knowledgemodel.WikiProvenanceLLM),
	}
	remove, prune := CascadePlan(pages, 7)
	if len(remove) != 0 || len(prune) != 0 {
		t.Fatalf("无关页面不应参与级联：remove=%v prune=%v", remove, prune)
	}
}

func TestCascadePlanInvalidSourceDoesNothing(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		pageForTest("any", []int{0}, knowledgemodel.WikiProvenanceLLM),
	}
	remove, prune := CascadePlan(pages, 0)
	if len(remove) != 0 || len(prune) != 0 {
		t.Fatalf("sourceDocID<=0 应直接返回空：remove=%v prune=%v", remove, prune)
	}
}
