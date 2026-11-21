package workspace

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
)

// TestEveryWebhookEventHasAFireSite 是给"功能存在但没人接线"这一类缺陷立的常驻门禁。
//
// 起因：TriggerWebhooks 全仓零调用方，而它的另一半全都活着 —— 建/改/删 webhook 的
// CRUD 路由、管理页「工具 → Webhook 嵌入」、投递记录查询，以及 SupportedWebhookEvents()
// 里那六个事件名。用户配置得认认真真，一条请求也发不出去，且没有任何一处会报错。
//
// 判据：每个受支持的事件名，必须作为 FireWebhook 的事件实参出现在 internal/ 下某个
// 非测试 .go 文件里。刻意不看 DB、不看运行时 —— 运行时工装只看渲染出来的东西，
// 而"一次 HTTP 外呼有没有发生"根本不是渲染态能读出来的量。
func TestEveryWebhookEventHasAFireSite(t *testing.T) {
	constants := map[string]string{
		workspacemodel.WebhookEventDocCreated:        "WebhookEventDocCreated",
		workspacemodel.WebhookEventDocUpdated:        "WebhookEventDocUpdated",
		workspacemodel.WebhookEventDocDeleted:        "WebhookEventDocDeleted",
		workspacemodel.WebhookEventCommentCreated:    "WebhookEventCommentCreated",
		workspacemodel.WebhookEventApprovalSubmitted: "WebhookEventApprovalSubmitted",
		workspacemodel.WebhookEventApprovalHandled:   "WebhookEventApprovalHandled",
	}

	// 前提本身：对照表必须与 SupportedWebhookEvents 一一对应，
	// 否则"每个事件都有接线"会漏掉新增事件（新增了但没进这张表 = 测试永远绿）。
	supported := SupportedWebhookEvents()
	if len(supported) != len(constants) {
		t.Fatalf("SupportedWebhookEvents 有 %d 条，本测试的对照表只有 %d 条 —— 新增事件时必须同时在这里登记", len(supported), len(constants))
	}
	for _, e := range supported {
		if _, ok := constants[e]; !ok {
			t.Fatalf("事件 %q 在 SupportedWebhookEvents 里，但对照表没有它，本测试看不见它", e)
		}
	}

	var files []string
	root := filepath.Join("..", "..")
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// _test.go 里的名字是断言素材，不是接线；testdata 同理。
		if info.IsDir() && (info.Name() == "testdata" || info.Name() == ".rv-tmp") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描 %s 失败: %v", root, err)
	}
	if len(files) < 100 {
		t.Fatalf("只扫到 %d 个源文件，明显低于本仓规模 —— 扫描退化会让下面所有断言安静地全绿", len(files))
	}

	hit := map[string][]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "FireWebhook(") {
				continue
			}
			for event, constName := range constants {
				if strings.Contains(line, "."+constName) {
					rel := filepath.ToSlash(f)
					hit[event] = append(hit[event], rel+":"+strconv.Itoa(i+1))
				}
			}
		}
	}

	for event, constName := range constants {
		if len(hit[event]) == 0 {
			t.Errorf("事件 %q（%s）没有任何 FireWebhook 调用点 —— 这正是当初 TriggerWebhooks 零调用方的形态："+
				"配置页、事件列表、投递记录全都在，就是没人发。请在对应动作成功后调 FireWebhook", event, constName)
		}
	}
	t.Logf("webhook 接线覆盖：")
	for event, sites := range hit {
		t.Logf("  %-22s %s", event, strings.Join(sites, ", "))
	}
}

// TestFireWebhookIsSafeWithoutDB 钉住两个"不该炸"的场合：
// 数据库尚未就绪（启动早期与全部单测）与事件名写错。
// 派发失败可以静默，但绝不能把一次成功的文档保存变成 500 或 panic。
func TestFireWebhookIsSafeWithoutDB(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		FireWebhook(workspacemodel.WebhookEventDocCreated, 1, func(p *WebhookEventPayload) {
			p.DocumentID = 7
		})
	}()
	// 未知事件名：应当只记一条日志并直接返回。
	FireWebhook("document.did-something", 1, nil)
	wg.Wait()
}
