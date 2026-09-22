package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/scfabricio/redis-app/internal/domain"
)

const BrandUpdatedChannel = "brand.updated"

// BrandUpdated is emitted after a brand has been persisted with new values.
type BrandUpdated struct {
	Type       string       `json:"type"`
	OccurredAt time.Time    `json:"occurredAt"`
	Brand      domain.Brand `json:"brand"`
}

type Publisher interface {
	PublishBrandUpdated(context.Context, BrandUpdated) error
}

type RedisPublisher struct{ client redis.UniversalClient }

func NewRedisPublisher(client redis.UniversalClient) *RedisPublisher {
	return &RedisPublisher{client: client}
}

func (p *RedisPublisher) PublishBrandUpdated(ctx context.Context, event BrandUpdated) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode brand updated event: %w", err)
	}
	if err := p.client.Publish(ctx, BrandUpdatedChannel, payload).Err(); err != nil {
		return fmt.Errorf("publish brand updated event: %w", err)
	}
	return nil
}
