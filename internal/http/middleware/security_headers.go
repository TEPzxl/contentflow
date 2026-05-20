package middleware

import "github.com/gin-gonic/gin"

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}
