package main

import (
	"ec/config"
	"ec/database"
	"fmt"
	"log"
	"net/http"
	
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(".env文件加载失败")
	}
	db, err := database.LinkMySQL(cfg)
	if err != nil {
		log.Fatalf("连接 MySQL 失败: %v", err)
	}
	fmt.Println("连接MySQL成功")
	rdb, err := database.LinkRedis(cfg)
	if err != nil {
		log.Fatalf("连接 Redis 失败: %v", err)
	}
	fmt.Println("连接Redis成功")
	defer rdb.Close()
	r := gin.Default()
	r.GET("/health", healthHandle)
	_ = r.Run(":8080")
}

func healthHandle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
