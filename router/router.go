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
	productHandler := handler.NewProductHandler(db)
	cartHandler := handler.NewCartHandler(db)
	adminMiddleware := middleware.RequireAdmin()
	authMiddleware := middleware.Auth(userHandler.JWTSecret)
	r.GET("/me", authMiddleware, handler.Me)
	r.POST("/register", userHandler.Register)
	r.POST("/login", userHandler.Login)
	category := r.Group("/categories")
	{
		category.GET("", categoryHandler.List)
		category.POST("", authMiddleware, adminMiddleware, categoryHandler.Create)
		category.PUT("/:id", authMiddleware, adminMiddleware, categoryHandler.Update)
		category.DELETE("/:id", authMiddleware, adminMiddleware, categoryHandler.Delete)
	}
	product := r.Group("/products")
	{
		product.GET("", productHandler.List)
		product.GET("/:id", productHandler.Get) //单个查询
		product.POST("", authMiddleware, adminMiddleware, productHandler.Create)
		product.PUT("/:id", authMiddleware, adminMiddleware, productHandler.Update)
		product.DELETE("/:id", authMiddleware, adminMiddleware, productHandler.Delete)
	}
	cart := r.Group("/cart")
	{
		cart.GET("", authMiddleware, cartHandler.GetCart)
		cart.POST("/items", authMiddleware, cartHandler.AddItem)
		cart.PUT("/items/:id", authMiddleware, cartHandler.UpdateCart)
		cart.DELETE("/items/:id", authMiddleware, cartHandler.DeleteCart)
	}
	return r
}
