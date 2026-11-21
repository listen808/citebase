# 更新日志

> 本文件由 `node tools/gen-changelog.mjs` 从 git log 生成，**不要手工编辑**——
> 手写的条目会在下一次发布时被整体覆盖。要解释"为什么这么改"请写进提交信息或
> [`docs/decision-log/`](docs/decision-log) 的决策记录，那里才是不会过期的地方。
>
> 覆盖范围：`v1.0.0..HEAD`，共 171 个非 merge 提交。

## 新增

- feat(brand): 项目改名 AIWiki → Citebase（模块路径、组件文件名、部署产物名一起迁）（2026-10-07 · `5fe05051`）
- feat(version): 版本号收成单源并接进 /health 与管理端诊断（四条门禁 + 前端注入）（2026-10-07 · `b13aa556`）
- feat(repo): Go module 路径迁到 github.com/listen808/AIWiki（2026-10-07 · `348084e3`）
- feat(web): 品牌标记重做（摊开的书 + 上升脊 + 节点）+ ApBrand 单一源（2026-10-04 · `d6493299`）
- feat(web): 接入 morphicons 图标形变 + 全站图标一致性加固 + 新门禁（2026-10-04 · `35fa4e8a`）
- feat(share): 公开分享页补 rel=canonical，受保护那一支不给（2026-10-03 · `7c2cd8e0`）
- feat(web): 面包屑并成一处实现，容器那份手写 nav 删掉（R8 反向设防）（2026-10-03 · `2d30a72f`）
- feat(web): 缓动曲线 / spinner / 骨架屏各收成一处，新增 G30 拦手抄曲线（2026-10-03 · `cb248b44`）
- feat(web): grid 三种写法收口 —— 新增 R17 补上 R10 看不见的一族（2026-10-03 · `044280ee`）
- feat(web): 两套外壳的工具条顺序并轨，并把「数行」做成门禁 B9 + 弹窗宽度 G29（2026-10-03 · `9bd9cfb4`）
- feat(web): 页头下三行 chrome 收成一行工具区（11 页，用户截图指出「红框太浪费空间」）（2026-10-03 · `d7eb3918`）
- feat(build): Windows 安装包与免安装包同源产出（2026-10-03 · `5ff9aa19`）
- feat(web): 九处「按钮颜色当选中态」并回 ApSegmentedControl，删掉 pill variant（2026-10-03 · `1e74430a`）
- feat(web): 图片灯箱与全屏遮罩各收成一处实现，补 G26/G27 两条静态门禁（2026-10-03 · `0b302545`）
- feat(web): 三个可拖走的浮层换成同一份弹簧物理，新增 check:gesture 验收台（2026-10-03 · `cd251bae`）
- feat(web): 新增 surface-primary 主对象档，材质那根轴开始表达层级（2026-10-03 · `3caa0296`）
- feat(web): 页头抬到 24px 顶档 —— 层级不再只靠间距一根轴（2026-10-03 · `a1348dc7`）
- feat(share): 分享页 og 由服务端注入且默认不说标题；顺带堵掉输密码前就泄标题的既存泄漏（2026-10-02 · `2f470e0b`）
- feat(ai): 可观测性追踪面板改用专用列表端点，从 10 条放开到 200 条（2026-09-30 · `be3f5214`）
- feat(ai): 重复检测结果可生成合并方案，并修一个必然 400 的 API 包装（2026-09-30 · `2a18c917`）
- feat(ai): 新增 Wiki 治理页，接上引脚/待审/体检/链接九项既有能力（2026-09-30 · `105bbe12`）
- feat(web): 空间删除前给影响范围，MCP 服务器可展开工具清单（2026-09-30 · `67f94ef8`）
- feat(ai): AI 配置页接上分析器限额与自动调度（2026-09-30 · `ed241b94`）
- feat(auth): OIDC 提供方管理页，并把初始化失败从日志里捞出来（2026-09-30 · `6a4bc68b`）
- feat(ai): 接上记忆管理页，并修统计漏算 user_feedback（2026-09-30 · `5dd4e590`）
- feat(search): 补上搜索统计底账 aw_search_log 与索引监控接口（2026-09-30 · `9226fbc8`）
- feat(web): 收藏开关收敛为 ApFavoriteButton 一处实现，星标位置由卡片操作位定死（2026-09-30 · `7ebbbdad`）
- feat(web): 补全站页脚、图标提示统一走气泡、认证守卫区分限流与凭据失效（2026-09-30 · `eb0eed83`）
- feat(web): 设计系统加 data-ap 身份锚点，术语巡检配适用性声明（2026-09-29 · `bdf2d17b`）
- feat(web): 主题契约化——五套主题统一差异表 + 全元素令牌收口（2026-09-28 · `c0488e89`）
- feat(web): 全元素主题匹配——面板/阴影基础设施 + 逐主题签名完善（2026-09-28 · `b690ea3d`）
- feat(web): 液态玻璃背景光晕 + 列表交错入场动效（2026-09-28 · `e8a945e1`）
- feat(web): 主题材质层——液态玻璃模糊与主题化圆角（2026-09-28 · `f6bcc4e8`）
- feat(web): 扩展主题系统——新增液态玻璃/纸感阅读/杂志排版三套主题（2026-09-28 · `a02264f5`）
- feat: 对齐 2026 主流 LLM Wiki 的检索/评测/权限/可观测性增强（P0+P1）（2026-09-28 · `24121575`）
- feat(gate): 新增布局契约门禁 check:layout，并串进 check:style（2026-09-27 · `98b3e7b6`）
- feat(scripts): 旧库 v1(mw_) → v2(aw_) 迁移与核对脚本（2026-09-27 · `36af8a0a`）
- feat(ai-import): 接入连接器抓取/解析端点，并把导入改成真正落库（2026-09-27 · `02dbf770`）

## 修复

- fix(brand): 模块路径第三跳迁到现实仓库位置 github.com/listen808/citebase（2026-10-08 · `ff91e8a0`）
- fix(deps): 前端 high 级漏洞清零（vue 3.5.43 / vite 8.3.3 / axios 1.20.0 / nanoid / postcss）（2026-10-07 · `c4d2f55d`）
- fix(docker): 镜像补上中文分词词典，并加一份跨三份部署文件的挂载点门禁（2026-10-07 · `aff46f9b`）
- fix(deploy): 文档要求的加密密钥透传补上，并加 compose ↔ .env.example ↔ os.Getenv 的双向门禁（2026-10-07 · `d9d5e780`）
- fix(web): 品牌预览生成器里的无用转义（它让 lint:check 挂了 3 个提交）（2026-10-06 · `7d69f0af`）
- fix(config): 模板重复键导致全新安装起不来，并加两条常驻门禁（2026-10-06 · `e16d6c7e`）
- fix(backend): 项目质量审计批量修复（提权/并发崩溃/静默失败/SSRF/关停）（2026-10-06 · `da65d3e8`）
- fix(web): 分段控件药丸外盒回到控件尺（small 档 48→40）（2026-10-06 · `3ebe4884`）
- fix(web): 工具条分段控件统一到 small + check:geom 真正置码（2026-10-06 · `1f40744d`）
- fix(web): 前端一致性审计批量修复（弹窗栈/字号表/浮层阶梯/静默失败）（2026-10-06 · `61cbffca`）
- fix(backend): 文档 path 接口不存在时返回 404 而非 500（2026-10-06 · `34572835`）
- fix(web): 知识库卡片选中态去掉用阴影当第二套选中语言（2026-10-05 · `4f1af89e`）
- fix(web): 锁文件 tarball 主机归一到 npmjs，消除 npm 12 EALLOWREMOTE（2026-10-05 · `42f54773`）
- fix(web): 文档 meta 行作者名不换行 + 巡检登记固定标签（2026-10-05 · `e440b44f`）
- fix(web): 修好 AI Widget 预览 iframe 并在巡检里清零运行时错误（2026-10-05 · `7fd515b0`）
- fix(web): 移除 lucide 运行时依赖，形变图标几何改内联（2026-10-05 · `e36f33a2`）
- fix(web): 删掉模块顶层的空 onUnmounted，消除登录页 Vue 告警（2026-10-05 · `a2a67d60`）
- fix(web): 顶栏帮助下拉菜单缩放原点朝触发元素（P1d 部分）（2026-10-04 · `15e6f99a`）
- fix(web): 记住登录 checkbox 布尔收敛 + 字段族 token 统一（P2）（2026-10-04 · `a4a3935e`）
- fix(web): vite optimizeDeps 收录 lucide + morphicons/vue（2026-10-04 · `0ed9c003`）
- fix(web): 交互一致性审计 P0 —— 功能与无障碍止血（2026-10-04 · `eebb1978`）
- fix(web): 合成探针也先 settle 再量，B2 命中附带元素坐标（2026-10-03 · `31e391bc`）
- fix(links,rate-limit): 点击量补上真实数据源，429 不再把整页崩成"未知错误"（2026-10-03 · `5015cb77`）
- fix(web): 修掉 G19/R11 两处"永远绿"，被它们藏住的死码现形并删除（2026-10-03 · `87fce483`）
- fix(build): DATABASE-SETUP.sql 是随包模板却不是被跟踪文件，缺了还照打 [OK]（2026-10-03 · `22f394e9`）
- fix(web): 浮层退路收成一处 useClickOutsideClose，登录页主题从 2 套补回全 5 套（2026-10-02 · `8ae919b0`）
- fix(web): 10 份手写浮层表面并成 menuSurfaceStyle() 一张 —— 暗色下 4 个菜单本来没有可见边缘（2026-10-02 · `af14fcb7`）
- fix(web): 15 处手搓进度条收进唯一的 ApProgress —— 高度/圆角/轨道底/色档/可访问名一次并档（2026-10-02 · `d7b992f7`）
- fix(web): 批 C 收尾 —— 删三个被再导出撑着的 hero 死导出、AIWriterPanel 空态并回 ApEmpty（2026-10-02 · `47a5801d`）
- fix(web): 批 C 高频概念并档 —— Card 圆角/内边距、标签档、label 唯一来源、光标与自绘框（2026-10-02 · `b5c7e38a`）
- fix(web): 修 18 条行为与语义缺陷 —— 触屏拖得动了、Toast 不再静默丢失、浮层接得住 Esc（2026-10-02 · `8450fc31`）
- fix(web): 动效令牌只准动画合成器属性 —— 全站 2000+ 元素不再随时准备动布局（2026-10-02 · `ecebb262`）
- fix(web): 工具栏同形态行按中心对齐 —— 矮组的 label 不再被底对齐拽下去（2026-10-02 · `f6250775`）
- fix(web): 筛选组内不许换行 —— label 被折到控件上方会让一行高低不齐（2026-10-02 · `3fad7bec`）
- fix(web): 顶栏没有 data-ap 锚点，在全部运行时工装里是隐形的（2026-10-01 · `a8aa6a3e`）
- fix(web): 搜索框上下各 11px 对指针是死的，外壳点击也从来没转发（2026-10-01 · `63b901a1`）
- fix(web): 阅读页两个右下角悬浮按钮，一个 44 一个 36（2026-10-01 · `81b57e07`）
- fix(web): 工作流编辑器两个标签手写字重，借了字号档却没借字重档（2026-10-01 · `49502fa6`）
- fix(web): 5 个图标按钮的可访问名是一串源码文本（2026-10-01 · `2cd07a69`）
- fix(web): 按下即反馈补成全站默认，浮层动效全部回到令牌档位（2026-10-01 · `4dea5fcb`）
- fix(web): 新建记录类文案归一成「新建 X」，添加只留给挂接已有东西（2026-10-01 · `f01fe893`）
- fix(web): 长内容一变长就现形的四处排版缺陷，外加滑块只有 4px 可点（2026-10-01 · `ee6b9dda`）
- fix(web): 补上 html 根字体——Teleport 出去的 modal/toast 一直在渲染衬线体（2026-10-01 · `1728b98d`）
- fix(web): 三页的卡片标题从 h3 升为 h2，补回被跳掉的标题层级（2026-10-01 · `ef3d8ad8`）
- fix(web): 窄视口三处布局事故——触控档不一致、正文被裁、截断丢信息（2026-10-01 · `d9c080a0`）
- fix(web): 元素级几何四处偏差——开关撑高、间距凑数、数值列不对齐（2026-10-01 · `8b7d2c27`）
- fix(api): 认证被限流不再当成凭证失效把人踢下线（2026-10-01 · `52863bce`）
- fix(web): 排版天阶收口——同类元素全站只留一档（2026-10-01 · `197f6ff6`）
- fix(web): OIDC 页空态 props 传错组件；巡检路由清单补上新页（2026-09-30 · `fbc998e4`）
- fix(web): ApTable 空态把 display:flex 加在 td 上，导致 colspan 失效（2026-09-30 · `1f7ed176`）
- fix(scripts): 术语巡检的字数诊断此前在剥字母 s 而不是剥空白（2026-09-30 · `c31ef5d9`）
- fix(web): 顶栏/页脚/侧栏用户行压在同一个壳层标尺上（2026-09-30 · `37b1c8cb`）
- fix(web): 焦点环与表单可访问名全站收口，术语巡检差异归零（2026-09-30 · `718e2977`）
- fix(web): 全局 border-box 统一控件盒模型，巡检工装补几何断言（2026-09-29 · `f807b5d0`）
- fix(web): 修亮色主题下隐形的文字——渐变卡遗留的白字与浅染上的 on-accent（2026-09-28 · `c8664fb7`）
- fix(web): 修可读性与布局密度——液态玻璃材质重调 + WCAG 对比度门禁（2026-09-28 · `97f60c34`）
- fix(web): 液态玻璃重做为苹果式浅色清玻璃（非暗色系）（2026-09-28 · `4eae1409`）
- fix(web): 液态玻璃面板表面与 editorial 阅读排版签名（2026-09-28 · `c494193f`）
- fix(web): markdown 渲染与编辑体验修复（目录锚点/公式/任务列表/脚注/wiki/emoji/CRLF）（2026-09-28 · `452e9526`）
- fix: AI 未启用时所有 AI 能力不调用、不显示（2026-09-28 · `13cfcc64`）
- fix: 目录树层级错位/系统设置排序语义/权限收权三处修复（2026-09-28 · `2ea630a9`）
- fix(web): 导航单一事实源——侧边栏与面包屑从路由 meta 派生（2026-09-28 · `6df07ff4`）
- fix(ui): 页面左右留白只由 ApPageContainer 一层负责（2026-09-27 · `3ffe4586`）
- fix(ui): 列表页补齐 compact，表格页版心统一回 1600（2026-09-27 · `44100359`）
- fix(genddl): 不再给 LOB 列合成默认值，table.sql 恢复可导入（2026-09-27 · `567ef110`）
- fix: 重生成 Swagger 并修接线查出的 8 处后端契约缺陷（2026-09-27 · `9c23ef44`）
- fix: 对齐前后端接口与库结构真源，补齐未接线能力的前端 UI（2026-09-27 · `b7802653`）

## 重构

- refactor(web): FAB 并档成 ApFloatingActionButton（2026-10-06 · `9a97b6eb`）
- refactor(web): ApCard 收口为单一形态，删 5 个零消费者 variant（2026-10-04 · `0c341a25`）
- refactor(web): 交互一致性 P1a —— 选中态收敛到一套语言（2026-10-04 · `d14df7bc`）
- refactor(web): 液态玻璃从独立主题降级为材质轴（外观 × 材质 双轴）（2026-10-04 · `bd2f9629`）
- refactor(web): 全局关键帧只剩一个注入块，删掉 styles/global-animations.ts（2026-10-03 · `e8f7a9a6`）
- refactor(web): 删掉 ai-data.ts 里 8 个从未被调用的 schema-spaces 包装（2026-09-30 · `c54e85b2`）
- refactor(web): 令牌表面一个概念只留一个名字，删掉 10 个转发别名与 12 个废弃常量（2026-09-30 · `902bdd61`）
- refactor(web): 控件尺寸与条目呈现收口为单一实现，并修四道假绿门禁（2026-09-29 · `3048e96e`）
- refactor(web): 表面阶梯与卡片网格尺度统一——壳层/输入/浮层各归其位（2026-09-29 · `63964f88`）
- refactor(web): 卡片密度与度量收口（2026-09-28 · `c4cfed54`）
- refactor(web): 全站重绘为克制档——去掉通用 AI 界面观感，主题一致性收口（2026-09-28 · `17898255`）
- refactor: 备份与恢复不再依赖 mysqldump / mysql 客户端（2026-09-27 · `bce90b29`）
- refactor(ui): 面包屑收敛为单一实现，首页页头入容器插槽，分享页套 680 行度量（2026-09-27 · `697af9ee`）
- refactor(model): 13 处无名 uniqueIndex 显式命名为 uk_* 规范索引（2026-09-27 · `dc4b2d1c`）
- refactor(auth): 去掉 POST /auth/sso/:id/login 的重复注册（2026-09-27 · `e21feec0`）
- refactor: 清理冗余与文档失真条目（2026-09-27 · `15cc021f`）
- refactor(databases): 合并为单一 DDL + 单一 DML（2026-09-24 · `3c28386e`）

## 文档

- docs(release): 写清改名后哪些旧名是刻意保留的，以及验收口径为什么不是"全仓 0 处"（2026-10-07 · `bdc44340`）
- docs(brand): 改名写进文档与发布物料；CHANGELOG 判据改成行首 trailer（2026-10-07 · `7c24e168`）
- docs(repo): 把今天被改动的计数校正回来（699 Go 文件 / 98 测试文件 / 191 单测）（2026-10-07 · `07e8fd02`）
- docs(release): 清单第 7 条 —— 上 GitHub 的五步只能维护者做（2026-10-07 · `0a01baf4`）
- docs(release): v2.0.0 发布说明 + 安全渠道接上 GitHub + 修 CHANGELOG 的假阳性（2026-10-07 · `49f65ae6`）
- docs(release): 英文发布面 —— README 拆成 EN/中文两份 + CONTRIBUTING + 行为准则（2026-10-07 · `05e7a250`）
- docs(repo): 否掉"行尾归一"待办 —— 实测仓库本来就是 LF（2026-10-06 · `60601953`）
- docs(brand): 竞品拆解结论 + 开源发布清单 + 品牌视觉方案（2026-10-06 · `b574dcbe`）
- docs(build): 补两条 bat 踩坑——括号块别放中文、跑打包时别改脚本（2026-10-03 · `83260e0a`）
- docs(build): 打包文档补上安装版这条产出线（2026-10-03 · `a69592ca`）
- docs: 动效与字距三条口径，以及 check:motion 的覆盖面边界（2026-10-01 · `b3c488d9`）
- docs: 记录三块新覆盖（主题轮 / 键盘焦点 / 长内容）与两条排版口径（2026-10-01 · `de8c3bdd`）
- docs: 修正工装段落的插入位置（check:detail 的自检说明被挤到 check:elements 段尾）（2026-10-01 · `fc8cdd19`）
- docs: 主题规范同步液态玻璃浅色形态（WWDC25 查证结论）（2026-09-28 · `7318bd45`）
- docs: README 补按源码复刻的界面图，并修正 AGENTS/features 的失真数字（2026-09-27 · `ce88c1ec`）
- docs: 补分层架构设计与编码规范（2026-09-24 · `87ccb5cc`）

## 格式化（ESLint / gofmt / prettier，不改行为）

- style(web): eslint --fix（属性顺序 / v-model 连字符 / 引号）（2026-10-05 · `a23ffbd4`）
- style(backend): gofmt 6 个文件（结构体对齐 + health 导入排序）（2026-10-05 · `47a8f1e7`）
- style: 全站布局一致性重排（四标准 A-D 落地）（2026-09-29 · `310bcd6f`）

## 测试

- test(repocheck): OpenAPI 覆盖率两道棘轮 + CI 校验 spec 未过期（2026-10-07 · `0eef864d`）
- test(web): 巡检覆盖补四个"要走到才渲染"的入口，并把零渲染分成死码/条件渲染/需交互三类（2026-10-02 · `ebf82f20`）
- test(web): 补 5 条静态教条判据 G17–G21，并清掉它们抓到的 40 项源码级漂移（2026-10-02 · `dd859f0b`）
- test(web): 逐页 apple-design 审计工装 check:apple + 十台产物按页记分卡（2026-10-02 · `1f842d67`）
- test(web): R8 豁免出流控件 —— 绝对定位悬浮层不该被中间层的 overflow:hidden 记账（2026-10-02 · `1f5d3c6f`）
- test(web): 页面对等性巡检 check:parity —— 把每页画出来再逐页比（2026-10-02 · `71afb266`）
- test(web): 逐组件位置尺寸普查 check:boxes，附一份诚实的覆盖清单（2026-10-01 · `54bb97ec`）
- test(web): 逐元素视觉签名巡检 check:visual（2026-10-01 · `84e31724`）
- test(web): 动效巡检 check:motion，另加两条门禁 G15/G16（2026-10-01 · `53562c62`）
- test(web): 元素巡检补上主题轮与键盘焦点，另开一台长内容抗压工装（2026-10-01 · `b6a172e3`）
- test(web): 元素级令牌一致性工装 check:elements（六条断言 + 探针自检）（2026-10-01 · `0de471ff`）
- test(web): 页面细节巡检 check:detail（7 条内容/结构断言 + 术语漂移登记）（2026-10-01 · `1284019b`）
- test(web): check:geom 补 R9、check:dialog 补 D7/D8，两处"看着像缺陷"被实测否掉（2026-10-01 · `0225d041`）
- test(web): check:geom 扩到多档视口 + 两条新断言，新增弹层巡检 check:dialog（2026-10-01 · `b9bd2899`）
- test(web): 元素级几何工装 check:geom，七条探针全部带反向验证（2026-10-01 · `bbd5ab2b`）
- test(web): 壳层横条同高加两层门禁，并清掉两处藏起来的第二把尺（2026-09-30 · `79e5420a`）

## CI

- ci(backend): golangci-lint 钉 v1 线并按 PR diff 拦新增债，记下第一次实跑的 340 条存量（2026-10-07 · `29b65d8f`）

## 杂项

- chore(release): CHANGELOG 落成生成物（tools/gen-changelog.mjs + make changelog）（2026-10-06 · `b31ae2a6`）
- chore(hooks): pre-commit 补上 ESLint 第一道 —— 一条 error 曾跟着钩子过三个提交（2026-10-06 · `ea141c7c`）
- chore(repo): 开源发布物料 —— SECURITY、issue/PR 模板、CI 补单测与文档门禁（2026-10-06 · `a9e5665a`）
- chore(web): 清零全部 32 个 no-unused-vars（含一处死链级联）（2026-10-05 · `04d5becc`）
- chore(web): 删 SpaceRootContent 两个未使用的样式常量（2026-10-05 · `180e1edd`）
- chore(web): 把 pagePxLiterals 降到实测 727（弹窗宽度并档后少两枚裸 px）（2026-10-03 · `dc4f846a`）
- chore(web): 把两条 ratchet 降到本批实测值（localStyleClones 119、pagePxLiterals 729）（2026-10-03 · `df706070`）
- chore(web): 跨页一致性量测工装，快照文件名去序号（2026-10-01 · `29df1b23`）
- chore: 更正 AGENTS.md 被本轮推翻的计数，加前端死导出检测工装（2026-09-30 · `62e23f96`）
- chore: initial commit（2026-09-24 · `d0a45c16`）

## 其他（提交标题不符合 Conventional Commits）

- security(repocheck): 入库内容凭据形状扫描 + 记下"两条历史血统"这件公开前必须知道的事（2026-10-07 · `0a78ae6c`）
- 优化（2026-10-03 · `820c8263`）
- 清理（2026-09-29 · `b0a5e8aa`）
