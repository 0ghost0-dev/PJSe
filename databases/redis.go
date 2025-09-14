package databases

import (
	"PJS_Exchange/utils"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient() *RedisClient {
	dbNum, err := strconv.Atoi(utils.GetEnv("REDIS_DB", "0"))
	if err != nil {
		dbNum = 0
	}
	poolSize, err := strconv.Atoi(utils.GetEnv("REDIS_POOL_SIZE", "10"))
	if err != nil {
		poolSize = 10
	}
	minIdleConns, err := strconv.Atoi(utils.GetEnv("REDIS_MIN_IDLE_CONNS", "5"))
	if err != nil {
		minIdleConns = 5
	}
	maxRetries, err := strconv.Atoi(utils.GetEnv("REDIS_MAX_RETRIES", "3"))
	if err != nil {
		maxRetries = 3
	}
	dialTimeoutSec, err := strconv.Atoi(utils.GetEnv("REDIS_DIAL_TIMEOUT", "5"))
	if err != nil {
		dialTimeoutSec = 5
	}
	readTimeoutSec, err := strconv.Atoi(utils.GetEnv("REDIS_READ_TIMEOUT", "3"))
	if err != nil {
		readTimeoutSec = 3
	}
	writeTimeoutSec, err := strconv.Atoi(utils.GetEnv("REDIS_WRITE_TIMEOUT", "3"))
	if err != nil {
		writeTimeoutSec = 3
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         utils.GetEnv("REDIS_ADDR", "localhost:6379"),
		Username:     utils.GetEnv("REDIS_USERNAME", ""),
		Password:     utils.GetEnv("REDIS_PASSWORD", ""),
		DB:           dbNum,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
		MaxRetries:   maxRetries,
		DialTimeout:  time.Duration(dialTimeoutSec) * time.Second,
		ReadTimeout:  time.Duration(readTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(writeTimeoutSec) * time.Second,
	})

	ctx := context.Background()

	// 연결 테스트
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("Redis 연결 실패: %v", err))
	}

	return &RedisClient{
		client: rdb,
		ctx:    ctx,
	}
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}
