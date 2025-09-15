package postgresql

import (
	"PJS_Exchange/databases"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AcceptCode struct {
	ID         int        `json:"id"`
	CodeID     string     `json:"code_id"`
	AcceptCode string     `json:"accept_code"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Used       bool       `json:"used"`
	Admin      bool       `json:"admin"`
	UserID     *int       `json:"user_id"`
}

type AcceptCodeRepository interface {
	GenerateAcceptCode(ctx context.Context, userID int) (*AcceptCode, error)
	GenerateAcceptCodeForAdmin(ctx context.Context) (*AcceptCode, error)
	ValidateAcceptCode(ctx context.Context, code string) (bool, error)
	RelationShipUser(ctx context.Context, code string, userID int) error
}

type AcceptCodeDBRepository struct {
	db *databases.PostgresDBPool
}

func NewAcceptCodeRepository(db *databases.PostgresDBPool) *AcceptCodeDBRepository {
	return &AcceptCodeDBRepository{db: db}
}

// CreateAcceptCodesTable users 테이블을 생성합니다.
func (r *AcceptCodeDBRepository) CreateAcceptCodesTable(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS accept_codes (
    id SERIAL PRIMARY KEY,
    code_id VARCHAR(255) UNIQUE NOT NULL,
    accept_code VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    admin BOOLEAN NOT NULL DEFAULT FALSE,
    user_id INTEGER REFERENCES users(id)
    );`

	_, err := r.db.GetPool().Exec(ctx, query)
	if err != nil {
		log.Println("Failed to create users table:", err)
		return err
	}
	return nil
}

func generateRandomCode(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}

func (r *AcceptCodeDBRepository) GenerateAcceptCode(ctx context.Context, expiresAt *time.Time) (string, *AcceptCode, error) {
	code, err := generateRandomCode(16)
	if err != nil {
		return "", nil, err
	}

	// 고유 식별자 생성
	newUUID, _ := uuid.NewUUID()
	codeID := "pjse-accept-" + newUUID.String()

	// 실제 반환할 코드는 "식별자:원본코드" 형태
	fullCode := fmt.Sprintf("%s:%s", codeID, code)

	// bcrypt 해싱은 원본 코드만
	hashedCode, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	query := `INSERT INTO accept_codes (code_id, accept_code, expires_at) VALUES ($1, $2, $3) RETURNING id, code_id, accept_code, created_at, expires_at, used, user_id;`
	acceptCode := &AcceptCode{}
	err = r.db.GetPool().QueryRow(ctx, query, codeID, string(hashedCode), expiresAt).Scan(
		&acceptCode.ID, &acceptCode.CodeID, &acceptCode.AcceptCode,
		&acceptCode.CreatedAt, &acceptCode.ExpiresAt, &acceptCode.Used, &acceptCode.UserID)
	if err != nil {
		return "", nil, err
	}

	return fullCode, acceptCode, nil
}

func (r *AcceptCodeDBRepository) GenerateAcceptCodeForAdmin(ctx context.Context, expiresAt *time.Time) (string, *AcceptCode, error) {
	code, err := generateRandomCode(16)
	if err != nil {
		return "", nil, err
	}

	// 고유 식별자 생성
	newUUID, _ := uuid.NewUUID()
	codeID := "pjse-accept-" + newUUID.String()

	// 실제 반환할 코드는 "식별자:원본코드" 형태
	fullCode := fmt.Sprintf("%s:%s", codeID, code)

	// bcrypt 해싱은 원본 코드만
	hashedCode, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	query := `INSERT INTO accept_codes (code_id, accept_code, expires_at, admin) VALUES ($1, $2, $3, TRUE) RETURNING id, code_id, accept_code, created_at, expires_at, used, user_id;`
	acceptCode := &AcceptCode{}
	err = r.db.GetPool().QueryRow(ctx, query, codeID, string(hashedCode), expiresAt).Scan(
		&acceptCode.ID, &acceptCode.CodeID, &acceptCode.AcceptCode,
		&acceptCode.CreatedAt, &acceptCode.ExpiresAt, &acceptCode.Used, &acceptCode.UserID)
	if err != nil {
		return "", nil, err
	}

	return fullCode, acceptCode, nil
}

func (r *AcceptCodeDBRepository) ValidateAcceptCode(ctx context.Context, fullCode string) (bool, bool, error) {
	parts := strings.Split(fullCode, ":")
	if len(parts) != 2 {
		return false, false, fmt.Errorf("invalid code format")
	}

	codeID := parts[0]
	code := parts[1]

	var storedHash string
	var expiresAt time.Time
	var used bool
	var admin bool

	err := r.db.GetPool().QueryRow(ctx,
		"SELECT accept_code, expires_at, used, admin FROM accept_codes WHERE code_id = $1",
		codeID).Scan(&storedHash, &expiresAt, &used, &admin)
	if err != nil {
		return false, false, err
	}

	if used {
		return false, false, fmt.Errorf("code already used")
	}

	if time.Now().After(expiresAt) {
		return false, false, fmt.Errorf("accept code has expired")
	}

	// bcrypt로 실제 코드 비교
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(code))
	if err != nil {
		return false, false, err
	}

	// 사용됨으로 표시
	_, err = r.db.GetPool().Exec(ctx, "UPDATE accept_codes SET used = TRUE WHERE code_id = $1", codeID)
	if err != nil {
		return false, false, err
	}

	return true, admin, nil
}

func (r *AcceptCodeDBRepository) RelationShipUser(ctx context.Context, code string, userID int) error {
	// code 형식: codeID:rawCode (GenerateAcceptCode 반환 포맷과 동일)
	parts := strings.Split(code, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid code format")
	}
	codeID := parts[0]
	rawCode := parts[1]

	var (
		storedHash   string
		used         bool
		existingUser *int
	)

	err := r.db.GetPool().QueryRow(ctx,
		"SELECT accept_code, used, user_id FROM accept_codes WHERE code_id = $1",
		codeID,
	).Scan(&storedHash, &used, &existingUser)
	if err != nil {
		return err
	}

	if !used {
		return fmt.Errorf("code must be validated first")
	}
	if existingUser != nil {
		return fmt.Errorf("code already assigned")
	}

	if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(rawCode)) != nil {
		return fmt.Errorf("invalid code")
	}

	tag, err := r.db.GetPool().Exec(ctx,
		"UPDATE accept_codes SET user_id = $1 WHERE code_id = $2 AND user_id IS NULL",
		userID, codeID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("code already assigned")
	}

	return nil
}
