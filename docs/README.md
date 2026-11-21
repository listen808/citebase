# Citebase 文档索引

> 本文件是 `docs/` 的**唯一索引**。新增文档必须在这里登记一行；仓库没有脚本强制这件事
> （`tools/validate-doc-links.mjs` 只保证链接不指空，不保证新文档被收录）。

## 先读哪份

| 场景 | 读 |
|---|---|
| 第一次接触项目 | 根 `README.md`（产品导览 + 功能地图） |
| 要改代码 | `AGENTS.md`（命令、分层、关键文件、规矩的汇总入口） |
| 改后端 / 前端 / 主题 / 门禁 | 本目录下对应的 `engineering/`、`gates/` 文档 |
| 想知道"为什么当初这么定" | `decision-log/` 里按日期归档的决策与事故记录 |

三者的关系是：**`README.md` 是导览，`AGENTS.md` 是目录，`docs/` 是正文。**
`AGENTS.md` 里每条"详见某文档"的指针都应落到本目录的真实文件；指空即是缺陷
（2026-09-29 曾一次性删掉 12 份文档而索引未改，之后才加了链接门禁）。

## 目录

```
docs/
├── README.md                  ← 本索引
├── engineering/
│   ├── backend-conventions.md     Go 分层、响应封套、错误、RBAC、迁移、测试
│   ├── frontend-conventions.md    页面容器、版心、共享样式、组件与交互口径
│   ├── one-implementation.md      "一个概念一处实现"的收口清单
│   ├── theme-system.md            外观 × 材质、主题契约、偏好跟账号走
│   └── liquid-glass.md            液态玻璃材质的实现与禁区
├── design/
│   ├── personalization.md     个性化五档（动效/对比度/透明度/强调色/版面人格）的方案与取舍
│   └── brand.md               品牌名三个落点、静态外壳、旧名归一
├── gates/
│   ├── static-gates.md        check:style 七道静态门禁与规则族（T/G/R/I）
│   └── runtime-inspectors.md  浏览器侧运行时巡检（snap / theme-rounds / glass / i18n）
├── decision-log/
│   ├── README.md              记录的写法与命名
│   └── 2026-10-06-line-endings.md
│   └── 2026-10-07-ci-and-module-path.md
│   └── 2026-10-07-rename-scope.md
├── release/
│   ├── launch-checklist.md    上线前逐项检查（环境变量、库、代理、备份、回滚）
│   └── windows-packaging.md   Windows 便携包 / Inno Setup 安装器的构建链与坑
├── audits/                    一次性审计的人读报告（机读 JSON 不入库）
├── glossary/                  术语表（frontend-glossary.md 是 glossary-audit.mjs 的输入）
├── ui-previews/               生成物：npm run gen:theme-preview 产出
└── swagger/                   生成物：make swagger 产出
```

## 各篇一句话

| 文档 | 回答的问题 | 主要读者 |
|---|---|---|
| [backend-conventions.md](engineering/backend-conventions.md) | 新接口该写在哪一层？错误怎么返回？加表要动哪几处？ | 后端 |
| [frontend-conventions.md](engineering/frontend-conventions.md) | 页面根节点怎么摆？版心多宽？为什么没有 CSS 文件？ | 前端 |
| [one-implementation.md](engineering/one-implementation.md) | 某个概念现在唯一的落点在哪？ | 所有人 |
| [theme-system.md](engineering/theme-system.md) | 四套外观 × 两种材质怎么组合？偏好怎么跟账号走？ | 前端 / 设计 |
| [liquid-glass.md](engineering/liquid-glass.md) | 玻璃材质哪些能付 `backdrop-filter`、哪些会静默失效？ | 前端 |
| [personalization.md](design/personalization.md) | 五档个性化为什么默认 `auto`？`persona` 为什么没有 `auto`？ | 设计 / 前端 |
| [brand.md](design/brand.md) | 产品名到底存在哪几处？改名要动哪些？ | 设计 / 前端 / 后端 |
| [static-gates.md](gates/static-gates.md) | 提交前到底跑哪几道门？每道拦什么？ | 所有改前端的人 |
| [runtime-inspectors.md](gates/runtime-inspectors.md) | 哪些问题静态查不到、必须开浏览器量？ | 前端 / QA |
| [decision-log/](decision-log/) | 这条规矩的来历是什么？ | 所有人 |
| [launch-checklist.md](release/launch-checklist.md) | 上线前还有哪几项没配？ | 运维 / 发布 |
| [windows-packaging.md](release/windows-packaging.md) | 安装包为什么装在 `%LOCALAPPDATA%`？快捷方式为什么指 `start.bat`？ | 发布 |

## 文档落点规则

1. **根目录**只留"进门就要看"的：`README.md`、`AGENTS.md`、`CONTRIBUTING.md`、`SECURITY.md`、`CHANGELOG.md`、`NOTICE.md`、`LICENSE`。它们各自有机器门禁（`tools/repocheck/`）守着，挪位置会连带改门禁。
2. **`docs/`** 放展开后的细节。一篇文档只回答一类问题，不写"大而全"的合集。
3. **`web/src/styles/README.md` 是唯一例外**：它住在它描述的那个目录里，改样式的人抬头就能看到，因此不进 `docs/`。从本索引链过去即可。
4. **生成物不进 `docs/` 的索引正文**：`ui-previews/`、`swagger/` 由脚本产出，改脚本而不是改产物；`audits/` 只收人读结论，机读 JSON 由 `.gitignore` 挡在外面（例如术语审计的 `glossary-audit.json`）。
5. **一次性记录带日期**，写进 `decision-log/`（决策/事故）或 `audits/`（审计报告）；架构与约定类不带日期，随代码演进改写。
6. **改文件名必须同步改本索引**，并跑 `node tools/validate-doc-links.mjs`（等价于 `make check-docs`）。

## 校验

```bash
node tools/validate-doc-links.mjs     # 或 make check-docs：所有 markdown 的相对链接必须落地
go test ./tools/repocheck/ -count=1   # 文档相关的其它门禁（发布数字复算、脱敏、未跟踪产物引用）
```

`docs/swagger/` 是 `make swagger` 的产出物，链接检查跳过它（生成文件里的链接由生成器负责）。
