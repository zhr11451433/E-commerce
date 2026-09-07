package auth

import (
	"ec/database"

	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func Sign(existingUser *database.User, jwts string) (string, error) {
	// 1. 设置过期时间（类型特殊，用 jwt.NewNumericDate 转）
	exp := jwt.NewNumericDate(time.Now().Add(24 * time.Hour))
	// 2. 组装 claims
	claims := Claims{
		UserID: existingUser.ID,
		Role:   existingUser.Role,
		Email:  existingUser.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: exp,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	// 3. 创建 token + 签名（HS256 = 对称加密，同一个 secret 既能签也能验）
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwts))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
