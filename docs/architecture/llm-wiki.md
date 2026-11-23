# LLM Wiki 知识编译子系统

> 把"一堆文档"编译成"一张知识网"的子系统：文档 → 实体/概念页 → 交叉引用图 → 治理队列。
> 口径：这是产品内的 Wiki（`/ai/wiki/*`），**不是** GitHub wiki，也不是 `docs/` 文档站。

## 它解决什么问题

RAG 每次提问都要重新拼片段，答案之间没有积累。Wiki 把"已经想清楚的知识"落成页面：
再次提问先读编译产物（目录 + 全文），只有读不到时才回退到 chunk 检索。页面带来源、
版本、置信度和取代链，所以知识可追溯、可裁决、可导出。

## 分层

| 层 | 位置 | 职责 |
|---|---|---|
| API | `internal/api/ai/wiki.go` | 参数校验、空间权限收口（`internal/api/ai/access.go`）、响应封套、zip 打包 |
| Service | `internal/service/ai/wiki_service.go` | 编译、发布、治理、查询、导出的业务规则；领域哨兵错误 `ErrWiki*` |
| 编译内核 | `internal/ai/wiki/` | Plan/Generate/Polish、引用校验、Louvain 社区、pin/lint/review、级联 |
| Repository | `internal/repository/knowledge/wiki*.go` | `aw_wiki_page` / `_link` / `_pin` / `_review` / `_lint_issue` / `_page_history` |

Service 签名里没有用户维度（与仓库既定口径一致：越权只能在 handler 层收口），
所以权限判定集中在 `access.go`：读走 `CheckSpaceAccess`（公开空间放行），写要求
`GetUserSpacePrivilege >= 1`；超级管理员天然是 2。

## 主链路

1. **触发**：调度器分析完成后异步编译；`IsImmutable==1` 的文档内容变更走
   `WikiRecompileFunc` 重编译；文档入回收站/彻底删除走 `WikiRemoveSourceFunc` 级联。
2. **Plan**：候选页串行去重（同名合并、来源合并），产出 `created/updated/skipped`。
3. **Generate**：并发生成草稿（`draft`），写入压缩日志、刷新 overview、算置信度；
   不自动发布。
4. **Publish**：乐观锁（`base_version`）发布，基线不匹配要求重新生成草稿（rebase）。
5. **查询**：`BuildIndex` 确定性目录 ∪ 全文检索双路召回；聊天里的 `wiki_query`
   意图命中后把页面正文注入本轮上下文，并要求按 `[slug]` 标注出处。
6. **治理**：
   - **pin**：人工确认的事实锚点，重编译后复核（放行 / conflict / orphan）；
   - **lint**：孤立页、稀疏社区、死链、过时内容；
   - **review**：异步审核队列，批准时同样走乐观锁发布；
   - **supersede**：取代链，人工页不取代、已取代页不重复取代。
7. **导出**：OKF 风格 markdown；`zip` 打全部页 + `links.md` 交叉引用表，`md` 导单页。

## 级联规则（删文档时最容易被误解的地方）

- 只被该源贡献的页面 → 整页删除；
- 多源共享的页面 → 只摘掉该源的溯源条目，页面保留（否则删一份文档会抹掉别人贡献的知识）；
- `provenance=human` 的页面永不被级联删除；
- 删除后按"已死 slug"清理死链（从死页出发的边也要清，不能只清指向死页的边）。

决策部分是纯函数 `CascadePlan`，单测覆盖四种分支；DB 操作在 `RemoveSource` 里。

## 配置与开关

- `wiki.multiagent.enabled=0`：默认关闭多智能体编排；打开后深度研究走
  `MultiagentOrchestrator`，关闭时退化为单轮汇编。
- Purpose / Schema 按空间存储：Purpose 参与编译与检索选页，Schema 追加进编译
  system prompt；两者空串表示"不限定/不追加"，可在治理页直接编辑。
- `config.AppConfig.AI.GraphExpansion`（`graph_expansion`）默认 `false`：
  打开后问答在检索阶段做 2-hop wiki 扩展。

## 前端入口

- 设置页：`wiki-compile`（`CitebaseCompileView.vue`，空间从"我的空间"选，不再手填 ID）、
  `wiki-write`（`CitebaseWriteView.vue`）、`wiki-governance`（`CitebaseGovernanceView.vue`：
  pin/review/lint/links 四个 tab + 编译配置与运维卡）。
- 聊天页：`AIChatView.vue` 的 `wiki_query` 分支真正调用 `/ai/wiki/search` 并回填正文。
- 客户端：`web/src/api/modules/ai/ai-wiki.ts`，全覆盖 32 个端点。

## 已知边界

- 编译是同步执行的（在请求内完成 Plan+Generate），大文档集会占用请求时长；生产建议走调度器。
- 前端无自动化测试，只有视图级类型检查与门禁；后端只有 `internal/ai/wiki` 纯函数单测，
  service/DB 路径依赖集成环境。
