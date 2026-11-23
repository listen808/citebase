# LLM Wiki 完整度审计（2026-11-23）

结论先行：**后端与数据层基本齐全，缺的是"能用起来的四件事"——空间权限、分层、产品入口、测试与文档。**
32 个端点全都落在已实现的 handler、6 张表都进了 DDL 与迁移，但 14 个读端点没有空间成员校验、
7 个后端能力没有任何前端入口、文档里没有一篇 wiki 专题。

本文件是一次性审计，不带日期维护；随代码演进请改 `docs/engineering/codebase-map.md` 与新增专题文档。

## 一、盘点（实测）

| 维度 | 现状 | 复算 |
|---|---|---|
| 编译包 | `internal/ai/wiki` 18 个源文件 2,757 行 + 4 个测试 217 行 | `ls internal/ai/wiki/*.go` |
| API | `internal/api/ai/wiki.go` 936 行，32 个 handler | `grep -c '^func (api \*WikiAPI)' internal/api/ai/wiki.go` |
| 路由 | 32 条（11 条带 `AutoRBAC`），全部有对应 handler | `grep -c '/wiki/' internal/api/ai/routes.go` |
| 数据 | 6 张表：page / link / pin / review / lint_issue / page_history；3 模型 + 3 仓库 | `grep -c 'CREATE TABLE.*wiki' databases/table.sql` |
| 迁移 | 7 条索引进 `AllIndexSpecs`（含历史表），2 个关键列进兜底清单，1 个配置种子 | `grep -n 'aw_wiki' internal/repository/db.go` |
| 前端 | 25 个 client 函数，全部有 UI 调用；3 个设置子页 + 聊天 1 处回填 | `web/src/api/modules/ai/ai-wiki.ts` |
| 自动链路 | 分析完成两步摄入（scheduler）、源文档变更重编译（`WikiRecompileFunc`）、2-hop 图扩展、MCP 4 个工具 | `internal/ai/ai.go:212,431` |

四个自动链路是这轮最扎实的部分：调度器在分析结果落地后异步编译、源文档内容变更触发重编译、
MCP 把 `wiki_search` / `wiki_get_page` / `wiki_list_pages` / `wiki_graph` 暴露给外部 Agent。
换句话说，"内容自己长出来"这条主线是通的，"人来管、人来用"这条主线还没通。

## 二、缺口

### P0-1 读端点没有空间成员校验（跨空间可读）

`api/ai/wiki.go` 的读路径（13 个 `GET` 加 `POST /wiki/search`、`POST /wiki/article/generate`）
只做 `space_id` 参数解析，**不校验调用者是否是目标空间成员**：

```go
spaceID, _ := strconv.Atoi(c.Query("space_id"))   // wiki.go:263
if spaceID <= 0 { response.BadRequestKey(c, response.MsgSpaceIDRequired); return }
```

同一仓库里已有现成判据：`internal/api/workspace/access.go` 的 `requireSpaceMember` /
`requireSpaceReadable`（后者走 `SpaceService.CheckSpaceAccess`）。wiki 一个都没用，13 个 `GET` 里
0 个带 `AutoRBAC`，32 条路由中 11 条 `AutoRBAC` 只落在写路径。后果：任何登录用户填一个 `space_id`
就能读别的空间的编译产物。（`ExportWiki` 还会把整包页面打成 zip 返回，影响面更大。）

建议：读路径统一走 `CheckSpaceAccess`，写路径补成员校验而不是只靠路由级 RBAC。

### P0-2 没有 service 层，936 行 handler 直接编排

`WikiAPI` 直接持有 5 个 repository 并调用 `wiki.NewCompiler(...)`，缺 `internal/service/*/wiki`：

- 与 `docs/engineering/codebase-map.md` 写的"api → service → repository → model"四层不符；
- 权限、版本、幂等这类规则散在 handler 里，无法被调度器/MCP/聊天复用（现在三处各自 `NewCompiler`）；
- handler 里既有 LLM 编排又有 HTTP 绑定，单测只能起 Gin。

### P1-1 文档删除没有级联

`wiki.RemoveSource`（cascade）只被 `POST /ai/wiki/sources/remove` 调用；这个端点**前端没有任何入口**
（见 P1-3）。文档删除服务也没接它。后果：删掉源文档后，引用它的 wiki 页面与被删的 block 会留在库里，
只能人工调接口清理。

### P1-2 wiki 没进问答主链路

- 2-hop 图扩展挂在 `config.AppConfig.AI.GraphExpansion` 上，默认关（`ai.go:210`）；
- 前端识别出 `wiki_query` 意图后只加一行提示语就照常 `startStream`，没有先调 `/ai/wiki/search`
  （`AIChatView.vue:666-669` 的注释写着"优先检索已编译的 Wiki 页面"，实现里没这一步）；
- 后端 `StreamAsk` 链路也没有 wiki 页注入。

也就是说：编译产物目前只服务三个管理页、聊天里的一处"回填"与 MCP，主问答仍然只走原始 chunk。

### P1-3 7 个后端能力没有前端入口

| 端点 | 用途 |
|---|---|
| `GET /wiki/index` | 目录页（编译总览） |
| `POST /wiki/sources/remove` | 源文档级联清理 |
| `GET/PUT /wiki/purpose` | 空间编译目标（Purpose）配置 |
| `GET/PUT /wiki/schema` | 空间页面结构（Schema）配置 |
| `POST /wiki/deep-research` | 多智能体深度研究 |

25/32 = 78% 有 entry。`docs/gates` 里没有"路由无 client"的判据，所以这类缺口不会变红
（`check:api-exports` 只查"已有导出没人用"，查不到"路由没有导出"）。

### P1-4 空间靠手填 ID

`CitebaseCompileView.vue:16` 的"空间 ID"是自由输入框，`CitebaseGovernanceView` 同样。没有空间选择器、
没有"我参与的空间"下拉，与全站其它页面的空间上下文（路由/侧边栏）脱节。

### P1-5 测试只覆盖 3 个源文件

4 个测试文件（`compiler_test` / `contradiction_test` / `louvain_test` / `retriever_test`）覆盖
compiler、louvain、retriever；**15 个源文件无任何测试**：cascade、governance、lint、ppr、related、
ingest、meta、purpose、schema、writer、delivery、deep_research、multiagent、hash、graph_bridge。
前端 3 个 wiki 视图 0 个测试。

### P2-1 Swagger 只有 5/32

`wiki.go` 里 5 条 `@Router`（history / rollback / contradictions / propose-analysis / export），
其余 27 条没有注解；`internal/api/ai` 全族 8 注解 / 215 注册。这是已知欠账（`swagger_coverage_test.go`
的棘轮说明里写着"ai 族 197 条只有 1 条"），但 wiki 这一批把缺口又拉大了 27 条。

### P2-2 文档缺一篇，数字已过期

- `docs/` 里没有 wiki 专题：`grep -rl wiki docs/*.md` 只有 `architecture/rag-pipeline.md` 一句
  "2-hop wiki 扩展"和两篇改名决策记录；
- `codebase-map.md` 的"AI 模块（19 个子包）"表**漏了 `wiki/`**，还把不属于 `internal/ai/` 的
  `text2sql/` 算进这张表；
- 同篇数字全部滞后：AI Go 文件 232→**237**、rag 根级 96→**97**、text2sql 88→**91**、
  `Ap*.vue` 50→**51**（19 子包 / 96 表 / 31 设置子页仍对得上）。

### P2-3 索引与 README 过期

- `docs/README.md` 的 decision-log 树只列了 3 篇，实际 4 篇（漏 `2026-11-22-ci-and-hooks.md`）；
  `audits/`、`glossary/` 两个 README 没有入口；
- `README.zh-CN.md:105` 仍写"托管 CI 从未跑起来（`.github/` 只有 issue 模板）"，而 CI 已存在并在 2026-11-22 跑绿。

## 三、建议顺序

1. 读路径补空间校验（P0-1），用现成的 `requireSpaceReadable`；
2. 抽 `service/knowledge/wiki` 或 `service/ai/wiki`，把 936 行 handler 的规则下沉（P0-2）；
3. 文档删除接 cascade、把 `sources/remove` 接进页面（P1-1 + P1-3）；
4. 决定 wiki 是否进问答主链：要么默认开 graph expansion，要么在 `wiki_query` 分支先查 wiki 页（P1-2）；
5. 补 Purpose/Schema/Index/DeepResearch 四个管理入口（P1-3/P1-4）；
6. 补测试：cascade / lint / governance / ppr / related 优先（P1-5）；
7. 补注解与一篇 `docs/architecture/wiki-pipeline.md`，顺手修 P2-2 的数字（P2-1/P2-2）。

## 四、复算命令

```bash
ls internal/ai/wiki/*.go | wc -l                                  # 18 源 + 4 测试
grep -c '/wiki/' internal/api/ai/routes.go                       # 32 路由
grep -c 'AutoRBAC' <(grep '/wiki/' internal/api/ai/routes.go)    # 11 条写路径
grep -c '^func (api \*WikiAPI)' internal/api/ai/wiki.go          # 32 handler
grep -c 'CREATE TABLE.*wiki' databases/table.sql                 # 6 表
grep -c '@Router' internal/api/ai/wiki.go                        # 5 注解
grep -c 'export function' web/src/api/modules/ai/ai-wiki.ts      # 25 client
```

## 五、修复记录（同日）

本报告的 10 条缺口已在 2026-11-23 同批修复，验证方式与落点如下（复算命令见文末）。

| 编号 | 修复 | 证据 |
|---|---|---|
| P0-1 空间权限 | 新建 `internal/api/ai/access.go`：读走 `CheckSpaceAccess`、写要求 `privilege>=1`；13 个 GET + `POST /search` + `POST /article/generate` 全部收口；pin/review 的 `resolve` 先按 ID 查所属空间再校验 | `internal/api/ai/access.go`、`wiki.go` 每个 handler 首行 |
| P0-2 service 分层 | 新建 `internal/service/ai/wiki_service.go`（518 行，编译/发布/治理/查询/导出 + 领域哨兵 `ErrWiki*`）；`api/ai/wiki.go` 改为薄 handler（参数、权限、响应、zip 打包），handler 不再持有 repository（971 行里约 160 行是 P2-1 补的注解） | 两个文件 |
| P1-1 删除级联 | 新增 `WikiRemoveSourceFunc` 注入，文档入回收站/彻底删除均触发 `RemoveSource`；死链清理改按"已死 slug"删（原实现会误删存活页入边） | `internal/service/workspace/document.go`、`internal/ai/ai.go`、`internal/ai/wiki/cascade.go` |
| P1-2 问答主链 | `AIChatView.vue` 的 `wiki_query` 分支真正调用 `/ai/wiki/search`，命中页正文注入本轮上下文并要求 `[slug]` 标注 | `web/src/views/ai/AIChatView.vue` |
| P1-3 前端入口 | 治理页新增"编译配置与运维"卡：Purpose / Schema 读写、知识目录查看、源文档下架级联、深度研究发起；client 补齐 7 个函数 | `CitebaseGovernanceView.vue`、`ai-wiki.ts` |
| P1-4 手填 space_id | 编译页空间改为 `ApSelect`（来自"我的空间"），默认选中第一个空间 | `CitebaseCompileView.vue` |
| P1-5 测试 | `CascadePlan` 抽纯函数并补 5 例单测；导出排序与 `pathID` 参数规则各补单测 | `internal/ai/wiki/cascade_test.go`、`internal/api/ai/wiki_export_test.go` |
| P2-1 Swagger | 32 个端点全部补 `@Router`（族内 8→35），`make swagger` 重生成（paths 102→130），棘轮基线同步抬升 | `docs/swagger/*`、`tools/repocheck/swagger_coverage_test.go` |
| P2-2 文档 | 新增 `docs/architecture/llm-wiki.md`；`codebase-map.md` 补 `wiki/` 行、子包 19→20、数字按实测修正（AI Go 238 / rag 97 / text2sql 85 / Ap*.vue 51） | 两文件 |
| P2-3 索引与 README | `docs/README.md` 补 decision-log 第 4 篇、`architecture/llm-wiki.md`、`audits/` 与 `glossary/` 入口；`README.zh-CN.md` 改为"CI 已于 2026-11-22 入库，钩子仍不在树里" | 两文件 |

门禁：`go build ./...`、`go vet`、`go test ./internal/... ./tools/repocheck/...`、`gofmt -l`、
`npx eslint .`、`npm run type-check`、`npm run check:style`、`npm run check:i18n-msg`、
`npm run test`（41 文件 / 354 例）、`npm run build`、`node tools/validate-doc-links.mjs` 全绿。
