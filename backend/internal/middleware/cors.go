package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS(optionsPassthroughPaths ...string) gin.HandlerFunc {
	return CORSWithOrigins(nil, optionsPassthroughPaths...)
}

func CORSWithOrigins(allowedOrigins []string, optionsPassthroughPaths ...string) gin.HandlerFunc {
	passthrough := make(map[string]struct{}, len(optionsPassthroughPaths))
	for _, value := range optionsPassthroughPaths {
		passthrough[value] = struct{}{}
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, value := range allowedOrigins {
		allowed[value] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		c.Writer.Header().Add("Vary", "Origin")
		if _, ok := allowed[origin]; ok && origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-CSRF-Token")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
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
