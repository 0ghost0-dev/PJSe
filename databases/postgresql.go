package databases

import (
	"PJS_Exchange/utils"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDBConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

func NewPostgresDBConfig() *PostgresDBConfig {
	maxConns, err := strconv.Atoi(utils.GetEnv("POSTGRES_DB_MAX_CONNS", "20"))
	if err != nil {
		maxConns = 20
	}
	minConns, err := strconv.Atoi(utils.GetEnv("POSTGRES_DB_MIN_CONNS", "5"))
	if err != nil {
		minConns = 5
	}
	macConnsLifetime, err := strconv.Atoi(utils.GetEnv("POSTGRES_DB_MAX_CONN_LIFETIME", "3600"))
	if err != nil {
		macConnsLifetime = 3600
	}
	maxConnIdleTime, err := strconv.Atoi(utils.GetEnv("POSTGRES_DB_MAX_CONN_IDLE_TIME", "1800"))
	if err != nil {
		maxConnIdleTime = 1800
	}

	return &PostgresDBConfig{
		Host:            utils.GetEnv("POSTGRES_DB_HOST", "localhost"),
		Port:            utils.GetEnv("POSTGRES_DB_PORT", "5432"),
		User:            utils.GetEnv("POSTGRES_DB_USER", "postgresql"),
		Password:        utils.GetEnv("POSTGRES_DB_PASSWORD", "1234"),
		DBName:          utils.GetEnv("POSTGRES_DB_NAME", "exchange_data"),
		SSLMode:         utils.GetEnv("POSTGRES_DB_SSLMODE", "disable"),
		MaxConns:        int32(maxConns),
		MinConns:        int32(minConns),
		MaxConnLifetime: time.Second * time.Duration(macConnsLifetime),
		MaxConnIdleTime: time.Second * time.Duration(maxConnIdleTime),
	}
}

func (cfg *PostgresDBConfig) getConnectionString() string {
	return "postgresql://" + cfg.User + ":" + cfg.Password + "@" + cfg.Host + ":" + cfg.Port + "/" + cfg.DBName + "?sslmode=" + cfg.SSLMode
}

// PostgresDBPool 데이터 베이스 풀 구성
type PostgresDBPool struct {
	pool *pgxpool.Pool
}

func NewPostgresDBPool(cfg *PostgresDBConfig) (*PostgresDBPool, error) {
	ctx := context.Background()

	poolConfig, err := pgxpool.ParseConfig(cfg.getConnectionString())
	if err != nil {
		fmt.Println("Failed to parse Postgres DB config:", err)
		return nil, err
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		fmt.Println("Failed to create Postgres DB pool:", err)
		return nil, err
	}

	// 연결 테스트
	err = pool.Ping(ctx)
	if err != nil {
		fmt.Println("Failed to connect to Postgres DB:", err)
		pool.Close()
		return nil, err
	}

	fmt.Println("Connected to Postgres DB successfully")
	return &PostgresDBPool{pool: pool}, nil
}

func (db *PostgresDBPool) Close() {
	db.pool.Close()
}

func (db *PostgresDBPool) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *PostgresDBPool) GetPool() *pgxpool.Pool {
	return db.pool
}
