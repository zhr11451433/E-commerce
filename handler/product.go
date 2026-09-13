package handler

import (
	"ec/database"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
	db  *gorm.DB
	rdb *redis.Client
}

func NewProductHandler(db *gorm.DB, rdb *redis.Client) *ProductHandler {
	return &ProductHandler{db: db, rdb: rdb}
}

func bindProductRequest(c *gin.Context) (ProductRequest, bool) {
	var req ProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return req, false
	}
	if req.Price.IsNegative() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "价格不能为负数"})
		return req, false
	}
	if req.Number < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "库存不能为负数"})
		return req, false
	}
	return req, true
}

type productListCache struct {
	Data     []database.Product `json:"data"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
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
	ctx := c.Request.Context()
	key := "products:list:" + strconv.Itoa(page) + ":" + strconv.Itoa(pageSize)
	val, err := u.rdb.Get(ctx, key).Result()
	if err == nil {
		//命中缓存
		var cached productListCache
		if err := json.Unmarshal([]byte(val), &cached); err == nil {
			c.JSON(http.StatusOK, cached)
			return
		}
	} else if !errors.Is(err, redis.Nil) {
		// Redis 故障，降级查库
	}
	err = u.db.Preload("Category").Limit(pageSize).Offset(offset).Find(&res).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	var count int64
	u.db.Model(&database.Product{}).Count(&count)
	//u.db.Raw("SELECT COUNT(*) FROM products").Scan(&count)
	out := productListCache{
		Data:     res,
		Total:    count,
		Page:     page,
		PageSize: pageSize,
	}
	data, err := json.Marshal(&out)
	if err == nil {
		u.rdb.Set(ctx, key, data, 10*time.Minute)
	}
	c.JSON(http.StatusOK, out)
}

func (u *ProductHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}
	ctx := c.Request.Context()
	key := "product:" + idStr
	var product database.Product
	val, err := u.rdb.Get(ctx, key).Result()
	// 1. 尝试从缓存读取
	if err == nil {
		// 命中缓存
		if val == "null" {
			// 之前标记过"该商品不存在"
			c.JSON(http.StatusNotFound, gin.H{"error": "产品不存在"})
			return
		}
		if err := json.Unmarshal([]byte(val), &product); err == nil {
			c.JSON(http.StatusOK, product)
			return
		}
		// 如果反序列化失败（缓存数据损坏），记录日志，然后继续往下走去查库
		// log.Printf("缓存数据损坏，key: %s, err: %v", key, err)
	} else if !errors.Is(err, redis.Nil) {
		// Redis 故障（比如连不上），降级去查库
		// log.Printf("Redis查询失败，降级查库: %v", err)
	}
	// 2. 缓存未命中 / Redis故障 / 缓存损坏 → 查数据库
	err = u.db.Preload("Category").First(&product, uint(id)).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防缓存穿透：把不存在的ID也缓存起来，设置短过期时间
			u.rdb.Set(ctx, key, "null", 1*time.Minute)
			c.JSON(http.StatusNotFound, gin.H{"error": "产品不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		}
		return
	}
	// 3. 写回缓存
	if data, err := json.Marshal(&product); err == nil {
		u.rdb.Set(ctx, key, data, 10*time.Minute)
	}
	c.JSON(http.StatusOK, product)
}

func (u *ProductHandler) Create(c *gin.Context) {
	req, ok := bindProductRequest(c)
	if !ok {
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
		invalidateProductCache(u.rdb, c.Request.Context(), product.ID)
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
		req, ok := bindProductRequest(c)
		if !ok {
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
		invalidateProductCache(u.rdb, c.Request.Context(), existingProduct.ID)
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
		invalidateProductCache(u.rdb, c.Request.Context(), uint(id))
		c.JSON(http.StatusOK, gin.H{"status": "删除成功"})
		return
	} else if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "商品不存在"})
		return
	}
}
