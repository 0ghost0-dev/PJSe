package postgresql

import (
	"PJS_Exchange/databases"
	"context"
	"fmt"
	"regexp"
	"time"
)

type AcceptCodeValidator interface {
	ValidateAcceptCode(ctx context.Context, code string) (bool, bool, error)
	RelationShipUser(ctx context.Context, code string, userID int) error
}

const (
	AccTypeNormal  = 0
	AccTypePremium = 1
	AccTypeAdmin   = 2
)

type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"Email"`
	Password  string    `json:"Password"` // Must be hashed
	CreatedAt time.Time `json:"CreatedAt"`
	Admin     bool      `json:"admin"`
	Type      int       `json:"type"`
	Enabled   bool      `json:"enabled"`
	//APIKey    uuid.UUID `json:"api_key"`
}

type UserRepository interface {
	CreateUser(ctx context.Context, username string, email string, password string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, userID int) (*User, error)
	GetUserIDs(ctx context.Context) (map[int]string, error)
	IsUserEnabled(ctx context.Context, userID int) (bool, error)
	EnableUser(ctx context.Context, userID int) error
	DisableUser(ctx context.Context, userID int) error
}

type UserDBRepository struct {
	db         *databases.PostgresDBPool
	acceptRepo AcceptCodeValidator
}

func NewUserRepository(db *databases.PostgresDBPool, acceptRepo AcceptCodeValidator) *UserDBRepository {
	return &UserDBRepository{db: db, acceptRepo: acceptRepo}
}

// CreateUsersTable users 테이블을 생성합니다.
func (r *UserDBRepository) CreateUsersTable(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(255) NOT NULL,
	    email VARCHAR(255) UNIQUE NOT NULL,
	    password VARCHAR(255) NOT NULL,
	    created_at TIMESTAMPTZ NOT NULL,
    	admin BOOLEAN NOT NULL DEFAULT FALSE,
    	type INT NOT NULL DEFAULT 0,
	    enabled BOOLEAN NOT NULL DEFAULT FALSE
-- 	    api_key UUID
	);`

	_, err := r.db.GetPool().Exec(ctx, query)
	if err != nil {
		fmt.Println("Failed to create users table:", err)
		return err
	}
	return nil
}

func (r *UserDBRepository) validateEmail(email string) error {
	// 더 엄격한 이메일 패턴 (RFC 5322 기준)
	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	if !emailPattern.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// CreateUser 유저를 생성합니다. *비밀번호 해싱은 여기서 하지 않음, 반드시 해싱된 비밀번호를 전달해야 함*
func (r *UserDBRepository) CreateUser(ctx context.Context, username string, email string, password string, acceptCode string) (*User, error) {
	// 입력 검증을 먼저 수행 (DB 쿼리 전에)
	if err := r.validateEmail(email); err != nil {
		return nil, err
	}

	// 중복 확인 쿼리 최적화 (인덱스 활용)
	var exists bool
	checkQuery := "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 OR username = $2)"
	if err := r.db.GetPool().QueryRow(ctx, checkQuery, email, username).Scan(&exists); err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("user with given email or username already exists")
	}

	// acceptCodeDB에 있는 acceptCode가 유효한지 확인
	accept, admin, err := r.acceptRepo.ValidateAcceptCode(ctx, acceptCode)
	if err != nil {
		return nil, fmt.Errorf("failed to validate accept code: %w", err)
	}

	if accept {
		// 사용자 생성
		user := &User{
			Username: username,
			Email:    email,
			Password: password, // 호출자가 이미 해싱했다고 가정
			Admin:    admin,
			Type: func() int {
				if admin {
					return AccTypeAdmin
				}
				return AccTypeNormal
			}(),
			Enabled: false,
		}

		insertQuery := `
		INSERT INTO users (username, email, password, created_at, enabled, admin, type) 
		VALUES ($1, $2, $3, NOW(), $4 ,$5, $6)
		RETURNING id, username, email, password, created_at, enabled, admin, type;`

		err := r.db.GetPool().QueryRow(ctx, insertQuery,
			user.Username, user.Email, user.Password, user.Enabled, user.Admin, user.Type).
			Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt, &user.Enabled, &user.Admin, &user.Type)

		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// acceptCode와 userID 연결
		err = r.acceptRepo.RelationShipUser(ctx, acceptCode, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to relate accept code with user: %w", err)
		}

		return user, nil
	}
	return nil, fmt.Errorf("invalid or used accept code")
}

func (r *UserDBRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := r.db.GetPool().QueryRow(ctx, "SELECT id, username, email, password, created_at, enabled, admin, type FROM users WHERE email=$1", email).
		Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt, &user.Enabled, &user.Admin, &user.Type)
	if err != nil {
		fmt.Println("Failed to get user by email:", err)
		return nil, err
	}
	return user, nil
}

func (r *UserDBRepository) GetUserByID(ctx context.Context, userID int) (*User, error) {
	user := &User{}
	err := r.db.GetPool().QueryRow(ctx, "SELECT id, username, email, password, created_at, enabled, admin, type FROM users WHERE id=$1", userID).
		Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.CreatedAt, &user.Enabled, &user.Admin, &user.Type)
	if err != nil {
		fmt.Println("Failed to get user by ID:", err)
		return nil, err
	}
	return user, nil
}

func (r *UserDBRepository) GetUserIDs(ctx context.Context) (map[int][]string, error) {
	rows, err := r.db.GetPool().Query(ctx, "SELECT id, username, enabled FROM users")
	if err != nil {
		fmt.Println("Failed to get user IDs:", err)
		return nil, err
	}
	defer rows.Close()
	userMap := make(map[int][]string)
	for rows.Next() {
		var id int
		var username string
		var enabled bool
		if err := rows.Scan(&id, &username, &enabled); err != nil {
			fmt.Println("Failed to scan user row:", err)
			return nil, err
		}
		userMap[id] = []string{username, fmt.Sprintf("%v", enabled)}
	}
	return userMap, nil
}

func (r *UserDBRepository) SetUsername(ctx context.Context, userID int, newUsername string) error {
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET username=$1 WHERE id=$2", newUsername, userID)
	if err != nil {
		fmt.Println("Failed to set username:", err)
		return err
	}
	return nil
}

func (r *UserDBRepository) SetEmail(ctx context.Context, userID int, newEmail string) error {
	// 이메일 형식 검증
	if err := r.validateEmail(newEmail); err != nil {
		return err
	}

	// 중복 확인 쿼리 최적화 (인덱스 활용)
	var exists bool
	checkQuery := "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id <> $2)"
	if err := r.db.GetPool().QueryRow(ctx, checkQuery, newEmail, userID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check existing email: %w", err)
	}

	if exists {
		return fmt.Errorf("email already in use by another user")
	}

	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET email=$1 WHERE id=$2", newEmail, userID)
	if err != nil {
		fmt.Println("Failed to set email:", err)
		return err
	}
	return nil
}

// SetUserPassword *비밀번호 해싱은 여기서 하지 않음, 반드시 해싱된 비밀번호를 전달해야 함*
func (r *UserDBRepository) SetUserPassword(ctx context.Context, userID int, newHashedPassword string) error {
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET password=$1 WHERE id=$2", newHashedPassword, userID)
	if err != nil {
		fmt.Println("Failed to set user password:", err)
		return err
	}
	return nil
}

func (r *UserDBRepository) SetUserAdminStatus(ctx context.Context, userID int, isAdmin bool) error {
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET admin=$1, type=$3 WHERE id=$2", isAdmin, userID, func() int {
		if isAdmin {
			return AccTypeAdmin
		}
		return AccTypeNormal
	})
	if err != nil {
		fmt.Println("Failed to set user admin status:", err)
		return err
	}
	return nil
}

func (r *UserDBRepository) SetUserType(ctx context.Context, userID int, userType int) error {
	_, err := r.db.GetPool().Exec(ctx, "UPDATE users SET type=$1 WHERE id=$2 AND admin=false", userType, userID)
	if err != nil {
		fmt.Println("Failed to set user type:", err)
		return err
	}
	return nil
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
