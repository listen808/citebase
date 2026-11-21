# 品牌：一个名字，三个落点

> 设计侧口径。改名的历史与取舍见 [../decision-log/2026-10-07-rename-scope.md](../decision-log/2026-10-07-rename-scope.md)；
> 门禁清单见 [../engineering/one-implementation.md](../engineering/one-implementation.md)。

## 1. 三个落点，缺一不可

| 层 | 落点 | 说明 |
|---|---|---|
| 代码默认名（后端） | `internal/brand/brand.go` 的 `DefaultName` | 零依赖包；`SiteName()` 按 `main_title` → `system_name` → `DefaultName` 取值，判据只在 `ResolveSiteName` 一处 |
| 代码默认名（前端） | `web/src/utils/core/app-info.ts` 的 `APP_NAME` | 与后端默认名逐字一致 |
| 运行时站点名 | `GET /api/v1/site` → `web/src/utils/core/site-brand.ts` | 管理端可改；`description` 为空时退回语言包的一句话定位 |

**改名的顺序是后端默认名 → 前端默认名 → 库里的 `main_title` / `system_name`。** 前两个改完只影响"没配过的部署"；第三个是运行时才生效的那一处，改完必须重启并刷新站点。

## 2. 静态外壳四件：JS 起来之前就看得见

构建期定死、必须在 `APP_NAME` 改的同时一起改：

1. `index.html` 的 `<title>` 与 meta description；
2. PWA manifest 的 `name` / `short_name`；
3. 两枚图标的 `aria-label`；
4. 页面字标 / 登录页大标题 / 移动端头部 / 分享页脚（这四处取 `/api/v1/site`，不走构建期常量）。

漏掉 1–3 的症状是：页面已经显示新名，浏览器标签页和安装到桌面的图标还是旧名——`web/tests/brand-closure.test.ts` 会拦。

## 3. 品牌名不是译文

语言包里**不许**把品牌名当可翻译项：`en-US` 与 `zh-CN` 都必须原样呈现 `Citebase`。模板里的 `{app}` 占位符每个调用点都必须传参，否则界面会出现字面 `{app}`——同一个测试拦这两条。

## 4. 旧名归一：只在写入侧做一次

存量库里的旧产品名由 `repository.normalizeLegacySiteNames()` 在启动时归一：

- 清单在 `brand.LegacySiteNames()`（`AIWiki`、`AI Wiki`、`AIWIKI`、`aiwiki`、`MM-Wiki`、`MMWiki`、`mm-wiki`、`MM Wiki`）；
- **只认"整串等于旧产品名"**——用户把空间命名为 "AI Wiki 资料库" 不会被改；
- 改动行数 > 0 打 WARN 指路，不静默；
- **读取端不做替换**：否则后台显示旧名、界面显示新名，等于再造一处不一致。

## 5. 视觉上的品牌边界

- 图标是 lucide 线性图标 + 单一强调色，不做立体字标、不用渐变字（G1/G2 拦展示级大字与装饰机制）；
- 分享页/登录页可以有版面人格编排（`persona=signature`），产品内页不行（G34）；
- 品牌色不写死 hex：走 `accent` 档，浅色/深色各一个亮度。

## 6. 门禁

| 门禁 | 拦什么 |
|---|---|
| `tools/repocheck/brand_source_test.go` | 后端字面量只准一处、三份种子逐字段一致、前后端默认名一致、配置键清单一致、安装向导不许抄第二份、swagger `@title` 首词必须是 `DefaultName` |
| `web/tests/brand-closure.test.ts` | 前端默认名与静态外壳四件、语言包不许翻译品牌、`{app}` 必须传参 |
| `internal/brand/brand_test.go` | `ResolveSiteName` 的优先级与 legacy 清单纯度 |

## 7. 待办

- 下一个发布周期摘除 `AIWIKI_*` 环境变量双读，同步 `.env.example`；
- 若新增第二套品牌形态（如"企业版"），先想清楚它是**主题**还是**产品分叉**：前者走向 [theme-system.md](../engineering/theme-system.md)，后者必须先过数据模型与部署的评估，不要靠字符串替换。
