package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS(optionsPassthroughPaths ...string) gin.HandlerFunc {
	passthrough := make(map[string]struct{}, len(optionsPassthroughPaths))
	for _, value := range optionsPassthroughPaths {
		passthrough[value] = struct{}{}
	}
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			if _, ok := passthrough[c.Request.URL.Path]; ok {
				c.Next()
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
