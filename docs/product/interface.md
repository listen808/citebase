# 界面预览与复刻依据

> 本文是 `README` 的视觉附录：真截图、复刻示意图，以及图上每个取值在代码里的唯一出处。

## 真截图

从运行实例截的像素图，1440×900。内容是临时演示空间，拍完已删除——不把真实知识库的标题带进公开仓库。

| 页面 | 语言 | 图 |
|---|---|---|
| 登录 | zh-CN | `assets/readme/login-zh.png` |
| 登录 | en-US | `assets/readme/login-en.png` |
| 文档阅读 | zh-CN | `assets/readme/document-zh.png` |
| 文档阅读 | en-US | `assets/readme/document-en.png` |
| 编辑器 | en-US | `assets/readme/editor-en.png` |

英文档界面仍有部分中文外壳（侧栏、工具栏、搜索框），归宿在 `README` 的 Known gaps 表。

## 复刻示意图（**不是截图**）

下面几张按前端源码逐项复刻：对话与文档内容为示例数据，每个色值、圆角、宽度都能指到代码。留成示意图的只有两处没有真 LLM key 与语料就无法诚实拍照的面：AI 回答与检索管线。

- `assets/readme/hero.svg` —— 定位图 + 答案引用三态徽标（绿已引用 / 橙未引用 / 红未找到），状态由 `[n]` 标记与片段覆盖度算出
- `assets/readme/workspace.svg` —— `/ai/workspace`：60px 玻璃顶栏、240px 侧导航、220px 会话列表、思考过程折叠块、行内引用、意图胶囊
- `assets/readme/space-editor.svg` —— 编辑页：300px 文档树、编辑/预览分栏、AI 幽灵文本、状态栏、36px 圆形目录按钮
- `assets/readme/rag-pipeline.svg` —— 十阶段：安全护栏 → 拆解 → 改写 → 混合检索（向量/关键词/HyDE/GraphRAG）→ RRF k=60 → 重排 → 权限过滤 → 生成校验 → 幻觉重试 → 引用定态

阶段名取自 `internal/ai/rag/retriever.go` 实际发出的 `RAGRetrievalStep`，不是事后补的说明文字。

两条不变量：**权限过滤按提问者身份执行，流式路径也不例外**；**引用状态由标记与覆盖度算出，不经 LLM 自评**。

## 复刻依据（取值 → 唯一实现处）

| 图里的东西 | 真实取值 | 唯一实现处 |
|---|---|---|
| 顶栏 48px + 玻璃 + 0.5px 下边框 | `AppleLayout.shellBarHeight`、`cv('surface-chrome')`、`cv('glass-blur')` | `apple-tokens.ts` · `ApNavBar.vue` |
| 侧栏 240px、白底、1px 右边框 | `sidebarWidth: '240px'` | `ApSidebar.vue` |
| 侧栏搜索框 radius 18 / bg `#f0f0f0` | `AppleRadius.card` + `cv('bg-tertiary')` | `ApSidebar.vue` |
| 导航项 = `ApButton size="small"` | h32 / padding `0 12px` / 13px / radius 13 | `AppleButtonSize.small` · `ApButton.vue` |
| 选中导航项蓝底白字 | `variant="primary"` → `#0071e3`（暗色 `#0a84ff`） | `ApSidebar.vue` |
| 会话列表 220px 固定不收缩 | `flexShrink: 0` | `ChatSidebar.vue` |
| 对话卡片 radius 18 + 1px `#f0f0f0` | `AppleRadius.card` | `styles/component/chat-styles.ts` |
| 行内引用角标 | bg `var(--apple-blue)`、11px semibold、h18 | `ChatMessageBubble.vue` |
| 三态徽标 | `#e6f9ef`+`#34c759` / `#fff3e0`+`#ff9500` / `#ffebeb`+`#ff3b30`，radius 6 | `ChatSourcesModal.vue` |
| 「思考过程 · N 步」折叠块 | bg `#f0f0f0`、radius 8、12px | `ChatMessageBubble.vue` |
| 意图胶囊 | `AppleRadius.pill` = 980px | `ChatInputBar.vue` |
| 文档树 300px（可拖 280–400） | `useResizable({ initialWidth: 300, minWidth: 280, maxWidth: 400 })` | `SpaceView.vue` |
| 树节点行高 26 / radius 8 / 缩进 `level*16`（上限 80） | — | `TreeNode.vue` |
| 选中行 16% 蓝底 | `--apple-blue-selection-strong: rgba(0,113,227,0.16)` | `TreeNode.vue` |
| 文件夹图标色 | `--apple-folder-icon: rgb(245,166,35)` | `apple-tokens.ts` |
| 编辑/预览分栏、5px 手柄、默认 0.5 | clamped 0.2–0.8 | `DocumentEditor.vue` |
| AI 幽灵文本 +「Tab 接受 · Esc 取消」 | `.ai-ghost-text` | `MonacoEditor.vue` |
| 右下 36px 圆形目录按钮 → 220px 面板 | `PANEL_WIDTH = 220` | `FloatingToc.vue` |
| 正文 17px / 页标题 21px / 微标签 12px | `AppleType.bodyPrimary` / `pageTitle` / `microUI` | `apple-tokens.ts` |

> 文字尺寸是唯一刻意改动项：真实界面 12–13px 的标签在图里按 14–16 画，否则 GitHub 窄栏不可读；几何、色值、圆角、边框、层级均为 1:1。深色模式同源：`[data-theme='dark']` 覆盖块换一套值，结构不变。

## 图里刻意没画的东西

前端代码里没有的，不复刻：

- AI 对话页**没有右侧面板**——只有会话列表 + 消息列两栏。
- 协同编辑**没有头像堆、没有彩色远端光标**——只有「会话 N」弹层与状态栏「N 人协作」。
- 文档页**没有审批状态徽章**——`待审批` / `已通过` 只出现在 `/doc-collab`。
- **没有固定右侧目录栏**——目录由右下浮动按钮展开 220px 面板。
- 回答下方**不显示 token 用量 / 耗时 / 模型名**——终帧带了数据，界面未渲染。
