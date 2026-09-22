package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/scfabricio/redis-app/internal/config"
	"github.com/scfabricio/redis-app/internal/event"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.LoadRedis()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	defer client.Close()
	pubsub := client.Subscribe(ctx, event.BrandUpdatedChannel)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		logger.Error("subscribe to brand events", "error", err)
		os.Exit(1)
	}
	logger.Info("subscribed to brand events", "channel", event.BrandUpdatedChannel)

	for {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Error("receive brand event", "error", err)
			}
			return
		}
		var received event.BrandUpdated
		if err := json.Unmarshal([]byte(message.Payload), &received); err != nil {
			logger.Warn("invalid brand event", "error", err, "payload", message.Payload)
			continue
		}
		logger.Info("brand event received", "type", received.Type, "occurred_at", received.OccurredAt, "brand", received.Brand)
	}
}
