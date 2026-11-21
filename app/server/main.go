package main

import (
	"fmt"
	"net"
	"os"
	"runtime/debug"

	"github.com/listen808/citebase/internal/ai/facade"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/bootstrap"
	"github.com/listen808/citebase/internal/brand"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/repository"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

// @title			Citebase API
// @version		2.0.0
// @description	Enterprise AI Knowledge Management Platform
// @description	Wiki + RAG + Text-to-SQL all-in-one
// @host			localhost:3001
// @BasePath		/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Bearer {token}
func main() {
	utils.InitLogger()

	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("[FATAL] Panic recovered: %v\n%s", r, string(debug.Stack()))
			fmt.Fprintf(os.Stderr, "\n[FATAL] Server crashed: %v\n", r)
			os.Exit(1)
		}
	}()

	logx.Info("=== " + brand.DefaultName + " starting ===")

	report := bootstrap.NewStartupReport()

	// Step 1: Config
	config.InitConfig()
	report.AddOK("Config")

	// Step 1.5: 端口预检 —— 必须放在所有重量级初始化之前。
	// 端口校验原本只在 Step 8 的 ListenAndServe 里：若本机已有一个实例在跑，
	// 第二个实例会先在 Step 4/5 被 BoltDB/bleve 的文件锁卡死（阻塞而非报错），
	// 永远走不到端口校验，表现为"进程活着但不在服务"的僵尸（2026-11-03 实测）。
	// 先试绑再放开：重复启动几秒内明确 Fatal 退出；预检到 Step 8 之间的
	// 理论竞态窗口由 ListenAndServe 的 Fatalf 兜底。
	preHost := config.AppConfig.Server.Host
	if preHost == "" {
		preHost = "0.0.0.0"
	}
	if ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", preHost, config.AppConfig.Server.Port)); err != nil {
		logx.Fatalf("端口 %d 已被占用（大概率已有一个 %s 实例在运行，先停掉它再启动）: %v", config.AppConfig.Server.Port, brand.DefaultName, err)
	} else {
		ln.Close()
	}

	// Step 2: Database
	repository.InitDB()
	if repository.DB != nil {
		report.AddOK("Database")
	} else {
		report.AddFail("Database", "connection failed")
	}

	// 站点展示名的读取口子：管理员在「系统设置 → 基础」改 main_title 之后，
	// 分享页标签、邮件、巡检报告、/site 端点都跟着变（判据见 internal/brand）。
	// 未注入或读失败都退回代码默认名，所以 DB 连不上也不影响这一步。
	brand.InstallBrandReader(adminrepo.NewConfigRepository().GetValueByKey)

	// Step 3: Cache
	repository.InitCache()
	if repository.AppCache != nil {
		report.AddOK("Cache")
	} else {
		report.AddWarn("Cache", "fallback to memory")
	}

	// Step 4: Search Index
	initSearchIndex()
	report.AddOK("Search Index")

	// Step 5: Vector Store
	initVectorStore()
	report.AddOK("Vector Store")

	// Step 6: AI Module
	aiModule := initAI()
	defer provider.StopTokenQuota()
	if aiModule != nil {
		if config.AppConfig.AI.Enabled {
			if aiModule.PrimaryProvider() != nil {
				report.AddOK("AI Module")
			} else {
				report.AddWarn("AI Module", "enabled but provider not ready")
			}
		} else {
			report.AddSkip("AI Module", "disabled")
		}
	} else {
		report.AddWarn("AI Module", "not initialized")
	}

	// Step 7: Text2SQL
	schemaSvc := initText2SQL(aiModule)
	if schemaSvc != nil && schemaSvc.GetTextToSQLGenerator() != nil {
		report.AddOK("Text2SQL")
	} else {
		report.AddWarn("Text2SQL", "generator not ready")
	}

	aiFacade := facade.NewAIFacade(aiModule, schemaSvc, schemaSvc.GetTextToSQLGenerator())

	// Step 8: HTTP Server
	r := setupGin()
	setupRoutes(r, aiFacade)
	setupStaticAndSPA(r)
	report.AddOK("HTTP Server")

	// 打印启动状态汇总
	fmt.Print(report.Format())

	startServer(r)
}
