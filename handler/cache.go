package handler

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

func delCache(rdb *redis.Client, ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
	}
}

func delCachePattern(rdb *redis.Client, ctx context.Context, pattern string) {
	if keys, err := rdb.Keys(ctx, pattern).Result(); err == nil {
		delCache(rdb, ctx, keys...)
	}
}

func productKey(id uint) string {
	return "product:" + strconv.FormatUint(uint64(id), 10)
}

// 失效单个商品 + 商品列表
func invalidateProductCache(rdb *redis.Client, ctx context.Context, ids ...uint) {
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, productKey(id))
	}
	delCache(rdb, ctx, keys...)
	delCachePattern(rdb, ctx, "products:list:*")
}

// 失效分类列表
func invalidateCategoryCache(rdb *redis.Client, ctx context.Context) {
	delCache(rdb, ctx, "categories")
}

// 分类变更：分类列表 + 所有商品缓存
func invalidateCategoryAndProductCache(rdb *redis.Client, ctx context.Context) {
	invalidateCategoryCache(rdb, ctx)
	delCachePattern(rdb, ctx, "products:list:*")
	delCachePattern(rdb, ctx, "product:*")
}
