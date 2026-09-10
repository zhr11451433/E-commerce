package handler

import (
	"ec/database"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CartHandler struct {
	db *gorm.DB
}

func NewCartHandler(db *gorm.DB) *CartHandler {
	return &CartHandler{db: db}
}

type cartItemRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}
type addCartItem struct {
	ProductId int `json:"product_id" binding:"required,min=1"`
	Quantity  int `json:"quantity" binding:"required,min=1"`
}

//看我的购物车

func (u *CartHandler) GetCart(c *gin.Context) {
	userId, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不存在"})
		return
	}
	id, ok := userId.(uint)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "类型错误"})
		return
	}
	var resCart database.Cart
	if err := u.db.Where("user_id = ?", id).First(&resCart).Error; err != nil {
		c.JSON(http.StatusOK, "")
		return
	}
	var resCartItem []database.CartItem
	u.db.Preload("Product").Where("cart_id = ?", resCart.ID).Find(&resCartItem)
	c.JSON(http.StatusOK, gin.H{
		"cart_id": resCart.ID,
		"items":   resCartItem,
	})
}

//加商品 {product_id, quantity}

func (u *CartHandler) AddItem(c *gin.Context) {
	userId, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不存在"})
		return
	}
	id, ok := userId.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "类型错误"})
		return
	}
	var req addCartItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var existingProduct database.Product
	if err := u.db.Where("id = ?", req.ProductId).First(&existingProduct).Error; err == nil {
		// 4. 找或建购物车 关键点 1
		var existingCart database.Cart
		u.db.FirstOrCreate(&existingCart, database.Cart{UserId: id})
		// 5. 判断商品是否已在购物车中（关键点 2
		var item database.CartItem
		if err := u.db.Where("cart_id = ? and product_id = ?", existingCart.ID, req.ProductId).First(&item).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			item = database.CartItem{
				CartId:    existingCart.ID,
				ProductId: uint(req.ProductId),
				Quantity:  req.Quantity,
			}
			if err := u.db.Create(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败"})
				return
			}
			c.JSON(http.StatusCreated, item)
			return
		} else if err == nil {
			// 已在购物车 → 数量累加
			item.Quantity += req.Quantity
			if err := u.db.Save(&item).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新数量失败"})
				return
			}
			c.JSON(http.StatusOK, item)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "没找到该产品"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
}

//改数量 {quantity}

func (u *CartHandler) UpdateCart(c *gin.Context) {
	userId, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不存在"})
		return
	}
	uid, ok := userId.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "类型错误"})
		return
	}
	idStr := c.Param("id")
	//购物车商品的id
	cartItemID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id错误"})
		return
	}
	var req cartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var existingCart database.Cart
	if err := u.db.Where("user_id = ?", uid).First(&existingCart).Error; err == nil {
		var existingCartItem database.CartItem
		if err := u.db.Where("id = ? and cart_id = ?", cartItemID, existingCart.ID).First(&existingCartItem).Error; err == nil {
			existingCartItem.Quantity = req.Quantity
			if err := u.db.Save(&existingCartItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
				return
			}
			c.JSON(http.StatusOK, existingCartItem)
			return
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "没找到该产品"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "没找到该产品"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
	return
}

//移除

func (u *CartHandler) DeleteCart(c *gin.Context) {
	userId, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不存在"})
		return
	}
	uid, ok := userId.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "类型错误"})
		return
	}
	idStr := c.Param("id")
	cartItemID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id错误"})
		return
	}
	var existingCart database.Cart
	if err := u.db.Where("user_id = ?", uid).First(&existingCart).Error; err == nil {
		result := u.db.Where("id = ? and cart_id = ?", cartItemID, existingCart.ID).Delete(&database.CartItem{})
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
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "没找到该产品"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
	return
}
