package databases

import (
	"PJS_Exchange/utils"
	"context"
	"fmt"
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
	return &PostgresDBConfig{
		Host:            utils.GetEnv("USER_DB_HOST", "localhost"),
		Port:            utils.GetEnv("USER_DB_PORT", "5432"),
		User:            utils.GetEnv("USER_DB_USER", "postgres"),
		Password:        utils.GetEnv("USER_DB_PASSWORD", "1234"),
		DBName:          utils.GetEnv("USER_DB_NAME", "exchange_data"),
		SSLMode:         utils.GetEnv("USER_DB_SSLMODE", "disable"),
		MaxConns:        30,
		MinConns:        5,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: time.Minute * 30,
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
		fmt.Println("Failed to parse User DB config:", err)
		return nil, err
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		fmt.Println("Failed to create User DB pool:", err)
		return nil, err
	}

	// 연결 테스트
	err = pool.Ping(ctx)
	if err != nil {
		fmt.Println("Failed to connect to User DB:", err)
		pool.Close()
		return nil, err
	}

	fmt.Println("Connected to User DB successfully")
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
