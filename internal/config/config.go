package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	HTTPAddr      string
}

type RedisConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

func Load() (Config, error) {
	redis, err := LoadRedis()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		RedisAddr:     redis.RedisAddr,
		RedisPassword: redis.RedisPassword,
		RedisDB:       redis.RedisDB,
		HTTPAddr:      valueOr("HTTP_ADDR", ":8080"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func LoadRedis() (RedisConfig, error) {
	cfg := RedisConfig{RedisAddr: valueOr("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD")}
	var err error
	if raw := valueOr("REDIS_DB", "0"); raw != "" {
		cfg.RedisDB, err = strconv.Atoi(raw)
		if err != nil {
			return RedisConfig{}, fmt.Errorf("invalid REDIS_DB: %w", err)
		}
	}
	return cfg, nil
}

func valueOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
