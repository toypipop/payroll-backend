package middleware

import (
	"net/http"
	"strings"

	"payroll-backend/handlers"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

/* --- AuthMiddleware validates JWT and extracts Tenant info (English Comment) --- */
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get Token from Header (English Comment)
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// 2. Check "Bearer " prefix (English Comment)
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims := &handlers.TokenClaims{}

		// 3. Parse and Validate Token (English Comment)
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return handlers.GetJWTSecret(), nil // แนะนำให้ทำ getter สำหรับ jwtSecret
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// 4. Set Tenant Data to Context (English Comment)
		c.Set("user_id", claims.UserID)
		c.Set("company_id", claims.CompanyID)
		c.Set("role_id", claims.RoleID)

		c.Next()
	}
}
