package redis

import (
	"PJS_Exchange/databases"
	"time"
)

type Price struct {
	Symbol    string      `json:"symbol"`
	Price     []float64   `json:"price"`
	Volume    []int64     `json:"volume"`
	Timestamp []time.Time `json:"timestamp"`
}

type PriceRepository struct {
	db *databases.RedisClient
}

func NewPriceRepository(client *databases.RedisClient) *PriceRepository {
	return &PriceRepository{db: client}
}

func (r *PriceRepository) SavePrice(price *Price) error {
	return nil
}
