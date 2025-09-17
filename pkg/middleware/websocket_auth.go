package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// WebSocketAuthMiddleware validates JWT tokens for WebSocket connections
func WebSocketAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from query parameter or header
		token := c.Query("token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Fields(authHeader)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					token = parts[1]
				}
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authentication token"})
			c.Abort()
			return
		}

		// Validate the token
		claims, err := ValidateToken(token, false)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Store user information in request context for WebSocket handler

		ctx := context.WithValue(c.Request.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "username", claims.Username)
		ctx = context.WithValue(ctx, "email", claims.Email)

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
