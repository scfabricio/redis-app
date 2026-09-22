package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/scfabricio/redis-app/internal/cache"
	"github.com/scfabricio/redis-app/internal/config"
	"github.com/scfabricio/redis-app/internal/domain"
	"github.com/scfabricio/redis-app/internal/event"
	"github.com/scfabricio/redis-app/internal/repository"
	"github.com/scfabricio/redis-app/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("ping database", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Warn("redis unavailable; requests will use database", "error", err)
	}
	brands := service.NewBrandService(repository.NewPostgresBrandRepository(pool), cache.NewRedisBrandCache(redisClient), event.NewRedisPublisher(redisClient), logger)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /brands/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id must be a positive integer"})
			return
		}
		brand, source, err := brands.FindByID(r.Context(), id)
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "brand not found"})
			return
		}
		if err != nil {
			logger.Error("get brand", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		w.Header().Set("X-Data-Source", string(source))
		writeJSON(w, http.StatusOK, brand)
	})
	mux.HandleFunc("PUT /brands/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id must be a positive integer"})
			return
		}
		var payload struct {
			Name    string `json:"name"`
			Country string `json:"country"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be valid JSON with name and country"})
			return
		}
		if err := ensureSingleJSONValue(decoder); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must contain only one JSON object"})
			return
		}
		payload.Name = strings.TrimSpace(payload.Name)
		payload.Country = strings.TrimSpace(payload.Country)
		if payload.Name == "" || payload.Country == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and country are required"})
			return
		}

		brand, err := brands.UpdateByID(r.Context(), id, domain.Brand{Name: payload.Name, Country: payload.Country})
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "brand not found"})
			return
		}
		if err != nil {
			logger.Error("update brand", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, brand)
	})
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: logging(logger, mux), ReadHeaderTimeout: 5 * time.Second}
	logger.Info("api started", "address", cfg.HTTPAddr, "cache_ttl", service.BrandTTL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server", "error", err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("more than one JSON value")
	}
	return nil
}

func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started).String())
	})
}
