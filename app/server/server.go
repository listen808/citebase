package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	path "path"
	filepath "path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/listen808/citebase/docs/swagger" // Swagger 自动生成的 API 文档
	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/facade"
	adminapi "github.com/listen808/citebase/internal/api/admin"
	aiapi "github.com/listen808/citebase/internal/api/ai"
	authapi "github.com/listen808/citebase/internal/api/auth"
	collabapi "github.com/listen808/citebase/internal/api/collab"
	commonapi "github.com/listen808/citebase/internal/api/common"
	knowledgeapi "github.com/listen808/citebase/internal/api/knowledge"
	schemaapi "github.com/listen808/citebase/internal/api/schema"
	searchapi "github.com/listen808/citebase/internal/api/search"
	socialapi "github.com/listen808/citebase/internal/api/social"
	"github.com/listen808/citebase/internal/api/websocket"
	workspaceapi "github.com/listen808/citebase/internal/api/workspace"
	"github.com/listen808/citebase/internal/brand"
	collab "github.com/listen808/citebase/internal/collab"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/maintenance"
	"github.com/listen808/citebase/internal/middleware"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/queue"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/envname"
	"github.com/listen808/citebase/internal/utils/safego"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// setupGin 创建 Gin 引擎并注册全局中间件
func setupGin() *gin.Engine {
	gin.SetMode(config.AppConfig.Server.Mode)
	r := gin.New()
	// 默认不信任任何反向代理：gin 默认信任所有代理，会让 X-Forwarded-For 决定 c.ClientIP()，
	// 从而伪造来源 IP 绕过 IP 白名单与按 IP 记数的登录锁定。
	// 部署在 Nginx/负载均衡后面时，用 server.trusted_proxies 或 AIWIKI_TRUSTED_PROXIES 声明网段。
	trustedProxies := config.AppConfig.Server.TrustedProxies
	if len(trustedProxies) == 0 {
		if v := strings.TrimSpace(envname.Getenv("CITEBASE_TRUSTED_PROXIES", "AIWIKI_TRUSTED_PROXIES")); v != "" {
			for _, item := range strings.Split(v, ",") {
				if item = strings.TrimSpace(item); item != "" {
					trustedProxies = append(trustedProxies, item)
				}
			}
		}
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		logx.Errorf("配置可信代理失败: %v", err)
	}
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.IPWhitelistMiddleware())
	r.Use(middleware.RateLimitMiddleware())
	// P1-10: 自定义 panic recover（带结构化日志），替代 gin.Recovery()
	r.Use(func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logx.Errorf("[PANIC] %v\n%s\n", err, string(debug.Stack()))
				response.Error(c, http.StatusInternalServerError, 500, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	})
	// 安全响应头（CSP / nosniff / Referrer-Policy）：CSP 是登录态 cookie
	// 维持 JS 可写（见 docs/decision-log/2026-11-07-auth-cookie-and-csp.md）
	// 时 XSS 的最后一道闸门，必须在 panic-recover 之后、业务路由之前生效。
	r.Use(middleware.SecurityHeadersMiddleware())
	// P1-11: 移除 gin.LoggerWithWriter，访问日志由 middleware.AccessLog 统一记录到 DB
	// P1 修复：AccessLog 提升为全局中间件，覆盖所有路由（auth/workspace/search/social/ai 等）
	r.Use(middleware.AccessLog(adminsvc.NewLogService()))
	r.Use(middleware.MetricsMiddleware())
	return r
}

// setupRoutes 注册所有 API 路由
func setupRoutes(r *gin.Engine, aiFacade *facade.AIFacade) {
	v1 := r.Group("/api/v1")
	v1.GET("/health", commonapi.HealthCheck)
	// 站点展示名（公开）：登录页与标签页标题在拿到令牌之前就要显示名字。
	// 判据在 internal/brand —— 管理员改「系统设置 → 基础」里的站点标题必须真的生效。
	v1.GET("/site", commonapi.SiteInfo)
	// P1-5：readiness 探真实依赖（MySQL/缓存），失败返回 503 让编排层摘流量。
	// liveness 仍是无条件 UP，避免一次 DB 抖动把所有副本重启一遍。
	v1.GET("/ready", commonapi.ReadyCheck)
	authapi.RegisterRoutes(v1)
	adminapi.RegisterRoutes(v1)
	workspaceapi.RegisterRoutes(v1)
	searchapi.RegisterRoutes(v1)
	socialapi.RegisterRoutes(v1)
	knowledgeapi.RegisterRoutes(v1)
	schemaapi.RegisterRoutes(v1)
	aiapi.RegisterAIRoutes(v1, aiFacade)
	collabapi.RegisterRoutes(r, v1)

	// 开放平台对外端点（G029）：走 API Key 鉴权
	// 管理端点（创建/列表/吊销）已在 adminapi.RegisterRoutes 的 authGroup 中注册
	openGroup := v1.Group("/open")
	openGroup.Use(middleware.ApiKeyAuthMiddleware())
	adminapi.RegisterOpenAPIRoutes(openGroup)

	// WebSocket 实时通知（需要认证）
	v1.GET("/ws", middleware.AuthMiddleware(), func(c *gin.Context) {
		websocket.HandleWS(c.Writer, c.Request)
	})

	// Prometheus metrics 端点（HTTP 指标 + 运行时指标 + Text2SQL 指标 + AI 指标）
	// P1-5：不再要求 JWT——Prometheus 抓取器拿不到用户 token，指标等于没人看。
	// 指标本身不含业务数据；若部署在公网，请在网络层（安全组/Ingress）限制来源。
	r.GET("/metrics", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		middleware.WriteMetrics(c.Writer)
		text2sql.GetMetricsRegistry().WritePrometheus(c.Writer)
		middleware.WriteAIMetrics(c.Writer)
	})

	// Swagger API 文档：非生产模式 + 显式开启才注册。
	// 仅凭 mode != release 就把完整 API 面（含参数与内部接口）暴露给未鉴权访问者，等于给攻击者的地图。
	swaggerEnabled := config.AppConfig.Server.SwaggerOpen || envname.Getenv("CITEBASE_SWAGGER", "AIWIKI_SWAGGER") == "1"
	if config.AppConfig.Server.Mode != "release" && swaggerEnabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
}

// setupStaticAndSPA 配置静态资源服务和 SPA 兜底路由
func setupStaticAndSPA(r *gin.Engine) {
	imageDir := config.AppConfig.Storage.ImageDir
	if imageDir == "" {
		imageDir = "./data/images"
	}
	attachmentDir := config.AppConfig.Storage.AttachmentDir
	if attachmentDir == "" {
		attachmentDir = "./data/attachment"
	}
	imageDir = toAbsDir(imageDir)
	attachmentDir = toAbsDir(attachmentDir)
	logx.Infof("图片目录: %s", imageDir)
	logx.Infof("附件目录: %s", attachmentDir)

	// 静态文件路由需鉴权：路径形如 /attachments/{spaceID}/{documentID}/{filename}
	// 通过解析 spaceID 校验用户对该空间的访问权限，避免私有空间附件被未授权下载
	// P2：attachment_source=db 且 storage_kind=1 的记录改从 aw_attachment_chunk 流式输出；
	// 第二个参数是库里存的 path 前缀（/attachments/ URL 对应 attachment/，与 URL 多个 s 不同）
	r.GET("/images/*filepath", middleware.OptionalAuthMiddleware(), serveAttachmentWithAuth(imageDir, "images/", false))
	r.GET("/attachments/*filepath", middleware.OptionalAuthMiddleware(), serveAttachmentWithAuth(attachmentDir, "attachment/", true))
	r.HEAD("/images/*filepath", middleware.OptionalAuthMiddleware(), serveAttachmentHeadWithAuth(imageDir, "images/", true))
	r.HEAD("/attachments/*filepath", middleware.OptionalAuthMiddleware(), serveAttachmentHeadWithAuth(attachmentDir, "attachment/", false))

	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/favicon.ico", "./web/dist/favicon.ico")
	r.StaticFile("/manifest.webmanifest", "./web/dist/manifest.webmanifest")
	r.StaticFile("/sw.js", "./web/dist/sw.js")
	r.StaticFile("/registerSW.js", "./web/dist/registerSW.js")
	r.StaticFile("/", "./web/dist/index.html")

	// 分享页要服务端注入 og:*：这条链接的唯一价值就是"被发到别处后能被认出来"，
	// 而聊天工具的抓取不跑 JS，NoRoute 那份原样送出的 index.html 里什么标签都没有。
	// 注册在 NoRoute 之前，其余路由仍走兜底、不多读一次盘。
	workspaceapi.RegisterSharePageRoutes(r)

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/images/") ||
			strings.HasPrefix(p, "/attachments/") || strings.HasPrefix(p, "/assets/") {
			response.Error(c, http.StatusNotFound, 404, "not found")
			return
		}
		if strings.HasPrefix(p, "/workbox-") && strings.HasSuffix(p, ".js") {
			c.File("./web/dist/" + strings.TrimPrefix(p, "/"))
			return
		}
		c.File("./web/dist/index.html")
	})
}

func toAbsDir(dir string) string {
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
	}
	return dir
}

// startServer 启动 HTTP 服务并优雅关停
func startServer(r *gin.Engine) {
	host := config.AppConfig.Server.Host
	if host == "" {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", host, config.AppConfig.Server.Port)
	logx.Infof(brand.DefaultName+" Server starting on %s...", addr)

	srv := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 10 * time.Second}

	// 启动消息队列后台 worker（G034 基础）
	queue.Start()
	// 周期清理（回收站记录 / webhook 投递记录 / 过期 AI 记忆）
	maintenance.Start()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logx.Fatalf("[FATAL] Server error: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logx.Info("[Server] Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// 关停顺序按"先断新工作，再排在途的"：
	//  1. 调度器先停 —— 否则优雅关停期间 cron 还会再起一次分析/git pull，
	//     而那些任务拿的是一个正在关闭的数据库连接。
	//  2. 协作编辑的内存 Yjs 状态刷盘（DocManager 是单例，Stop 会等刷完或超时 10s）。
	//  3. 队列停止收新消息并等待在途消息完成。
	//  4. 最后关 HTTP —— 此时不会再有新请求进来触发上面任何一步。
	aimod.GetModule().Shutdown()
	maintenance.Stop()
	collab.ShutdownGlobal()
	if err := queue.Stop(ctx); err != nil {
		logx.Errorf("[Server] queue stop timeout: %v", err)
	}
	if err := srv.Shutdown(ctx); err != nil {
		logx.Fatalf("[Server] Forced shutdown: %v", err)
	}
}

// parseSpaceIDFromPath 从静态文件路径 /{spaceID}/{documentID}/{filename} 中解析 spaceID
// 路径示例：/3/15/1690000000_12345.png
func parseSpaceIDFromPath(fp string) (int, error) {
	// 统一用 / 切分
	cleaned := strings.TrimPrefix(fp, "/")
	cleaned = strings.TrimSuffix(cleaned, "/")
	parts := strings.Split(cleaned, "/")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid path format")
	}
	return strconv.Atoi(parts[0])
}

// checkAttachmentAccess 校验登录用户对路径所属空间的访问权限
// 公开空间（visit_level=public）放行；非公开空间需为成员
// 未登录用户（浏览器 <img> 等不带 token 的请求）仅可访问公开空间
func checkAttachmentAccess(c *gin.Context, fp string) bool {
	spaceID, err := parseSpaceIDFromPath(fp)
	if err != nil || spaceID <= 0 {
		// 路径不合法，拒绝
		response.Error(c, http.StatusForbidden, 403, "forbidden")
		return false
	}

	spaceRepo := workspacerepo.NewSpaceRepository()
	space, err := spaceRepo.FindByIDCached(spaceID)
	if err != nil || space == nil || space.IsDelete == 1 {
		response.Error(c, http.StatusNotFound, 404, "space not found")
		return false
	}
	// 公开空间放行（含未登录用户，如浏览器 <img> 请求）
	if space.IsPublic() {
		return true
	}

	// 非公开空间需要登录
	userID, err := middleware.GetUserID(c)
	if err != nil || userID <= 0 {
		response.Unauthorized(c, "未登录")
		return false
	}
	roleID, _ := c.Get("role_id")
	if roleID != nil && roleID.(int) == 1 {
		// 超级管理员放行
		return true
	}
	// 非公开空间需为成员
	spaceUser, err := spaceRepo.FindSpaceUserCached(spaceID, userID)
	if err != nil || spaceUser == nil {
		response.Error(c, http.StatusForbidden, 403, "no access permission to this space")
		return false
	}
	return true
}

// dbAttachmentForPath 查库取附件记录：attachment_source=db 且 storage_kind=1 才走库，
// 其余（file 模式、未迁移的 kind=2 记录、无库记录的孤儿文件）返回 nil 落回磁盘读。
func dbAttachmentForPath(storedPrefix, fp string) *workspacemodel.Attachment {
	if !workspacerepo.UseDB() {
		return nil
	}
	att, err := workspacerepo.NewAttachmentRepository().FindByPath(storedPrefix + fp)
	if err != nil || att.StorageKind != workspacemodel.AttachmentStorageDB {
		return nil
	}
	return att
}

// serveAttachmentFromDB 从 aw_attachment_chunk 按 seq 逐块流式输出（内存峰值一个块 4MB）。
func serveAttachmentFromDB(c *gin.Context, att *workspacemodel.Attachment, download bool) {
	contentType := getContentType(att.Path)
	if download {
		// 附件下载：不缓存，避免下载旧版本
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		if att.Name != "" {
			encodedName := url.PathEscape(att.Name)
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", encodedName, encodedName))
		}
	} else {
		// 文档图片：短期缓存 + ETag=sha256，二次访问 304 免读库
		c.Header("Cache-Control", "private, max-age=3600, immutable")
		if att.SHA256 != "" {
			etag := `"` + att.SHA256 + `"`
			c.Header("ETag", etag)
			// 比较时去引号：部分客户端/代理会剥掉 If-None-Match 的引号
			if inm := strings.ReplaceAll(c.GetHeader("If-None-Match"), `"`, ""); inm != "" && strings.Contains(inm, att.SHA256) {
				c.Status(http.StatusNotModified)
				return
			}
		}
	}
	c.Header("X-Accel-Buffering", "no")
	pr, pw := io.Pipe()
	safego.Go("ServeAttachmentFromDB", func() {
		defer pw.Close()
		pw.CloseWithError(workspacerepo.NewAttachmentRepository().StreamAttachmentChunks(att.ID, pw))
	})
	c.DataFromReader(http.StatusOK, att.Size, contentType, pr, nil)
}

// serveAttachmentWithAuth 静态文件服务 + 空间权限校验
func serveAttachmentWithAuth(baseDir string, storedPrefix string, download bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		fp := c.Param("filepath")
		// Gin 的 *filepath 包含开头的 /，需要去掉，否则在 Windows 上 filepath.Join 会将其视为绝对路径
		fp = strings.TrimPrefix(fp, "/")
		if !checkAttachmentAccess(c, "/"+fp) {
			return
		}
		if att := dbAttachmentForPath(storedPrefix, fp); att != nil {
			serveAttachmentFromDB(c, att, download)
			return
		}
		fullPath := filepath.Join(baseDir, filepath.FromSlash(fp))
		rel, err := filepath.Rel(baseDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			c.JSON(403, gin.H{"error": "forbidden"})
			return
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.Header("Content-Type", getContentType(fullPath))
		if download {
			// 附件下载：不缓存，避免下载旧版本
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		} else {
			// 文档图片：启用短期缓存，切换文档可复用浏览器缓存，提升加载速度
			// token 参数变化时浏览器视为新资源重新下载，安全无副作用
			c.Header("Cache-Control", "private, max-age=3600, immutable")
		}
		c.Header("X-Accel-Buffering", "no")
		if download {
			// 查找原始文件名（DB 中存储的 name 字段），用 RFC 5987 编码支持中文文件名
			downloadName := filepath.Base(fp)
			cleaned := strings.TrimPrefix(fp, "/")
			storedPath := "attachment/" + filepath.ToSlash(cleaned)
			if att, findErr := workspacerepo.NewAttachmentRepository().FindByPath(storedPath); findErr == nil && att.Name != "" {
				downloadName = att.Name
			}
			encodedName := url.PathEscape(downloadName)
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", encodedName, encodedName))
		}
		file, err := os.Open(fullPath)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to read file"})
			return
		}
		defer file.Close()
		io.Copy(c.Writer, file)
	}
}

// serveAttachmentHeadWithAuth HEAD 方法 + 空间权限校验
func serveAttachmentHeadWithAuth(baseDir string, storedPrefix string, cacheable bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		fp := c.Param("filepath")
		fp = strings.TrimPrefix(fp, "/")
		if !checkAttachmentAccess(c, "/"+fp) {
			return
		}
		if att := dbAttachmentForPath(storedPrefix, fp); att != nil {
			c.Header("Content-Type", getContentType(att.Path))
			if cacheable {
				c.Header("Cache-Control", "private, max-age=3600, immutable")
				if att.SHA256 != "" {
					c.Header("ETag", `"`+att.SHA256+`"`)
				}
			} else {
				c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
				c.Header("Pragma", "no-cache")
				c.Header("Expires", "0")
			}
			c.Header("Content-Length", strconv.FormatInt(att.Size, 10))
			c.Status(200)
			return
		}
		fullPath := filepath.Join(baseDir, filepath.FromSlash(fp))
		rel, err := filepath.Rel(baseDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			c.Status(403)
			return
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			c.Status(404)
			return
		}
		c.Header("Content-Type", getContentType(fullPath))
		if cacheable {
			// 文档图片：启用短期缓存，与 GET 保持一致
			c.Header("Cache-Control", "private, max-age=3600, immutable")
		} else {
			// 附件下载：不缓存
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Status(200)
	}
}

func getContentType(filePath string) string {
	ext := path.Ext(filePath)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}
