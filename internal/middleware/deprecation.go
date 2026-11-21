package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func DeprecationMiddleware(replacement string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Deprecation", "true")
		c.Writer.Header().Set("Sunset", "Sat, 30 Jun 2026 00:00:00 GMT")
		c.Writer.Header().Set("Link", fmt.Sprintf("</api/v1%s>; rel=\"successor-version\"", replacement))
		c.Next()
	}
}
