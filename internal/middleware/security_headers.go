package middleware

import "github.com/gin-gonic/gin"

// securityHeadersCSP 是主应用的 Content-Security-Policy。
//
// 为什么要 CSP（见 docs/decision-log/2026-11-07-auth-cookie-and-csp.md）：
// 登录态 cookie（auth_token）是 JS 可写的非 HttpOnly 档——<img>/附件请求发不了
// Authorization 头，靠 cookie 兜底（extractToken 的第三优先级）。它的主要风险面
// 是 XSS 读到 cookie 后跨站重放，CSP 是 XSS 的最后一道闸门。
//
// 各指令的口径（2026-11-07 按 web/index.html 与全仓 web/src 实测定死）：
//   - default-src 'self'：基线自域。index.html 没有内联 <script>（只有内联
//     <style>），入口是 /src/main.ts 的 module script；script-src 不开
//     'unsafe-eval' 的前提是前端构建定义了 __INTLIFY_JIT_COMPILATION__=true
//     （web/vite.config.ts）：vue-i18n 默认的消息编译器会用 new Function，
//     CSP 一挡，bootstrap() 在 app.mount() 前抛 EvalError → 整页白屏。
//   - style-src 'self' 'unsafe-inline'：index.html 有内联 <style> 基底重置，
//     且全站是 :style 内联绑定（没有 CSS 文件）——'unsafe-inline' 是既成事实，
//     不是放宽。style 注入不构成 XSS 面（style-src 挡的是 <style> 标签注入，
//     挡不住 :style 属性，两者都不执行脚本）。
//   - img-src 'self' data: blob:：站内图片 + 文档预览的 data: 内联图 +
//     前端七处 createObjectURL 的下载中间产物。
//   - media-src 'self' blob:：语音输入（useVoice.ts）把识别结果挂成 blob: 播放。
//   - connect-src 'self'：API 与 WebSocket 都走同源（开发期 Vite 代理，生产期
//     同一后端），AI 请求由后端代发，前端不直连第三方。
//   - font-src 'self' data:：设计系统字体走自域，data: 留一格给内嵌的 woff2。
//   - object-src 'none'：全站没有 <object>/<embed>。
//   - base-uri 'self'：挡 <base> 劫持相对 URL 解析。
//   - form-action 'self'：表单只提交到自域。
//
// 刻意不写 frame-ancestors / X-Frame-Options（全局档）：
// 嵌入 widget（internal/ai/rag/embed_widget.go）要允许**任意站点**把 AI 助手
// 挂进自己的页面，全局锁 frame 会把它打死。widget 那份 HTML 自己带
// "frame-ancestors *"，自域页面的点击劫持残余风险记在 decision-log 里，
// 与 widget 产品定位一起权衡，不在这条中间件里单方面收紧。
const securityHeadersCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'self'; form-action 'self'"

// SecurityHeadersMiddleware 给所有响应补上安全响应头。
// 与 CORS 的分工：CORS 管"跨域资源访问"，这里管"浏览器怎么对待这份响应本身"。
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", securityHeadersCSP)
		// 响应体 Content-Type 由路由自己定；这条头是双保险，
		// 挡浏览器把 .js/.svg 猜成可执行内容（MIME 嗅探型 XSS 的老入口）。
		c.Header("X-Content-Type-Options", "nosniff")
		// 登录态在 cookie 里，Referer 泄露完整 URL（含 ?token= 兜底通道）
		// 等于把凭证送到第三方页脚。same-origin：只发 origin，不发 path+query。
		c.Header("Referrer-Policy", "same-origin")
		c.Next()
	}
}
