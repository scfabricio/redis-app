package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/scfabricio/redis-app/internal/cache"
	"github.com/scfabricio/redis-app/internal/config"
	"github.com/scfabricio/redis-app/internal/event"
	"github.com/scfabricio/redis-app/internal/repository"
	"github.com/scfabricio/redis-app/internal/service"
)

type result struct {
	total, min, max, average time.Duration
	hits, misses             int
}

func main() {
	count := flag.Int("count", 100, "number of reads per scenario")
	flag.Parse()
	if *count < 1 {
		fmt.Fprintln(os.Stderr, "count must be positive")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		panic(fmt.Errorf("redis is required for benchmark: %w", err))
	}
	repo := repository.NewPostgresBrandRepository(pool)
	ids, err := repo.ListIDs(ctx, *count)
	if err != nil {
		panic(err)
	}
	if len(ids) == 0 {
		panic("no brands available")
	}
	ids = repeat(ids, *count)
	service := service.NewBrandService(repo, cache.NewRedisBrandCache(redisClient), event.NewRedisPublisher(redisClient), slog.New(slog.NewTextHandler(os.Stderr, nil)))
	fmt.Printf("Benchmark: %d queries; cache TTL: %s\n\n", len(ids), serviceTTL())
	print("database only", runDatabase(ctx, repo, ids))
	if err := service.ClearCache(ctx, ids); err != nil {
		panic(err)
	}
	print("cold cache", runService(ctx, service, ids))
	print("warm cache", runService(ctx, service, ids))
}

func repeat(ids []int64, count int) []int64 {
	result := make([]int64, count)
	for i := range result {
		result[i] = ids[i%len(ids)]
	}
	return result
}
func runDatabase(ctx context.Context, repo repository.BrandRepository, ids []int64) result {
	return measure(ids, ctx, repo, nil)
}
func runService(ctx context.Context, svc *service.BrandService, ids []int64) result {
	return measure(ids, ctx, nil, svc)
}
func measure(ids []int64, ctx context.Context, repo repository.BrandRepository, svc *service.BrandService) result {
	durations := make([]time.Duration, 0, len(ids))
	started := time.Now()
	hits, misses := 0, 0
	for _, id := range ids {
		one := time.Now()
		var source service.Source
		var err error
		if svc != nil {
			_, source, err = svc.FindByID(ctx, id)
		} else {
			_, err = repo.FindByID(ctx, id)
			source = service.SourceDatabase
		}
		if err != nil {
			panic(err)
		}
		durations = append(durations, time.Since(one))
		if source == service.SourceCache {
			hits++
		} else {
			misses++
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return result{total: time.Since(started), min: durations[0], max: durations[len(durations)-1], average: sum / time.Duration(len(durations)), hits: hits, misses: misses}
}
func print(name string, r result) {
	fmt.Printf("%-15s total=%-12s avg=%-12s min=%-12s max=%-12s hits=%d misses=%d\n", name, r.total, r.average, r.min, r.max, r.hits, r.misses)
}
func serviceTTL() time.Duration { return service.BrandTTL }
