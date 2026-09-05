package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.Default()
	r.GET("/health", healthHandle)
	return r
}

func healthHandle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
