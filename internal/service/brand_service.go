package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/scfabricio/redis-app/internal/cache"
	"github.com/scfabricio/redis-app/internal/domain"
	"github.com/scfabricio/redis-app/internal/repository"
)

const BrandTTL = 10 * time.Second

type Source string

const (
	SourceCache    Source = "cache"
	SourceDatabase Source = "database"
)

type BrandService struct {
	repo   repository.BrandRepository
	cache  cache.BrandCache
	logger *slog.Logger
}

func NewBrandService(repo repository.BrandRepository, cache cache.BrandCache, logger *slog.Logger) *BrandService {
	return &BrandService{repo: repo, cache: cache, logger: logger}
}

func (s *BrandService) FindByID(ctx context.Context, id int64) (domain.Brand, Source, error) {
	brand, found, err := s.cache.Get(ctx, id)
	if err != nil {
		s.logger.Warn("cache unavailable; falling back to database", "error", err)
	} else if found {
		return brand, SourceCache, nil
	}
	brand, err = s.repo.FindByID(ctx, id)
	if err != nil {
		return domain.Brand{}, "", fmt.Errorf("find brand: %w", err)
	}
	if err := s.cache.Set(ctx, brand, BrandTTL); err != nil {
		s.logger.Warn("could not cache brand", "error", err)
	}
	return brand, SourceDatabase, nil
}

func (s *BrandService) ClearCache(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		if err := s.cache.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
