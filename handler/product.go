package handler

import (
	"ec/database"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductRequest struct {
	Name        string          `binding:"required" json:"name"`
	Price       decimal.Decimal `binding:"required" json:"price"`
	Description string          `binding:"required" json:"description"`
	Number      int             `binding:"gte=0" json:"number"`
	CategoryID  uint            `binding:"required" json:"category_id"`
}

type ProductHandler struct {
	db *gorm.DB
}

func NewProductHandler(db *gorm.DB) *ProductHandler {
	return &ProductHandler{db: db}
}

//page 在这里是分页查询中的页码，表示你想查看第几页的数据。它通常和 page_size（每页数据条数）配合使用。

func (u *ProductHandler) List(c *gin.Context) {
	var res []database.Product
	pageString := c.DefaultQuery("page", "1")
	pageSizeString := c.DefaultQuery("page_size", "10")
	page, err := strconv.Atoi(pageString)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeString)
	if err != nil || pageSize < 1 {
		pageSize = 10
	}
	// 计算偏移量
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	u.db.Preload("Category").Limit(pageSize).Offset(offset).Find(&res)
	var count int64
	u.db.Model(&database.Product{}).Count(&count)
	//u.db.Raw("SELECT COUNT(*) FROM products").Scan(&count)

	c.JSON(http.StatusOK, gin.H{
		"data":      res,
		"total":     count,
		"page":      page,
		"page_size": pageSize,
	})
}
func (u *ProductHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var product database.Product
	err = u.db.Preload("Category").First(&product, uint(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "产品不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		}
		return
	}

	c.JSON(http.StatusOK, product)
}

func (u *ProductHandler) Create(c *gin.Context) {
	var req ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 手动检查价格非负
	if req.Price.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "价格不能为负数"})
		return
	}
	// 手动检查库存非负（binding 已含 gte=0，但双保险）
	if req.Number < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "价格不能为负数"})
		return
	}
	var existingCategory database.Category
	if err := u.db.Where("id = ?", req.CategoryID).First(&existingCategory).Error; err == nil {
		//分类表存在，但还要判断该产品是否已经存在
		var existingProduct database.Product
		if err1 := u.db.Where("name = ? AND category_id = ?", req.Name, req.CategoryID).First(&existingProduct).Error; err1 == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "产品已存在"})
			return
		} else if !errors.Is(err1, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
			return
		}
		product := database.Product{
			Description: req.Description,
			Name:        req.Name,
			CategoryId:  req.CategoryID,
			Price:       req.Price,
			Number:      req.Number,
		}
		if err := u.db.Create(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败"})
			return
		}
		c.JSON(http.StatusCreated, product)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	//没找到分类表，所以该产品无法创建
	c.JSON(http.StatusBadRequest, gin.H{"error": "没找到分类表,该产品无法创建"})
	return
}

func (u *ProductHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id错误"})
		return
	}
	var existingProduct database.Product
	if err := u.db.Where("id = ?", uint(id)).First(&existingProduct).Error; err == nil {
		//表存在，可以更新
		var req ProductRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// 手动检查价格非负
		if req.Price.IsNegative() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "价格不能为负数"})
			return
		}
		// 检查库存非负
		if req.Number < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "库存不能为负数"})
			return
		}
		// 3. 更新非零值字段
		// 全量更新字段
		existingProduct.Name = req.Name
		existingProduct.Description = req.Description
		existingProduct.Price = req.Price
		existingProduct.Number = req.Number
		existingProduct.CategoryId = req.CategoryID
		// 保存到数据库
		if err := u.db.Save(&existingProduct).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}
		c.JSON(http.StatusOK, existingProduct)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	//不存在，无法更新
	c.JSON(http.StatusNotFound, gin.H{"error": "更新失败"})
	return
}

func (u *ProductHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id错误"})
		return
	}
	result := u.db.Where("id = ?", uint(id)).Delete(&database.Product{})
	if result.Error != nil {
		//表不存在或其他错误
		c.JSON(http.StatusInternalServerError, result.Error.Error())
		return
	} else if result.Error == nil && result.RowsAffected > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "删除成功"})
		return
	} else if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "删除成功"})
}
