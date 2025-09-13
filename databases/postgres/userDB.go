package postgres

import (
	"PJS_Exchange/databases"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"Email"`
	Password  string    `json:"Password"` // Must be hashed
	CreatedAt time.Time `json:"CreatedAt"`
	Enabled   bool      `json:"enabled"`
	APIKey    uuid.UUID `json:"api_key"`
}

type UserRepository interface {
	CreateUser(ctx context.Context, username string, email string, password string) (*User, error)
	NewAPIKey(ctx context.Context, userID int) (string, error)
	isAPIKeyValid(apiKey string) bool
	IsUserEnabled(ctx context.Context, userID int) (bool, error)
	EnableUser(ctx context.Context, userID int) error
	DisableUser(ctx context.Context, userID int) error
}

type UserDBRepository struct {
	db *databases.PostgresDBPool
}

func NewUserRepository(db *databases.PostgresDBPool) *UserDBRepository {
	return &UserDBRepository{db: db}
}

// CreateUsersTable users 테이블을 생성합니다.
func (r *UserDBRepository) CreateUsersTable(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) NOT NULL,
	    email VARCHAR(255) UNIQUE NOT NULL,
	    password VARCHAR(255) NOT NULL,
	    created_at TIMESTAMP NOT NULL,
	    enabled BOOLEAN NOT NULL DEFAULT FALSE,
	    api_key UUID
	);`

	_, err := r.db.GetPool().Exec(ctx, query)
	if err != nil {
		fmt.Println("Failed to create users table:", err)
		return err
	}
	return nil
}

// CreateUser 유저를 생성합니다. *비밀번호 해싱은 여기서 하지 않음, 반드시 해싱된 비밀번호를 전달해야 함*
func (r *UserDBRepository) CreateUser(ctx context.Context, username string, email string, password string) (*User, error) {
	// 이메일이나 사용자 이름이 이미 존재하는지 확인
	var exists bool
	errE := r.db.GetPool().QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email=$1 OR username=$2)", email, username).Scan(&exists)

	if errE != nil {
		fmt.Println("Failed to check existing user:", errE)
		return nil, errE
	}
	if exists {
		return nil, fmt.Errorf("user with given email or username already exists")
	}

	// 사용자 생성
	query := `INSERT INTO users (username, email, password, created_at, enabled) VALUES ($1, $2, $3, $4, $5) RETURNING id , username, email, password, created_at, enabled;`
	user := &User{}
	err := r.db.GetPool().QueryRow(ctx, query, username, email, password, time.Now(), false).Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt, &user.Enabled)

	if err != nil {
		fmt.Println("Failed to create user:", err)
		return nil, err
	}
	return user, nil
}

// NewAPIKey 활성화된 계정에 대해 새로운 API 키를 생성합니다. 계정이 활성화되지 않은 경우 오류를 반환합니다. *로그인한 사용자만 호출해야 함*
func (r *UserDBRepository) NewAPIKey(ctx context.Context, userID int) (string, error) {
	// 계정이 활성화 되었는지 확인
	enabled, _ := r.IsUserEnabled(ctx, userID)
	if !enabled {
		return "", fmt.Errorf("user account is not enabled")
	}

	// API 키 생성
	apiKey, _ := uuid.NewUUID()
	// 데이터 베이스에 API 키 저장
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET api_key=$1 WHERE id=$2", apiKey, userID)
	if err != nil {
		fmt.Println("Failed to store new API key:", err)
		return "", err
	}
	return apiKey.String(), nil
}

// isAPIKeyValid 주어진 API 키가 유효한지 확인합니다.
func (r *UserDBRepository) isAPIKeyValid(apiKey string) bool {
	parse, err := uuid.Parse(apiKey)
	if err != nil {
		return false
	}

	// 데이터 베이스에서 API 키 존재 여부 확인
	query := `SELECT COUNT(*) FROM users WHERE api_key=$1`
	var count int
	err = r.db.GetPool().QueryRow(context.Background(), query, parse).Scan(&count)
	if err != nil {
		fmt.Println("Failed to validate API key:", err)
		return false
	}
	return count > 0
}

func (r *UserDBRepository) IsUserEnabled(ctx context.Context, userID int) (bool, error) {
	var enabled bool
	err := r.db.GetPool().QueryRow(ctx, "SELECT enabled FROM users WHERE id=$1", userID).Scan(&enabled)
	if err != nil {
		fmt.Println("Failed to check user enabled status:", err)
		return false, err
	}
	return enabled, nil
}

func (r *UserDBRepository) EnableUser(ctx context.Context, userID int) error {
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET enabled=true WHERE id=$1", userID)
	if err != nil {
		fmt.Println("Failed to enable user:", err)
		return err
	}
	return nil
}

func (r *UserDBRepository) DisableUser(ctx context.Context, userID int) error {
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET enabled=false WHERE id=$1", userID)
	if err != nil {
		fmt.Println("Failed to disable user:", err)
		return err
	}
	return nil
}
