package caches

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	hits   int64
	misses int64
}

func NewRedisCache(addr string) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 5 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		return nil, err
	}
	return &RedisCache{client: rdb}, nil
}

func (r *RedisCache) Set(key string, value []byte, _ int64) bool {
	ctx := context.Background()
	err := r.client.Set(ctx, key, value, 0).Err()
	return err == nil
}

func (r *RedisCache) Get(key string) ([]byte, bool) {
	ctx := context.Background()
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		r.misses++
		return nil, false
	} else if err != nil {
		r.misses++
		return nil, false
	}
	r.hits++
	return []byte(val), true
}

func (r *RedisCache) Close() error {
	return r.client.Close()
}

func (r *RedisCache) Metrics() Metrics {
	return Metrics{
		HitCount:  r.hits,
		MissCount: r.misses,
		// Redis doesn't expose evictions easily in this mode
		EvictionCount: 0,
	}
}

func (r *RedisCache) Name() string { return "Redis" }
