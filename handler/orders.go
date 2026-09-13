package handler

import (
	"ec/database"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type OrderHandler struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewOrderHandler(db *gorm.DB, rdb *redis.Client) *OrderHandler {
	return &OrderHandler{db: db, rdb: rdb}
}

type UpdateRequest struct {
	Status int `json:"status" binding:"required"`
}

// 1. 拿当前用户
// 2. 查他的购物车 + items（要 Preload Product，因为需要价格和库存）
// 3. 购物车是空的 → 400 "购物车为空"
// 4. 开事务 tx：
// a. 遍历每个 item：
// - 查商品库存，库存 < 数量 → return 错误（触发回滚）
// - 扣库存：库存 -= 数量
// - 算小计 = 商品价格 × 数量，累加进 total
// - 攒一条 order_item（价格用"下单这一刻的商品价"）
// b. 建 order（total、status=待支付）
// c. 建所有 order_item（关联刚建的 order id）
// d. 删掉这个用户的全部 cart_item（清空购物车）
// 5. 事务成功 → 返回订单

//下单

func (u *OrderHandler) Checkout(c *gin.Context) {
	userId, ok := getCurrentUserID(c)
	if ok != true {
		return
	}
	var userCart database.Cart
	if err := u.db.Preload("CartItems.Product").Where("user_id = ?", userId).First(&userCart).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "购物车为空"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
			return
		}
	}
	if len(userCart.CartItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "购物车为空"})
		return
	}
	//找到用户的购物车
	//开事务
	var order database.Order
	err := u.db.Transaction(func(tx *gorm.DB) error {
		//遍历购物车，检查库存
		total := decimal.Zero
		for _, item := range userCart.CartItems {
			if item.Quantity > item.Product.Number {
				return fmt.Errorf("商品 %s 库存不足", item.Product.Name)
			}
			subtotal := item.Product.Price.Mul(decimal.NewFromInt(int64(item.Quantity))) //小计=单价*数量
			total = total.Add(subtotal)
		}
		//建订单
		order = database.Order{
			Status: database.OrderPending, //待支付
			Total:  total,
			UserId: userId,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		//建订单明细（价格用下单那一刻的商品价格）
		for _, item := range userCart.CartItems {
			item.Product.Number -= item.Quantity //减库存
			if err := tx.Save(&item.Product).Error; err != nil {
				return err //回滚
			}
			oi := database.OrderItem{
				ProductId: item.ProductId,
				Price:     item.Product.Price,
				Quantity:  item.Quantity,
				OrderId:   order.ID,
			}
			if err := tx.Create(&oi).Error; err != nil {
				return err
			}
			order.OrderItems = append(order.OrderItems, oi)
		}
		//清空购物车（删掉这个购物车的所以cart_item）
		if err := tx.Where("cart_id = ?", userCart.ID).Delete(&database.CartItem{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 扣库存成功后再失效商品缓存
	ids := make([]uint, 0, len(userCart.CartItems))
	for _, item := range userCart.CartItems {
		ids = append(ids, item.ProductId)
	}
	invalidateProductCache(u.rdb, c.Request.Context(), ids...)
	c.JSON(http.StatusCreated, order)
}

//更新订单状态

func (u *OrderHandler) Update(c *gin.Context) {
	//id为订单id
	OrderIdString := c.Param("id")
	OrderId, err := strconv.ParseUint(OrderIdString, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Status > 4 || req.Status < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态码错误"})
		return
	}
	var Order database.Order
	if err := u.db.Where("id = ?", OrderId).First(&Order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
			return
		}
	}
	Order.Status = req.Status
	if err := u.db.Save(&Order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "更新成功"})
}

//查看所有订单

func (u *OrderHandler) List(c *gin.Context) {
	userId, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var userOrder []database.Order
	if err := u.db.Preload("OrderItems.Product").Where("user_id = ?", userId).Find(&userOrder).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	if len(userOrder) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户没有订单"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "查找成功", "order": userOrder})
}

//查看单个订单

func (u *OrderHandler) ListId(c *gin.Context) {
	//用户id
	userId, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	OrderIdString := c.Param("id")
	//用户请求查询的订单id
	OrderId, err := strconv.ParseUint(OrderIdString, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var userOrder database.Order
	//防越权
	if err := u.db.Preload("OrderItems.Product").Where("user_id = ? and id = ?", userId, OrderId).First(&userOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"status": "用户没有订单"})
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "查找成功", "order": userOrder})
}
