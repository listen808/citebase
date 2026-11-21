# 一个概念，一处实现

> 本仓的"收口清单"。判断标准只有一句：**同一个概念，全仓只能有一份真值；第二份实现出现时，门禁必须变红。**

## 为什么要有这份清单

这个仓库最贵的 bug 不是"代码写错"，而是"同一件事存在两份，且两份慢慢分叉"：

- 产品名在后端有三处种子、前端有一份 `APP_NAME`，改名那次漏掉了库里的 `main_title`，于是 2026-10-12 界面显示的是旧名；
- 后端把中文直接写进响应（245 处、去重 123 条），前端只有 `err.message` 一根管子，英文界面必然读到中文，而且前端修不了；
- 收藏开关历史上出现过四五种写法（斜线星、红色"取消"、`fill: currentColor`），用户看到的是"这颗星怎么跟别处不一样"；
- 门禁本身也会"写两份判据"：静态门禁按令牌算对比度，运行时门禁按浏览器算，两边不一致时说明有东西在运行时覆盖了令牌。

所以每条收口都配了机器判据，而不是靠 review 记仇。

## 文案与品牌

| 概念 | 唯一落点 | 谁在守 |
|---|---|---|
| 后端对用户说的话 | `internal/response/messages.go`（键 → 中文原文） | `internal/response/messages_test.go`、`tools/repocheck/server_message_test.go`、`web/tests/server-messages.test.ts` |
| 前端拿到的译文 | `web/src/i18n/locales/server-messages.ts`（**生成物**，源在 `web/scripts/server-messages.en.json`） | 生成物过期即红 |
| 产品名（后端默认） | `internal/brand/brand.go` 的 `DefaultName` | `tools/repocheck/brand_source_test.go`、`internal/brand/brand_test.go` |
| 产品名（前端） | `web/src/utils/core/app-info.ts` 的 `APP_NAME` | `web/tests/brand-closure.test.ts`（含静态外壳四件） |
| 站点名/描述的运行时值 | `GET /api/v1/site` → `utils/core/site-brand.ts` | 前后端键清单一致性 |
| 旧产品名清单 | `brand.LegacySiteNames()`，启动时 `normalizeLegacySiteNames()` | `TestLegacySiteNamesAreNothingButOldBrand`（混进泛词即红） |

## 主题与样式

| 概念 | 唯一落点 | 谁在守 |
|---|---|---|
| 主题清单 | `web/src/styles/theme/themes.ts` 的 `THEME_REGISTRY` | T3（`THEME_REGISTRY` / `THEME_IDS` / `EXTENDED_THEME_IDS` 三者一致） |
| 主题契约 token | 同文件的 `THEME_CONTRACT_KEYS` | T1/T2（缺一套即编译不过） |
| 个性化档位（行为） | `web/src/styles/theme/personalization.ts` | `check:design` G32、`check:theme-rounds` |
| 强调色色板 | `web/src/styles/theme/accents.ts` | `check:contrast`（全色板一次过） |
| 个性化档位的服务端校验 | `internal/model/user/preferences.go` | `tools/repocheck/preference_contract_test.go`（逐条比对前端清单） |
| 动效时长 | `apple-tokens.ts` 的 `BASE_DURATION_MS` → `--apple-dur-*` | G32（不许写死秒数） |
| 缓动曲线 | `AppleEasing` | G30 |
| 字距 | `trackingFor(px)` | G15（不许逐条目手写） |
| 表面阶梯 | `surface-chrome / canvas / panel / elevated / float / field`（令牌层） | T12（一种颜色只有一个名字，别名不得回潮）、R12/R13 |
| 壳层加厚时机 | `composables/ui/useScrollEdge.ts` 的 `shellSurfaceKey()` | `check:glass` |
| 网格地板 | `AppleGridFloor` 四档 | R10/R11、`appleFixedColumns()`（R17） |

## 交互组件

| 概念 | 唯一实现 | 谁在守 |
|---|---|---|
| 页面骨架 | `ApPageContainer` | R1（页面必须用它） |
| 面包屑 | `ApBreadcrumb` | R7/R8（页面手搓即红） |
| 弹窗 | `ApModal` + `AppleModalSize` | G29（不许传 `width`） |
| 进度条 | `ApProgress` | G22 |
| 分段控件 | `ApSegmentedControl` | G12（同档同高） |
| 收藏开关 | `ApFavoriteButton` | G14 |
| 菜单/浮层表面 | `menuSurfaceStyle()` | G10 / T12 |
| 图标 | `ApIcon` + `icons.ts` 注册表 + lucide | I1/I3/I7/I9 |
| 表格 | `ApTable`（`density` 而非 `size`） | R18 |
| 浮动定位 | `composables/ui/` 的浮层工装 | G29 / 布局几何巡检 |

## 工程与文档

| 概念 | 唯一落点 | 谁在守 |
|---|---|---|
| 文档索引 | `docs/README.md`（**只此一份**，其他地方引用它） | `make check-docs` 管链接；"新文档是否登记"靠人守 |
| 后端分层 | API → Service → Repository → Model | 评审 + 包布局约定（`model/` 无 `auth/`，根级 `.go` 只在 `repository/`） |
| 集成边界 | `app/server/main.go` 的 `InstallXxx` 注入 | `brand` 零依赖就是为这条立的样本 |
| 版本号 | `internal/version/version.go` 的 `Version` const + `web/package.json` | `version_test.go` 逐处比对 |
| 备份/回灌 | `internal/utils/dbdump` | 两处调用方共用（CLI 与后台备份） |
| 后台 goroutine | `internal/utils/safego.Go` | `goroutine_guard_test.go`（裸 `go func` 只降不升） |
| 出站 HTTP | `internal/utils/netguard` | 拨校验过的 IP，防 DNS rebinding |
| DDL | `databases/table.sql`（`go run ./tools/genddl` 产出） | `migrate_columns_test.go`、`genddl` |
| 配置种子 | `repository.DefaultConfigSeeds()` | `brand_source_test.go`（安装向导不许抄第二份） |

## 棘轮（只降不升的存量基线）

收口不是一次清干净，而是"存量登记、新增零容忍"。当前基线都在文件里写着，整改一处就下调一处：

| 基线 | 当前值 | 位置 |
|---|---|---|
| 手写样式对象与组合层重复 | 127 | `check-style-tokens.mjs` 的 `localStyleClones` |
| `AppleSpace['Npx']` 使用点 | 22 | 同上 `spacePixelKeys` |
| `AppleRadius['Npx']` 使用点 | 15 | 同上 `radiusPixelKeys` |
| 像素键注册表定义 | space 31 / radius 20 | 同上 |
| 后端直出中文（未迁键） | 0 | `serverMessageUnmigratedBaseline` |
| 中文写进数据字段 | 42（只许降） | `serverMessageFieldBaseline` |
| 裸 `go func` | 递减 | `goroutine_guard_test.go` |
| API 泄漏原文（`Fail` 残留） | 递减 | `internal/api/error_leak_test.go` |

## 新增一个概念时的流程

1. 先搜：已有落点能不能表达？能复用就复用，差异用参数（`crudStatsRowStyle('compact')`）而不是复制函数；
2. 确属新概念：在**对应层**新建唯一实现，并在本清单登记一行；
3. 同时写门禁：注入一条违例，确认它变红，再撤回（别用 `git checkout`，会连未提交的工作一起丢）；
4. 如果旧实现还有存量，先登记基线，再逐处收编，**不要一次性大改**——那会把"收口"变成"重写"。
