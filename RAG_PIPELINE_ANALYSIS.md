# Citebase RAG 检索管线架构分析

## 管线总览

```
用户问题
  │
  ├─ S1 安全护栏 (middleware/safety_guard.go)
  │     BeforeRetrieval → CheckQuery；拦截则设 ctx.Err 终止
  │
  ├─ S2 语义缓存 (query_cache.go)
  │     Lookup(question, spaceID, userID)：精确 key + 向量相似度(阈值0.95)，TTL 24h
  │     命中直接返回，跳过后续全部阶段
  │
  ├─ S3 问题分解 (retriever.go:decomposeQuestion)
  │     LLM 判断是否需拆分子问题；>1 则走 askWithSubQuestions 并行检索后合并
  │
  ├─ S4 查询重写 (retriever.go:rewriteQuery + middleware/query_rewrite.go)
  │     优化原始查询表述以提升检索精度
  │
  ├─ S5 混合检索 (retriever.go:hybridRetrieveInternal)
  │     ├─ 向量路: embedding.VectorSearch
  │     ├─ 关键词路: Bleve 全文索引
  │     ├─ HyDE (hyde.go): LLM 生成假设文档→向量化→仅向量路
  │     └─ 融合: RRF (k=60, shared 包)
  │
  ├─ S6 重排序 (reranker.go)
  │     CrossEncoderReranker: POST /v1/rerank (10s超时，错误回退原序)
  │     ScoreReranker 备选: 基于质量评分
  │
  ├─ S7 权限过滤 (permission_checker.go:FilterSourcesByPermission)
  │     按 userID 查可访问空间ID，过滤无权文档；查询失败→fail-closed(清空全部)
  │     过滤后分配 CitationIndex = i+1
  │
  ├─ S8 LLM 生成 (retriever.go:callLLM)
  │     system prompt + buildCitedContext(sources) 含 <document_title>/<section_path>
  │     ragContext 置于独立 user message 保持 prefix cache 稳定
  │
  ├─ S9 幻觉检测 (hallucination_detector.go)
  │     LLM-as-judge JSON 报告(20s, temp 0.1)
  │     ValidateAnswerWithRetry: confidence>0.7 → 重试最多2次
  │     仍失败则追加 ⚠️ 警告
  │
  └─ S10 后处理 (retriever.go:559-654)
        MaskPII → safetyGuard.ValidateAnswer → EnrichAnswer(verifiable.go)
        VerifyCitations: 正则 \[(\d+)\] 匹配引用标记，标注 Cited/VerifyStatus
        BuildReasoningChain: retrieve/rerank/generate/verify 四步可视化
        存入 QueryCache
```

## 关键设计决策

1. **中间件链模式** (middleware/chain.go)：Before/After × Retrieval/Generation 四阶段钩子，安全护栏、查询重写等以可插拔中间件注入，`RAGContext.Err` 短路终止。
2. **权限 fail-closed** (permission_checker.go:36)：`FindAccessibleSpaceIDs` 查询出错时返回 nil 源列表，宁可无结果不泄露。
3. **HyDE 仅向量路** (hyde.go:59)：假设文档只做 VectorSearch，不经关键词路，避免假设文本噪声污染 BLEVE 索引匹配。
4. **SSE 流式权限** (agent_chat.go:86)：`SearchDocumentsTool.Execute` 内部调用 `FilterSourcesByPermission`，保证 Agent 自主检索路径与预检索路径权限一致。
5. **GraphRAG 不进入引用** (graph_expansion.go:37)：2-hop wiki 扩展结果明确标注"非引用来源"，仅作背景上下文注入 prompt，避免虚构引用。
6. **引用可验证性** (verifiable.go:40)：纯文本规则（无 LLM 依赖）——正则提取 `[n]`，与 `CitationIndex` 匹配判定 `cited`/`unverified`/`not_found`，12 字滑窗匹配片段存在性。

## 弱点与 TODO

- HyDE 分支 (agent_chat.go:retrieveWithStrategy) 直接返回 `RetrieveWithHyDE` 结果，**未调用 FilterSourcesByPermission**——若上游无额外过滤则存在越权风险（未验证是否有上层补偿）。
- `BuildPermissionContext` (permission_checker.go:103) 遍历全部空间 (FindAll 上限 10000) 仅计数可见空间，空间量大时性能瓶颈。
- 幻觉检测 20s 超时 + 最多 2 次重生成（每次再 20s），极端情况单次问答可叠加 80s+ 延迟，无总时长熔断。
- CrossEncoder 回退策略为静默降级（仅计数器），无日志告警级别区分。
- QueryCache 进程内内存实现，多实例部署时缓存不共享，命中率受限。

## 建议关注点

1. **HyDE 权限缺口**：确认 agent_chat.go 的 HyDE 策略分支是否在调用方补做了权限过滤，否则应补 `FilterSourcesByPermission`。
2. **幻觉检测延迟预算**：评估是否需要为 2 次重试设总超时上限（如 45s），避免 SSE 场景下用户长时间等待。
3. **权限上下文性能**：`BuildPermissionContext` 全量空间遍历可考虑缓存或仅取当前 spaceID 相关数据。
4. **引用校验粒度**：`snippetFoundIn` 12 字滑窗对长文档 snippet 可能误判 `not_found`，可引入 embedding 余弦相似度作为二次判定。
5. **缓存一致性**：文档更新后 QueryCache 无失效机制，建议接入文档版本号或空间级 invalidation 信号。
