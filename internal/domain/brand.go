package domain

import "time"

// Brand represents a car manufacturer stored in the catalog.
type Brand struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Country   string    `json:"country"`
	CreatedAt time.Time `json:"createdAt"`
}
