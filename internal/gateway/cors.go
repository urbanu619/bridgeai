package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Headers clients may send (preflight must list non-simple headers).
const corsAllowHeaders = "Authorization, Content-Type, X-Request-ID, X-Anthropic-Key, X-Groq-Key, X-Gemini-Key"

// CORSMiddleware allows browser clients (e.g. demo.html on another origin) to call the API.
// OPTIONS preflight returns 204 before auth so no Bearer token is required on preflight.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", corsAllowHeaders)
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
