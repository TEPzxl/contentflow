package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tepzxl/contentflow/internal/http/requestctx"
	"github.com/tepzxl/contentflow/internal/http/response"
)

type ParseAccessTokenFunc func(token string) (int64, error)

func AuthRequired(parseAccessToken ParseAccessTokenFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "missing authentication header")
			c.Abort()
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "invalid authentication header")
			c.Abort()
			return
		}

		userID, err := parseAccessToken(parts[1])
		if err != nil || userID <= 0 {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "invalid access token")
			c.Abort()
			return
		}

		ctx := requestctx.WithUserID(c.Request.Context(), userID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func GetUserID(c *gin.Context) (int64, bool) {
	value, ok := requestctx.UserID(c.Request.Context())
	if !ok {
		return 0, false
	}

	return value, true
}
