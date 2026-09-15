package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/scfabricio/redis-app/internal/domain"
	"github.com/scfabricio/redis-app/internal/repository"
)

type fakeBrandRepository struct {
	found           domain.Brand
	findErr         error
	findCalls       int
	receivedFindID  int64
	updated         domain.Brand
	updateErr       error
	updateCalls     int
	receivedID      int64
	receivedPayload domain.Brand
}

func (f *fakeBrandRepository) FindByID(_ context.Context, id int64) (domain.Brand, error) {
	f.findCalls++
	f.receivedFindID = id
	return f.found, f.findErr
}

func (f *fakeBrandRepository) ListIDs(context.Context, int) ([]int64, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeBrandRepository) UpdateByID(_ context.Context, id int64, payload domain.Brand) (domain.Brand, error) {
	f.updateCalls++
	f.receivedID = id
	f.receivedPayload = payload
	return f.updated, f.updateErr
}

type fakeBrandCache struct {
	getBrand    domain.Brand
	getFound    bool
	getErr      error
	getCalls    int
	requestedID int64
	setErr      error
	setCalls    int
	setBrand    domain.Brand
	setTTL      time.Duration
	deleteErr   error
	deleteCalls int
	deletedID   int64
}

func (f *fakeBrandCache) Get(_ context.Context, id int64) (domain.Brand, bool, error) {
	f.getCalls++
	f.requestedID = id
	return f.getBrand, f.getFound, f.getErr
}

func (f *fakeBrandCache) Set(_ context.Context, brand domain.Brand, ttl time.Duration) error {
	f.setCalls++
	f.setBrand = brand
	f.setTTL = ttl
	return f.setErr
}

func (f *fakeBrandCache) Delete(_ context.Context, id int64) error {
	f.deleteCalls++
	f.deletedID = id
	return f.deleteErr
}

func TestBrandServiceUpdateByID(t *testing.T) {
	t.Parallel()

	updated := domain.Brand{ID: 7, Name: "Toyota Motor", Country: "Japan"}
	payload := domain.Brand{Name: "Toyota Motor", Country: "Japan"}
	databaseFailure := errors.New("database unavailable")
	cacheFailure := errors.New("redis unavailable")

	tests := []struct {
		name            string
		repositoryErr   error
		cacheErr        error
		want            domain.Brand
		wantErr         error
		wantDeleteCalls int
	}{
		{
			name:            "updates brand and invalidates its cache entry",
			want:            updated,
			wantDeleteCalls: 1,
		},
		{
			name:            "returns repository not found error without deleting cache",
			repositoryErr:   repository.ErrNotFound,
			wantErr:         repository.ErrNotFound,
			wantDeleteCalls: 0,
		},
		{
			name:            "returns repository failure without deleting cache",
			repositoryErr:   databaseFailure,
			wantErr:         databaseFailure,
			wantDeleteCalls: 0,
		},
		{
			name:            "returns updated brand when cache invalidation fails",
			cacheErr:        cacheFailure,
			want:            updated,
			wantDeleteCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeBrandRepository{updated: updated, updateErr: tt.repositoryErr}
			cache := &fakeBrandCache{deleteErr: tt.cacheErr}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := NewBrandService(repo, cache, logger)

			got, err := svc.UpdateByID(context.Background(), updated.ID, payload)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("UpdateByID() error = %v, want error matching %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateByID() = %#v, want %#v", got, tt.want)
			}
			if repo.updateCalls != 1 {
				t.Errorf("repository calls = %d, want 1", repo.updateCalls)
			}
			if repo.receivedID != updated.ID {
				t.Errorf("repository ID = %d, want %d", repo.receivedID, updated.ID)
			}
			if !reflect.DeepEqual(repo.receivedPayload, payload) {
				t.Errorf("repository payload = %#v, want %#v", repo.receivedPayload, payload)
			}
			if cache.deleteCalls != tt.wantDeleteCalls {
				t.Errorf("cache delete calls = %d, want %d", cache.deleteCalls, tt.wantDeleteCalls)
			}
			if tt.wantDeleteCalls == 1 && cache.deletedID != updated.ID {
				t.Errorf("cache deleted ID = %d, want %d", cache.deletedID, updated.ID)
			}
		})
	}
}

func TestBrandServiceFindByID(t *testing.T) {
	t.Parallel()

	id := int64(7)
	cached := domain.Brand{ID: id, Name: "Toyota", Country: "Japan"}
	fromDatabase := domain.Brand{ID: id, Name: "Toyota Motor", Country: "Japan"}
	redisFailure := errors.New("redis unavailable")
	databaseFailure := errors.New("database unavailable")
	cacheSetFailure := errors.New("could not cache")

	tests := []struct {
		name            string
		cacheBrand      domain.Brand
		cacheFound      bool
		cacheGetErr     error
		repositoryBrand domain.Brand
		repositoryErr   error
		cacheSetErr     error
		wantBrand       domain.Brand
		wantSource      Source
		wantErr         error
		wantFindCalls   int
		wantSetCalls    int
	}{
		{
			name:       "returns a cache hit without querying database",
			cacheBrand: cached,
			cacheFound: true,
			wantBrand:  cached,
			wantSource: SourceCache,
		},
		{
			name:            "queries database and fills cache on cache miss",
			repositoryBrand: fromDatabase,
			wantBrand:       fromDatabase,
			wantSource:      SourceDatabase,
			wantFindCalls:   1,
			wantSetCalls:    1,
		},
		{
			name:            "falls back to database when cache is unavailable",
			cacheGetErr:     redisFailure,
			repositoryBrand: fromDatabase,
			wantBrand:       fromDatabase,
			wantSource:      SourceDatabase,
			wantFindCalls:   1,
			wantSetCalls:    1,
		},
		{
			name:          "returns database error without filling cache",
			repositoryErr: databaseFailure,
			wantErr:       databaseFailure,
			wantFindCalls: 1,
		},
		{
			name:            "returns database result when cache fill fails",
			repositoryBrand: fromDatabase,
			cacheSetErr:     cacheSetFailure,
			wantBrand:       fromDatabase,
			wantSource:      SourceDatabase,
			wantFindCalls:   1,
			wantSetCalls:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeBrandRepository{found: tt.repositoryBrand, findErr: tt.repositoryErr}
			cache := &fakeBrandCache{
				getBrand: tt.cacheBrand,
				getFound: tt.cacheFound,
				getErr:   tt.cacheGetErr,
				setErr:   tt.cacheSetErr,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := NewBrandService(repo, cache, logger)

			gotBrand, gotSource, err := svc.FindByID(context.Background(), id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("FindByID() error = %v, want error matching %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(gotBrand, tt.wantBrand) {
				t.Errorf("FindByID() brand = %#v, want %#v", gotBrand, tt.wantBrand)
			}
			if gotSource != tt.wantSource {
				t.Errorf("FindByID() source = %q, want %q", gotSource, tt.wantSource)
			}
			if cache.getCalls != 1 || cache.requestedID != id {
				t.Errorf("cache Get() calls/id = %d/%d, want 1/%d", cache.getCalls, cache.requestedID, id)
			}
			if repo.findCalls != tt.wantFindCalls {
				t.Errorf("repository FindByID() calls = %d, want %d", repo.findCalls, tt.wantFindCalls)
			}
			if tt.wantFindCalls == 1 && repo.receivedFindID != id {
				t.Errorf("repository FindByID() ID = %d, want %d", repo.receivedFindID, id)
			}
			if cache.setCalls != tt.wantSetCalls {
				t.Errorf("cache Set() calls = %d, want %d", cache.setCalls, tt.wantSetCalls)
			}
			if tt.wantSetCalls == 1 {
				if !reflect.DeepEqual(cache.setBrand, fromDatabase) {
					t.Errorf("cache Set() brand = %#v, want %#v", cache.setBrand, fromDatabase)
				}
				if cache.setTTL != BrandTTL {
					t.Errorf("cache Set() TTL = %s, want %s", cache.setTTL, BrandTTL)
				}
			}
		})
	}
}
