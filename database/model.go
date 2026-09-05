package database

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	OrderPending = iota // 待支付
	OrderPaid
	OrderShipped
	OrderCompleted
	OrderCancelled
)

type User struct {
	gorm.Model
	Name     string
	Email    string `gorm:"uniqueIndex;size:255"`
	Password string `gorm:"size:100"`
	Role     string // 管理员 顾客
}

type Product struct {
	gorm.Model
	Description string
	Name        string
	CategoryId  uint
	Category    Category        `gorm:"foreignKey:CategoryId"`
	Price       decimal.Decimal `gorm:"type:decimal(10,2)"`
	Number      int             // 库存
}

type Category struct {
	Name string `gorm:"uniqueIndex;size:100"`
	gorm.Model
}

type Cart struct {
	gorm.Model
	UserId uint `gorm:"uniqueIndex"`
	User   User `gorm:"foreignKey:UserId"`
}

type Order struct {
	gorm.Model
	Status int             // 订单状态，用常量表示
	Total  decimal.Decimal `gorm:"type:decimal(10,2)"`
	UserId uint            `gorm:"index"`
	User   User            `gorm:"foreignKey:UserId"`
}

type OrderItem struct {
	gorm.Model
	ProductId uint
	Product   Product         `gorm:"foreignKey:ProductId"`
	Price     decimal.Decimal `gorm:"type:decimal(10,2)"`
	Quantity  int
	OrderId   uint
	Order     Order `gorm:"foreignKey:OrderId"`
}

type CartItem struct {
	gorm.Model
	ProductId uint
	Product   Product `gorm:"foreignKey:ProductId"`
	Quantity  int
	CartId    uint
	Cart      Cart `gorm:"foreignKey:CartId"`
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Category{},  // 分类
		&Product{},   // 商品（依赖分类）
		&User{},      // 用户
		&Cart{},      // 购物车（依赖用户）
		&CartItem{},  // 购物车项（依赖购物车和商品）
		&Order{},     // 订单（依赖用户）
		&OrderItem{}, // 订单项（依赖订单和商品）
	)
}
