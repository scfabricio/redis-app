package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scfabricio/redis-app/internal/domain"
)

type BrandCache interface {
	Get(context.Context, int64) (domain.Brand, bool, error)
	Set(context.Context, domain.Brand, time.Duration) error
	Delete(context.Context, int64) error
}

type RedisBrandCache struct{ client redis.UniversalClient }

func NewRedisBrandCache(client redis.UniversalClient) *RedisBrandCache {
	return &RedisBrandCache{client: client}
}
func brandKey(id int64) string { return "brand:" + strconv.FormatInt(id, 10) }

func (c *RedisBrandCache) Get(ctx context.Context, id int64) (domain.Brand, bool, error) {
	raw, err := c.client.Get(ctx, brandKey(id)).Bytes()
	if err == redis.Nil {
		return domain.Brand{}, false, nil
	}
	if err != nil {
		return domain.Brand{}, false, fmt.Errorf("redis get: %w", err)
	}
	var brand domain.Brand
	if err := json.Unmarshal(raw, &brand); err != nil {
		return domain.Brand{}, false, fmt.Errorf("decode cached brand: %w", err)
	}
	return brand, true, nil
}

func (c *RedisBrandCache) Set(ctx context.Context, brand domain.Brand, ttl time.Duration) error {
	raw, err := json.Marshal(brand)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, brandKey(brand.ID), raw, ttl).Err()
}
func (c *RedisBrandCache) Delete(ctx context.Context, id int64) error {
	return c.client.Del(ctx, brandKey(id)).Err()
}
