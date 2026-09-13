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

// 辅助函数：获取当前用户 ID（失败时已写好响应，调用方只需判断 ok 并 return）
func getCurrentUserID(c *gin.Context) (uint, bool) {
	userId, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return 0, false
	}
	uid, ok := userId.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户信息错误"})
		return 0, false
	}
	return uid, true
}

// 辅助函数：获取当前用户的购物车（失败时已写好响应）
func (u *CartHandler) getUserCart(c *gin.Context, uid uint) (*database.Cart, bool) {
	var cart database.Cart
	if err := u.db.Where("user_id = ?", uid).First(&cart).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "购物车不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		}
		return nil, false
	}
	return &cart, true
}

// 看我的购物车

func (u *CartHandler) GetCart(c *gin.Context) {
	uid, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	cart, ok := u.getUserCart(c, uid)
	if !ok {
		return
	}
	var items []database.CartItem
	u.db.Preload("Product").Where("cart_id = ?", cart.ID).Find(&items)
	c.JSON(http.StatusOK, gin.H{
		"cart_id": cart.ID,
		"items":   items,
	})
}

// 加商品 {product_id, quantity}

func (u *CartHandler) AddItem(c *gin.Context) {
	uid, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var req addCartItem
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var product database.Product
	if err := u.db.Where("id = ?", req.ProductId).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "没找到该产品"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		}
		return
	}

	// 找或建购物车
	var cart database.Cart
	if err := u.db.FirstOrCreate(&cart, database.Cart{UserId: uid}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}

	// 判断商品是否已在购物车
	var item database.CartItem
	err := u.db.Where("cart_id = ? AND product_id = ?", cart.ID, req.ProductId).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = database.CartItem{
			CartId:    cart.ID,
			ProductId: uint(req.ProductId),
			Quantity:  req.Quantity,
		}
		if err := u.db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败"})
			return
		}
		c.JSON(http.StatusCreated, item)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}

	// 已在购物车 → 数量累加
	item.Quantity += req.Quantity
	if err := u.db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新数量失败"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// 改数量 {quantity}

func (u *CartHandler) UpdateCart(c *gin.Context) {
	uid, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	cart, ok := u.getUserCart(c, uid)
	if !ok {
		return
	}

	idStr := c.Param("id")
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

	var item database.CartItem
	if err := u.db.Where("id = ? AND cart_id = ?", cartItemID, cart.ID).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "购物车项不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		}
		return
	}

	item.Quantity = req.Quantity
	if err := u.db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// 移除

func (u *CartHandler) DeleteCart(c *gin.Context) {
	uid, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	cart, ok := u.getUserCart(c, uid)
	if !ok {
		return
	}

	idStr := c.Param("id")
	cartItemID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id错误"})
		return
	}

	result := u.db.Where("id = ? AND cart_id = ?", cartItemID, cart.ID).Delete(&database.CartItem{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "购物车项不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "删除成功"})
}
