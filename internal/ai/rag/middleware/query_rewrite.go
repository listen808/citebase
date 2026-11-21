package middleware

type QueryRewriteMiddleware struct {
}

func NewQueryRewriteMiddleware() *QueryRewriteMiddleware {
	return &QueryRewriteMiddleware{}
}

func (m *QueryRewriteMiddleware) Name() string {
	return "query_rewrite"
}

func (m *QueryRewriteMiddleware) BeforeRetrieval(ctx *RAGContext) {
	if ctx.RewrittenQ == "" {
		ctx.RewrittenQ = ctx.Question
	}
	if len(ctx.SubQuestions) == 0 {
		ctx.SubQuestions = []string{ctx.Question}
	}
}

func (m *QueryRewriteMiddleware) AfterRetrieval(ctx *RAGContext) {
}

func (m *QueryRewriteMiddleware) BeforeGeneration(ctx *RAGContext) {
}

func (m *QueryRewriteMiddleware) AfterGeneration(ctx *RAGContext) {
}
