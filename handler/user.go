package handler

import (
	"ec/config"
	"ec/database"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserHandler struct {
	db        *gorm.DB
	JWTSecret string
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
}
type LoginRequest struct {
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"required,email"`
}
type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func NewUserHandler(db *gorm.DB, cfg *config.Config) *UserHandler {
	return &UserHandler{
		db:        db,
		JWTSecret: cfg.JWTSecret,
	}
}

func (u *UserHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var existingUser database.User
	err := u.db.Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		// 查到记录，邮箱已存在
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已注册"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// “数据库错误”
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}
	// 走到这里说明 err 是 gorm.ErrRecordNotFound，邮箱可用，继续注册
	data, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码生成失败"})
		return
	}
	user := database.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(data),
		Role:     "customer",
	}
	err = u.db.Create(&user).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册表创建失败"})
		return
	}
	// 返回用户信息（不包含密码）
	c.JSON(http.StatusCreated, gin.H{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
	})
}

func (u *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var existingUser database.User
	err := u.db.Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		// 查到记录，邮箱已存在
		if err1 := bcrypt.CompareHashAndPassword([]byte(existingUser.Password), []byte(req.Password)); err1 != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
			return
		}
		//密码正确，签发token
		tokenString, err2 := u.Sign(&existingUser)
		if err2 != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token签发失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"token": tokenString,
			"user": gin.H{
				"id":    existingUser.ID,
				"name":  existingUser.Name,
				"email": existingUser.Email,
			},
		})
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// 这个邮箱没注册
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}
	// 真正的数据库错误
	c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
	return
}

func HealthHandle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (u *UserHandler) Sign(existingUser *database.User) (string, error) {
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
	tokenString, err := token.SignedString([]byte(u.JWTSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
