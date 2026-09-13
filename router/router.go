package router

import (
	"ec/config"
	"ec/handler"
	"ec/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *gin.Engine {
	r := gin.Default()
	userHandler := handler.NewUserHandler(db, cfg)
	categoryHandler := handler.NewCategoryHandler(db, rdb)
	productHandler := handler.NewProductHandler(db, rdb)
	cartHandler := handler.NewCartHandler(db)
	orderHandler := handler.NewOrderHandler(db, rdb)
	adminMiddleware := middleware.RequireAdmin()             //验证
	authMiddleware := middleware.Auth(userHandler.JWTSecret) //门卫
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
	cart := r.Group("/cart", authMiddleware)
	{
		cart.GET("", cartHandler.GetCart)
		cart.POST("/items", cartHandler.AddItem)
		cart.PUT("/items/:id", cartHandler.UpdateCart)
		cart.DELETE("/items/:id", cartHandler.DeleteCart)
	}
	order := r.Group("/orders", authMiddleware)
	{
		order.POST("", orderHandler.Checkout)
		order.GET("", orderHandler.List)
		order.GET("/:id", orderHandler.ListId)
		order.PUT("/:id", adminMiddleware, orderHandler.Update)
	}
	return r
}
