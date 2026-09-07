package middleware

import (
	"ec/auth"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

//它是一个"门卫"，负责回答"你是谁"，然后把答案递给后面的人
//验证请求者的身份，并把身份信息传递给后续的处理函数或中间件

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 取 header：Authorization: Bearer <token>
		header := c.GetHeader("Authorization")
		// 2. 切成两段，校验格式 "bearer" "token"
		parts := strings.Fields(header)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			return
		}
		tokenString := parts[1]
		// 3. 解析 + 验证（用 auth.Claims，keyFunc 返回密钥）
		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
			return
		}

		// 4. 验证通过：把身份信息存进 context，放行
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("email", claims.Email)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		v, _ := c.Get("role")
		switch v {
		case "admin":
			c.Next()
		case "customer":
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			return
		}
	}
}
