package middleware

type Chain struct {
	middlewares []RAGMiddleware
}

func NewChain(mws ...RAGMiddleware) *Chain {
	return &Chain{middlewares: mws}
}

func (c *Chain) Use(mw RAGMiddleware) {
	c.middlewares = append(c.middlewares, mw)
}

func (c *Chain) RunBeforeRetrieval(ctx *RAGContext) {
	for _, mw := range c.middlewares {
		if ctx.Err != nil {
			return
		}
		mw.BeforeRetrieval(ctx)
	}
}

func (c *Chain) RunAfterRetrieval(ctx *RAGContext) {
	for _, mw := range c.middlewares {
		if ctx.Err != nil {
			return
		}
		mw.AfterRetrieval(ctx)
	}
}

func (c *Chain) RunBeforeGeneration(ctx *RAGContext) {
	for _, mw := range c.middlewares {
		if ctx.Err != nil {
			return
		}
		mw.BeforeGeneration(ctx)
	}
}

func (c *Chain) RunAfterGeneration(ctx *RAGContext) {
	for _, mw := range c.middlewares {
		if ctx.Err != nil {
			return
		}
		mw.AfterGeneration(ctx)
	}
}
