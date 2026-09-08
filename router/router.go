package router

import (
	"ec/config"
	"ec/handler"
	"ec/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()
	userHandler := handler.NewUserHandler(db, cfg)
	categoryHandler := handler.NewCategoryHandler(db)
	adminMiddleware := middleware.RequireAdmin()
	authMiddleware := middleware.Auth(userHandler.JWTSecret)
	r.GET("/me", authMiddleware, adminMiddleware, handler.Me)
	r.POST("/register", userHandler.Register)
	r.POST("/login", userHandler.Login)
	category := r.Group("/categories")
	{
		category.GET("", categoryHandler.List)
		category.POST("", authMiddleware, adminMiddleware, categoryHandler.Create)
		category.PUT("/:id", authMiddleware, adminMiddleware, categoryHandler.Update)
		category.DELETE("/:id", authMiddleware, adminMiddleware, categoryHandler.Delete)
	}
	return r
}
