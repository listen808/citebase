# 改名范围：AIWiki → Citebase，改了什么、什么故意没改

- 日期：2026-10-07
- 状态：已定
- 影响面：产品名、模块路径、数据库、环境变量、文档、前端静态外壳

## 背景

项目原发布名 **AIWiki**，2026-10-07 起叫 **Citebase**。改名不是"全局替换"：数据模型、部署配置、历史文档里都有旧名的合法残留，一刀切会同时打断三件事——已有数据库的表前缀、现存 `.env` 的变量名、以及文档里"当时它确实叫 AIWiki"的历史事实。

## 决策：改用户看得见的，保留会断链的

**改了**

- 产品名与所有用户可见文案：页面字标、登录页、分享页、邮件预览、PWA `name` / `short_name`、`index.html` 的 `<title>` 与 meta description；
- Go 模块路径 → `github.com/listen808/citebase`（详见 [ci-and-module-path](2026-10-07-ci-and-module-path.md)）；
- 组件文件名、部署产物名（`citebase.exe`、`Citebase-Setup-<version>.exe`、镜像名）；
- 文档标题与 README 的主叙述。

**故意没改**

| 对象 | 保留值 | 理由 |
|---|---|---|
| 表前缀 | `aw_` | 改它要同时动每个模型、`internal/repository/` 的手写 SQL、`databases/*.sql`，对已有库是一次全量迁移；收益只是"看起来一致" |
| 环境变量 | `AIWIKI_*` 仍可读 | viper 与 `os.Getenv` 对认不出的键**静默丢弃**：一次性改名会让现存 `.env` / `start.bat` 全部失效，而启动日志照样打印"配置加载成功" |
| 历史文档 | 原文保留 | 决策记录的价值在于"当时怎么想"，改写它等于伪造时间线 |
| 测试用例名、`.bat` 的 echo/title、`assets/readme/*.svg` 设计稿、`web/scripts/gen-*` 生成物 | 旧名 | 测试名与终端提示不是用户界面；图片与生成物里的字进门禁只会换来"为过门禁而改"的假一致 |

应用侧因此走**双读**：`internal/utils/envname.Getenv("CITEBASE_X", "AIWIKI_X")` —— 新名优先，旧名兜底。`.env.example` 目前仍用 `AIWIKI_*` 名字（供 Docker Compose），是因为 compose 模板要兼容现存部署；旧名会在一个发布周期后摘除。

## 代价：存量库里的旧名会"第一次被看见"

改名的接线会把存量库里的脏值第一次暴露到界面上：2026-10-12 实跑 `GET /api/v1/site`，返回的是 `AIWiki` —— 改名那次没动过库里任何 `main_title` / `system_name`，因为此前没有任何界面读它们。

处理方式：

- 启动时 `repository.normalizeLegacySiteNames()` 归一，**只认"整串等于旧产品名"**（清单在 `brand.LegacySiteNames()`：`AIWiki`、`AI Wiki`、`AIWIKI`、`aiwiki`、`MM-Wiki`、`MMWiki`、`mm-wiki`、`MM Wiki`）；
- 改动行数 > 0 时打一条 WARN，告诉管理员去哪儿改回来；
- **不在读取端偷偷替换**——那样后台一栏写着 AIWiki、界面写着 Citebase，等于再造一处不一致；
- 清单内容被 `TestLegacySiteNamesAreNothingButOldBrand` 钉住：混进 `Wiki` 这类泛词就红。

## 现在的唯一落点

| 概念 | 落点 |
|---|---|
| 后端默认名 | `internal/brand/brand.go` 的 `DefaultName` |
| 前端默认名 | `web/src/utils/core/app-info.ts` 的 `APP_NAME` |
| 运行时站点名 | `GET /api/v1/site` → `web/src/utils/core/site-brand.ts` |
| 旧名清单 | `brand.LegacySiteNames()` |

三条门禁守着：`tools/repocheck/brand_source_test.go`（后端字面量只准一处、三份种子逐字段一致、前后端默认名一致、配置键清单一致、安装向导不许抄清单、swagger `@title` 首词必须是 `DefaultName`）、`web/tests/brand-closure.test.ts`（前端同两条 + 语言包不许把品牌名当译文 + `{app}` 每个调用点必须传参 + 静态外壳四件）、`internal/brand/brand_test.go`。

## 后续

- 下一个发布周期摘除 `AIWIKI_*` 双读，届时同步更新 `.env.example` 与本目录的记录；
- 若将来要动表前缀，按"全量迁移 + 停写窗口"单独立项，不要夹在品牌类改动里。
