# 运行时巡检（浏览器侧）

> 静态门禁读源码，这一批**开真浏览器量渲染结果**。共同前提：前端 dev server 在 `:3000`、后端在 `:3001`。
> 静态规则见 [static-gates.md](static-gates.md)。

## 1. 为什么必须另开一批

有些问题在源码里看不见：

- 一屏**同时**叠了几层实时模糊（`v-for` 会把一处声明放大成二十层，实测过 23 层）；
- 玻璃压玻璃的嵌套深度；
- 滚动到顶栏底下时，壳面到底加厚了没有；
- 主题"切了"但材质轴没真正改变渲染（2026-10-16 实测八轮全跑在默认档上，报表却标成玻璃档）；
- 英文界面里"看得见的中文"，而其中大半是用户自己写的数据（空间名、文档标题），本就不该翻；
- 页面跑起来直接抛异常 / 白屏 / 落到 ErrorBoundary。

## 2. 工具与用途

| 命令 | 脚本 | 量什么 | 需要 |
|---|---|---|---|
| `npm run snap` | `visual-snapshot.mjs` | 遍历路由逐页全页截图 + 采集控制台/网络异常 | `SNAP_USER` / `SNAP_PASS`，浏览器 |
| `npm run check:runtime` | `check-runtime-ego.mjs` | 轻量冒烟：路由打得开、没落 ErrorBoundary、顺带记录可见中文（只报告） | 本机已登录的 ego-lite |
| `npm run check:runtime:pw` | `check-runtime-errors.mjs` | 四类运行时错误：未捕获异常、console error、请求失败、非 2xx/3xx（5xx 计红） | Playwright + 凭据 |
| `npm run check:glass:runtime` | `check-glass-runtime.mjs` | 同屏模糊层数（基线 16）、玻璃嵌套深度、滚动边缘是否触发 | 凭据；默认跑满四档玻璃轮 |
| `npm run check:theme-rounds` | `check-theme-rounds.mjs` | 八档外观×材质的四条不变量（见 §4） | 凭据；`--themes=all` |
| `npm run check:i18n-runtime` | `check-i18n-runtime.mjs` | 英文档下逐路由量可见中文，并按 `/api/` 响应体把"代码文案"与"用户数据"分开 | 前后端 + 凭据 |
| `npm run check:geom` | `check-layout-geometry.mjs` | 布局几何：间距、对齐、基线 | 前后端 + 凭据 |
| `npm run check:detail` / `check:parity` / `check:ui` | 对应脚本 | 页面细节一致性、页面间一致性、UI 规范 | 前后端 + 凭据 |

另有一批 Playwright 量测脚本（`check-element-tokens`、`check-dialog-layout`、`check-motion-consistency`、`check-gesture-dynamics`、`check-box-census`、`check-visual-signature`、`check-long-content`、`check-apple-design` 等），用法相同，改到对应区域时按需跑，不必每轮全跑。

## 3. 凭据与产物

- 口令**只从环境变量取**：`SNAP_USER` / `SNAP_PASS`，脚本里不写默认值。缺变量直接退出（`visual-snapshot.mjs` 退出码 2）——脚本会进仓库，写死账号等于把凭据推到远端。
- 路由夹具（空间/文档 id）由 `lib/route-list.mjs` 解析后写成临时文件；ego 的 Node 运行时拿不到 shell 环境变量，也不需要拿。
- 产物目录按**脚本位置**解析，不按 `cwd`：原先按 `process.cwd()` 算，在 `web/` 下跑会把 4.7 MB 截图写进 `web/docs/ui-snapshots`（`.gitignore` 只挡仓库根那份），git 看到的是未忽略的垃圾目录。
- 截图产物落在 `docs/ui-snapshots`（已被 `.gitignore` 忽略）；ego 冒烟落在 `docs/ui-ego`。

## 4. `check:theme-rounds` 的四条不变量

它不是"把每台工装 × 8"，而是专门的轮检：**每档都跑、每档只看少数几页**，判只有"换了主题"才成立的：

1. **这一轮真的切过去了**——DOM 上的 `data-theme` / `data-material` / `data-glass` 必须等于本轮请求的那一档（地基；没有它，其余三条测的是同一档四次）；
2. **材质轴真的改变渲染**——同外观的 `solid` 与 `glass` 轮，顶栏算出的背景必须不同，玻璃轮还得真带 `backdrop-filter`；
3. **浮层档在每个主题都落在自适应区间**（alpha ∈ [0.40, 0.94]）；
4. **每档的正文/壳面对比度过线**——按浏览器算出来的值算，与静态 `check:contrast` 不一致说明有东西在运行时覆盖了令牌（自适应加厚就是一个覆盖者）。

为什么不用全量 × 8：51 条路由 × 8 档 = 408 次加载，还要撞 `apiLimiter` 的 300/分钟。

## 5. 限流与跑法

- 全站限流 300/分钟/IP；`/auth/refresh` 独占 30/分钟/IP/路径；登录等口令猜测面 10/分钟/IP/路径。**一轮全量巡检很容易撞到 refresh 档**，脚本用 `--only` / `--max` / `--settle`（默认 1200ms）/ `--gap`（默认 2500ms）控制节奏。
- 撞 429 不是"页面坏了"：响应带 `Retry-After`，前端 `error-presentation.ts` 会显示"操作太频繁了"并倒计时。巡检报表里出现 429，先看是不是自己跑太快。

## 6. 用法约定

- 改完界面想立刻看一眼：`npm run check:runtime`（ego-lite，省冷启动与登录）；
- 要进流水线或需要截图存证：Playwright 那套；
- 动主题/材质：`npm run check:theme-rounds -- --themes=all`；
- 动玻璃层/壳面：`npm run check:glass`（静态普查）+ `check:glass:runtime`；
- 动文案/i18n：`npm run check:i18n-runtime`，看 `code` 与 `data` 两栏，**只有公开路由设硬门槛**；
- 任何浏览器能看见的改动：至少 `snap` 一页 + `check:geom`。

## 7. 判定与降噪

- `check-runtime-errors` 的门禁 = 未捕获异常 + console error + 请求失败 + 5xx；warn 与 4xx 只登记（预认证 401 探测是合法的），需要人工分真伪，不一刀切判红。
- 第三方编辑器（Monaco / CodeMirror）自带的 DOM（`z-index:-1`、`#000` 文本）不计入本项目的样式账。
- 玻璃普查"扫不到任何站点"一律判失败——**扫不到不等于没有问题**，这是这条工装存在的理由之一。
