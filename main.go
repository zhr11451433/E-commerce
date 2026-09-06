package main

import (
	"ec/config"
	"ec/database"
	"ec/handler"
	"ec/router"
	"fmt"
	"log"
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
	uh := handler.NewUserHandler(db, cfg)
	rdb, err := database.LinkRedis(cfg)
	if err != nil {
		log.Fatalf("连接 Redis 失败: %v", err)
	}
	fmt.Println("连接Redis成功")
	defer rdb.Close()
	err = database.AutoMigrate(db)
	if err != nil {
		log.Fatal("数据库迁移失败: ", err)
	}
	r := router.NewRouter()
	r.GET("/health", handler.HealthHandle)
	r.POST("/register", uh.Register)
	r.POST("/login", uh.Login)
	if err = r.Run(":8080"); err != nil {
		log.Fatal("服务启动失败", err)
	}
}
