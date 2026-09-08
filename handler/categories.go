package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ec/database"
)

type CategoryHandler struct {
	db *gorm.DB
}

type CategoryRequest struct {
	Name string `binding:"required" json:"name"`
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{db: db}
}

func (u *CategoryHandler) List(c *gin.Context) {
	var res []database.Category
	u.db.Model(&database.Category{}).Find(&res)
	c.JSON(http.StatusOK, res)
}

func (u *CategoryHandler) Create(c *gin.Context) {
	var req CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var existingReq database.Category
	if err := u.db.Model(&database.Category{}).Where("name = ?", req.Name).First(&existingReq).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "表已存在"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) { //数据库错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	//errors.Is(err,gorm.ErrRecordNotFound) 没找到
	res := database.Category{Name: req.Name}
	if err := u.db.Create(&res).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"status":   "创建成功",
		"category": res,
	})
}

func (u *CategoryHandler) Update(c *gin.Context) {
	var req CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 32) // 10 表示十进制，32 表示结果不超过 32 位
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	id := uint(id64)
	var existingReq database.Category
	if err := u.db.Model(&database.Category{}).Where("id = ?", id).First(&existingReq).Error; err == nil {
		existingReq.Name = req.Name
		if err := u.db.Save(&existingReq).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "更新成功"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) { //数据库错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"status": "表不存在"})
	return
}

func (u *CategoryHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 32) // 10 表示十进制，32 表示结果不超过 32 位
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	id := uint(id64)
	result := u.db.Where("id = ?", id).Delete(&database.Category{})
	if result.Error != nil {
		//表不存在或其他错误
		c.JSON(http.StatusInternalServerError, result.Error.Error())
		return
	} else if result.Error == nil && result.RowsAffected > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "删除成功"})
		return
	} else if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}
}
