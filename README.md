# E-Commerce API

一个基于 Go + Gin + GORM + MySQL + Redis 的电商后端 API，完整实现了用户认证、角色鉴权、分类/商品管理、购物车、下单结账（事务）、订单管理、Redis 缓存等功能。

## 技术栈

| 类别 | 技术 |
|---|---|
| 语言 | Go 1.26 |
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) |
| ORM | [GORM](https://gorm.io) |
| 数据库 | MySQL |
| 缓存 | [go-redis](https://github.com/redis/go-redis) |
| 认证 | [golang-jwt](https://github.com/golang-jwt/jwt) + bcrypt |
| 金额计算 | [shopspring/decimal](https://github.com/shopspring/decimal) |
| 配置 | [godotenv](https://github.com/joho/godotenv) |

## 功能

- 用户注册 / 登录（JWT 认证，bcrypt 加密密码）
- 角色鉴权（admin / customer，中间件实现）
- 分类增删改查
- 商品增删改查（分页 + Redis 缓存）
- 购物车（加购 / 改数量 / 移除，含越权防护）
- 下单结账（事务 + 扣库存 + 订单明细价格快照）
- 订单查询 + 管理员改订单状态
- Redis 缓存（商品详情 / 列表 / 分类，写后失效 + 防缓存穿透）

## 快速开始

### 环境要求

- Go 1.26+
- MySQL
- Redis

### 配置

在项目根目录创建 `.env` 文件：

```env
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=你的密码
DB_NAME=ecommerce
REDIS_ADDR=localhost:6379
JWT_SECRET=你的密钥
```

### 运行

```bash
go mod tidy
go run .
```

默认监听 `:8080`，启动时自动建表（AutoMigrate）。

## API 接口

> 鉴权方式：登录后把返回的 `token` 放入请求头 `Authorization: Bearer <token>`。

| 模块 | 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|---|
| 认证 | POST | `/register` | 公开 | 注册 |
| 认证 | POST | `/login` | 公开 | 登录，返回 JWT |
| 认证 | GET | `/me` | 登录 | 当前用户信息 |
| 分类 | GET | `/categories` | 公开 | 分类列表 |
| 分类 | POST | `/categories` | admin | 新建分类 |
| 分类 | PUT | `/categories/:id` | admin | 更新分类 |
| 分类 | DELETE | `/categories/:id` | admin | 删除分类 |
| 商品 | GET | `/products` | 公开 | 商品列表（分页，缓存） |
| 商品 | GET | `/products/:id` | 公开 | 商品详情（缓存） |
| 商品 | POST | `/products` | admin | 新建商品 |
| 商品 | PUT | `/products/:id` | admin | 更新商品 |
| 商品 | DELETE | `/products/:id` | admin | 删除商品 |
| 购物车 | GET | `/cart` | 登录 | 查看购物车 |
| 购物车 | POST | `/cart/items` | 登录 | 加购 |
| 购物车 | PUT | `/cart/items/:id` | 登录 | 改数量 |
| 购物车 | DELETE | `/cart/items/:id` | 登录 | 移除 |
| 订单 | POST | `/orders` | 登录 | 下单结账 |
| 订单 | GET | `/orders` | 登录 | 我的订单列表 |
| 订单 | GET | `/orders/:id` | 登录 | 订单详情 |
| 订单 | PUT | `/orders/:id` | admin | 改订单状态 |

## 目录结构

```
ec/
├── main.go              # 入口：读配置 → 连库 → 迁移 → 起路由
├── config/              # 配置（读取 .env）
├── database/
│   ├── database.go      # 连接 MySQL / Redis
│   └── model.go         # GORM 模型 + AutoMigrate
├── auth/                # JWT Claims + 签发
├── middleware/          # Auth / RequireAdmin 中间件
├── handler/             # 各资源的 HTTP handler
│   ├── user.go          # 注册 / 登录 / me
│   ├── categories.go    # 分类 CRUD
│   ├── product.go       # 商品 CRUD + 缓存
│   ├── cart.go          # 购物车
│   └── orders.go        # 订单
└── router/              # 路由注册
```

## 数据模型

- `users`：用户，`role` 区分 admin / customer
- `categories`：分类
- `products`：商品，属于某个分类
- `carts` / `cart_items`：购物车 + 明细
- `orders` / `order_items`：订单 + 明细（明细存价格快照）

## 关键设计

- **事务**：下单时「检查库存 → 建订单 → 扣库存 → 建明细 → 清空购物车」整体原子，任一步失败全部回滚。
- **价格快照**：`order_item` 存下单那一刻的价格，商品日后改价不影响历史订单。
- **越权防护**：购物车 / 订单的写操作按「资源 id + 归属（user_id / cart_id）」双重限定。
- **金额用 decimal**：避免 float 精度误差。
- **Redis Cache-Aside**：读缓存 → 未命中查库写回；写操作删缓存（含列表），并缓存空值防穿透。
