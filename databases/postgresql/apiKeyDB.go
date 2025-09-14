package postgresql

import (
	"PJS_Exchange/databases"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	APIKeyStatusActive   = "active"
	APIKeyStatusInactive = "inactive"
	APIKeyStatusRevoked  = "revoked"
	APIKeyStatusExpired  = "expired"
)

const (
	// 시장 데이터 관련
	ScopeMarketDataRead    = "market_data:read"    // 실시간 시세/호가 조회
	ScopeMarketHistoryRead = "market_history:read" // 과거 차트 데이터 조회
	ScopeMarketSymbolRead  = "market_symbol:read"  // 티커 정보 조회

	// 원시 데이터 관련
	ScopeRawDataRead = "raw_data:read" // 원시 거래 데이터 조회

	// 주문 관련
	ScopeOrderRead   = "order:read"   // 주문 조회
	ScopeOrderCreate = "order:create" // 주문 생성
	ScopeOrderCancel = "order:cancel" // 주문 취소
	ScopeOrderModify = "order:modify" // 주문 수정
	ScopeOrderNotify = "order:notify" // 주문 알림

	// 관리자 기능
	ScopeAdminAPIManage    = "admin:api_manage"    // API 키 관리
	ScopeAdminUserManage   = "admin:user_manage"   // 유저 관리
	ScopeAdminSymbolManage = "admin:symbol_manage" // 티커 관리
	ScopeAdminSystemRead   = "admin:system_read"   // 시스템 정보 조회
	ScopeAdminSystemWrite  = "admin:system_write"  // 시스템 설정 변경

	// API 키 관리
	ScopeAPIKeyRead  = "api_key:read"  // API 키 조회
	ScopeAPIKeyWrite = "api_key:write" // API 키 생성/수정
)

type APIKeyScope struct {
	// 시장 데이터
	MarketDataRead    bool `json:"market_data_read"`
	MarketHistoryRead bool `json:"market_history_read"`
	MarketSymbolRead  bool `json:"market_symbol_read"`

	// 원시 데이터
	RawDataRead bool `json:"raw_data_read"`

	// 주문
	OrderRead   bool `json:"order_read"`
	OrderCreate bool `json:"order_create"`
	OrderCancel bool `json:"order_cancel"`
	OrderModify bool `json:"order_modify"`
	OrderNotify bool `json:"order_notify"`

	// 관리자
	AdminAPIKeyManage bool `json:"admin_api_key_manage"`
	AdminUserManage   bool `json:"admin_user_manage"`
	AdminSymbolManage bool `json:"admin_symbol_manage"`
	AdminSystemRead   bool `json:"admin_system_read"`
	AdminSystemWrite  bool `json:"admin_system_write"`

	// API 키 관리
	APIKeyRead  bool `json:"api_key_read"`
	APIKeyWrite bool `json:"api_key_write"`
}

type APIKey struct {
	ID                string      `json:"id"`
	UserID            string      `json:"user_id"`
	KeyHash           string      `json:"-"` // JSON에서 제외
	KeyPrefix         string      `json:"key_prefix"`
	Name              string      `json:"name"`
	Scopes            APIKeyScope `json:"scopes"`
	AllowedIPs        []string    `json:"allowed_ips,omitempty"`
	AllowedCuntries   []string    `json:"allowed_countries,omitempty"`
	AllowedUserAgents []string    `json:"allowed_user_agents,omitempty"`
	Status            string      `json:"status"`
	CreatedAt         time.Time   `json:"created_at"`
	LastUsed          *time.Time  `json:"last_used"`
	ExpiresAt         *time.Time  `json:"expires_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type APIKeyRepository interface {
	CreateAPIKeysTable(ctx context.Context) error
	CreateAPIKey(ctx context.Context, userID, name string, scopes APIKeyScope, expiresAt *time.Time) (string, *APIKey, error)
	AuthenticateAPIKey(ctx context.Context, apiKey string) (*APIKey, error)
	GetUserAPIKeys(ctx context.Context, userID string) ([]*APIKey, error)
	GetAPIKeyByID(ctx context.Context, keyID string) (*APIKey, error)
	UpdateAPIKeyStatus(ctx context.Context, keyID, status string) error
	UpdateLastUsed(ctx context.Context, keyID string) error
	RevokeAPIKey(ctx context.Context, keyID string) error
	CleanupExpiredKeys(ctx context.Context) error
}

type APIKeyDBRepository struct {
	db         *databases.PostgresDBPool
	prefix     string
	bcryptCost int
}

func NewAPIKeyRepository(db *databases.PostgresDBPool) *APIKeyDBRepository {
	return &APIKeyDBRepository{
		db:         db,
		prefix:     "pjse_",
		bcryptCost: 12,
	}
}

func (r *APIKeyDBRepository) CreateAPIKeysTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS api_keys (
		id VARCHAR(32) PRIMARY KEY,
		user_id VARCHAR(36) NOT NULL,
		key_hash VARCHAR(255) NOT NULL,
		key_prefix VARCHAR(20) NOT NULL,
		name VARCHAR(100) NOT NULL,
		scopes JSONB DEFAULT '{}',
	    allowed_ips JSONB DEFAULT '["*"]',
	    allowed_countries JSONB DEFAULT '["*"]',
	    allowed_user_agents JSONB DEFAULT '["*"]',
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMPTZ DEFAULT NOW(),
		last_used TIMESTAMPTZ,
		expires_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);
	
	CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
	CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
	CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
	CREATE INDEX IF NOT EXISTS idx_api_keys_expires ON api_keys(expires_at);
	`
	_, err := r.db.GetPool().Exec(ctx, query)
	return err
}

func (r *APIKeyDBRepository) generateAPIKey() (string, error) {
	// 32바이트 랜덤 데이터 생성
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("crypto/rand 읽기 실패: %w", err)
	}

	// 랜덤 바이트를 hex 문자열로 변환
	randomHex := hex.EncodeToString(randomBytes)

	// SHA256 해시로 체크섬 생성
	hash := sha256.Sum256(randomBytes)
	checksum := hex.EncodeToString(hash[:])[:8]

	// 최종 API 키 조합
	apiKey := fmt.Sprintf("%s%s_%s", r.prefix, randomHex, checksum)

	return apiKey, nil
}

// 해시가 너무 오래 걸려서 SHA256으로 변경
//func (r *APIKeyDBRepository) hashAPIKey(apiKey string) (string, error) {
//	// bcrypt 72바이트 제한을 위해 먼저 SHA256으로 해싱
//	hash := sha256.Sum256([]byte(apiKey))
//	hashHex := hex.EncodeToString(hash[:])
//
//	// SHA256 해시(64글자)를 bcrypt로 해싱
//	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(hashHex), r.bcryptCost)
//	if err != nil {
//		return "", fmt.Errorf("bcrypt 해싱 실패: %w", err)
//	}
//	return string(hashedBytes), nil
//}
//
//func (r *APIKeyDBRepository) verifyAPIKey(apiKey, hashedKey string) bool {
//	// 동일한 방식으로 SHA256 먼저 적용
//	hash := sha256.Sum256([]byte(apiKey))
//	hashHex := hex.EncodeToString(hash[:])
//
//	err := bcrypt.CompareHashAndPassword([]byte(hashedKey), []byte(hashHex))
//	return err == nil
//}

func (r *APIKeyDBRepository) hashAPIKey(apiKey string) (string, error) {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:]), nil
}

func (r *APIKeyDBRepository) verifyAPIKey(apiKey, hashedKey string) bool {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:]) == hashedKey
}

func (r *APIKeyDBRepository) validateAPIKeyFormat(apiKey string) bool {
	if !strings.HasPrefix(apiKey, r.prefix) {
		return false
	}

	withoutPrefix := strings.TrimPrefix(apiKey, r.prefix)
	parts := strings.Split(withoutPrefix, "_")
	if len(parts) != 2 {
		return false
	}

	randomPart := parts[0]
	checksumPart := parts[1]

	if len(randomPart) != 64 || len(checksumPart) != 8 {
		return false
	}

	if _, err := hex.DecodeString(randomPart); err != nil {
		return false
	}

	if _, err := hex.DecodeString(checksumPart); err != nil {
		return false
	}

	return true
}

func (r *APIKeyDBRepository) verifyChecksum(apiKey string) bool {
	if !r.validateAPIKeyFormat(apiKey) {
		return false
	}

	withoutPrefix := strings.TrimPrefix(apiKey, r.prefix)
	parts := strings.Split(withoutPrefix, "_")

	randomHex := parts[0]
	providedChecksum := parts[1]

	randomBytes, err := hex.DecodeString(randomHex)
	if err != nil {
		return false
	}

	hash := sha256.Sum256(randomBytes)
	expectedChecksum := hex.EncodeToString(hash[:])[:8]

	return providedChecksum == expectedChecksum
}

func (r *APIKeyDBRepository) CreateAPIKey(ctx context.Context, userID, name string, scopes APIKeyScope, expiresAt *time.Time) (string, *APIKey, error) {
	// API 키 생성
	apiKey, err := r.generateAPIKey()
	if err != nil {
		return "", nil, fmt.Errorf("API 키 생성 실패: %w", err)
	}

	// API 키 해싱
	hashedKey, err := r.hashAPIKey(apiKey)
	if err != nil {
		return "", nil, fmt.Errorf("API 키 해싱 실패: %w", err)
	}

	// 마스킹용 prefix 추출 (처음 10글자)
	keyPrefix := apiKey
	if len(apiKey) > 10 {
		keyPrefix = apiKey[:10]
	}

	// 고유 ID 생성
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", nil, fmt.Errorf("ID 생성 실패: %w", err)
	}
	id := hex.EncodeToString(idBytes)

	// 데이터베이스에 저장
	query := `
		INSERT INTO api_keys (id, user_id, key_hash, key_prefix, name, scopes, status, created_at, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	now := time.Now().UTC()
	_, err = r.db.GetPool().Exec(ctx, query,
		id, userID, hashedKey, keyPrefix, name, scopes,
		APIKeyStatusActive, now, expiresAt, now)

	if err != nil {
		return "", nil, fmt.Errorf("데이터베이스 저장 실패: %w", err)
	}

	result := &APIKey{
		ID:        id,
		UserID:    userID,
		KeyHash:   hashedKey,
		KeyPrefix: keyPrefix,
		Name:      name,
		Scopes:    scopes,
		Status:    APIKeyStatusActive,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		UpdatedAt: now,
	}

	return apiKey, result, nil
}

func (r *APIKeyDBRepository) AuthenticateAPIKey(ctx context.Context, apiKey string) (*APIKey, error) {
	// 키 형식 검증
	if !r.validateAPIKeyFormat(apiKey) {
		return nil, fmt.Errorf("잘못된 API 키 형식")
	}

	// 체크섬 검증
	if !r.verifyChecksum(apiKey) {
		return nil, fmt.Errorf("잘못된 API 키 체크섬")
	}

	// 마스킹용 prefix 추출
	keyPrefix := apiKey
	if len(apiKey) > 10 {
		keyPrefix = apiKey[:10]
	}

	// 유효기한 지난 키 정리
	if err := r.CleanupExpiredKeys(context.Background()); err != nil {
		fmt.Printf("만료된 API 키 정리 실패: %v\n", err)
	}

	// 데이터베이스에서 해당 prefix를 가진 활성 키들 조회
	query := `
		SELECT id, user_id, key_hash, key_prefix, name, scopes, status, created_at, last_used, expires_at, updated_at
		FROM api_keys 
		WHERE key_prefix = $1 AND status = $2 AND (expires_at IS NULL OR expires_at > NOW())
	`

	rows, err := r.db.GetPool().Query(ctx, query, keyPrefix, APIKeyStatusActive)
	if err != nil {
		return nil, fmt.Errorf("데이터베이스 조회 실패: %w", err)
	}
	defer rows.Close()

	// 각 키에 대해 bcrypt 검증 수행
	for rows.Next() {
		var key APIKey
		err := rows.Scan(
			&key.ID, &key.UserID, &key.KeyHash, &key.KeyPrefix,
			&key.Name, &key.Scopes, &key.Status, &key.CreatedAt,
			&key.LastUsed, &key.ExpiresAt, &key.UpdatedAt,
		)
		if err != nil {
			continue
		}

		// bcrypt로 키 검증
		if r.verifyAPIKey(apiKey, key.KeyHash) {
			// 마지막 사용 시간 업데이트 (비동기)
			go func() {
				if err := r.UpdateLastUsed(context.Background(), key.ID); err != nil {
					fmt.Printf("마지막 사용 시간 업데이트 실패: %v\n", err)
				}
			}()
			return &key, nil
		}
	}

	return nil, fmt.Errorf("유효하지 않은 API 키")
}

func (r *APIKeyDBRepository) GetUserAPIKeys(ctx context.Context, userID string) ([]*APIKey, error) {
	query := `
		SELECT id, user_id, key_prefix, name, scopes, status, created_at, last_used, expires_at, updated_at
		FROM api_keys 
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.GetPool().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("API 키 조회 실패: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		err := rows.Scan(
			&key.ID, &key.UserID, &key.KeyPrefix, &key.Name,
			&key.Scopes, &key.Status, &key.CreatedAt,
			&key.LastUsed, &key.ExpiresAt, &key.UpdatedAt,
		)
		if err != nil {
			continue
		}
		keys = append(keys, &key)
	}

	return keys, nil
}

func (r *APIKeyDBRepository) GetAPIKeyByID(ctx context.Context, keyID string) (*APIKey, error) {
	query := `
		SELECT id, user_id, key_prefix, name, scopes, status, created_at, last_used, expires_at, updated_at
		FROM api_keys 
		WHERE id = $1
	`

	var key APIKey
	err := r.db.GetPool().QueryRow(ctx, query, keyID).Scan(
		&key.ID, &key.UserID, &key.KeyPrefix, &key.Name,
		&key.Scopes, &key.Status, &key.CreatedAt,
		&key.LastUsed, &key.ExpiresAt, &key.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("API 키 조회 실패: %w", err)
	}

	return &key, nil
}

func (r *APIKeyDBRepository) GetUserIDByAPIKey(ctx context.Context, apiKey string) (string, error) {
	key, err := r.AuthenticateAPIKey(ctx, apiKey)
	if err != nil {
		return "", err
	}
	return key.UserID, nil
}

func (r *APIKeyDBRepository) UpdateAPIKeyStatus(ctx context.Context, keyID, status string) error {
	query := `
		UPDATE api_keys 
		SET status = $1, updated_at = $2 
		WHERE id = $3
	`
	_, err := r.db.GetPool().Exec(ctx, query, status, time.Now().UTC(), keyID)
	if err != nil {
		return fmt.Errorf("API 키 상태 업데이트 실패: %w", err)
	}
	return nil
}

func (r *APIKeyDBRepository) UpdateLastUsed(ctx context.Context, keyID string) error {
	query := `UPDATE api_keys SET last_used = $1 WHERE id = $2`
	_, err := r.db.GetPool().Exec(ctx, query, time.Now().UTC(), keyID)
	return err
}

func (r *APIKeyDBRepository) RevokeAPIKey(ctx context.Context, keyID string) error {
	return r.UpdateAPIKeyStatus(ctx, keyID, APIKeyStatusRevoked)
}

func (r *APIKeyDBRepository) CleanupExpiredKeys(ctx context.Context) error {
	query := `
		UPDATE api_keys 
		SET status = $1, updated_at = $2 
		WHERE expires_at < NOW() AND status = $3
	`
	_, err := r.db.GetPool().Exec(ctx, query,
		APIKeyStatusExpired, time.Now().UTC(), APIKeyStatusActive)
	if err != nil {
		return fmt.Errorf("만료된 API 키 정리 실패: %w", err)
	}
	return nil
}

// 추가 유틸리티 메서드들

func (r *APIKeyDBRepository) MaskAPIKey(apiKey string) string {
	if len(apiKey) < 12 {
		return "****"
	}
	return apiKey[:8] + "****" + apiKey[len(apiKey)-4:]
}

func APIScopeToMap(scopes APIKeyScope) map[string]bool {
	return map[string]bool{
		ScopeMarketDataRead:    scopes.MarketDataRead,
		ScopeMarketHistoryRead: scopes.MarketHistoryRead,
		ScopeMarketSymbolRead:  scopes.MarketSymbolRead,
		ScopeRawDataRead:       scopes.RawDataRead,
		ScopeOrderRead:         scopes.OrderRead,
		ScopeOrderCreate:       scopes.OrderCreate,
		ScopeOrderCancel:       scopes.OrderCancel,
		ScopeOrderModify:       scopes.OrderModify,
		ScopeOrderNotify:       scopes.OrderNotify,
		ScopeAdminAPIManage:    scopes.AdminAPIKeyManage,
		ScopeAdminUserManage:   scopes.AdminUserManage,
		ScopeAdminSymbolManage: scopes.AdminSymbolManage,
		ScopeAdminSystemRead:   scopes.AdminSystemRead,
		ScopeAdminSystemWrite:  scopes.AdminSystemWrite,
		ScopeAPIKeyRead:        scopes.APIKeyRead,
		ScopeAPIKeyWrite:       scopes.APIKeyWrite,
	}
}

func FilterTrueScopes(scopes map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for key, value := range scopes {
		if value {
			result[key] = value
		}
	}
	return result
}

func FilterScopesExcludeAdmin(scopes map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for key, value := range scopes {
		if value && key != "admin" {
			result[key] = value
		}
	}
	return result
}

// HasScope 스코프 검증을 위한 헬퍼 메서드들
func (r *APIKeyDBRepository) HasScope(key *APIKey, scope string) bool {
	switch scope {
	case ScopeMarketDataRead:
		return key.Scopes.MarketDataRead
	case ScopeMarketHistoryRead:
		return key.Scopes.MarketHistoryRead
	case ScopeMarketSymbolRead:
		return key.Scopes.MarketSymbolRead
	case ScopeRawDataRead:
		return key.Scopes.RawDataRead
	case ScopeOrderRead:
		return key.Scopes.OrderRead
	case ScopeOrderCreate:
		return key.Scopes.OrderCreate
	case ScopeOrderCancel:
		return key.Scopes.OrderCancel
	case ScopeOrderModify:
		return key.Scopes.OrderModify
	case ScopeOrderNotify:
		return key.Scopes.OrderNotify
	case ScopeAdminAPIManage:
		return key.Scopes.AdminAPIKeyManage
	case ScopeAdminUserManage:
		return key.Scopes.AdminUserManage
	case ScopeAdminSymbolManage:
		return key.Scopes.AdminSymbolManage
	case ScopeAdminSystemRead:
		return key.Scopes.AdminSystemRead
	case ScopeAdminSystemWrite:
		return key.Scopes.AdminSystemWrite
	case ScopeAPIKeyRead:
		return key.Scopes.APIKeyRead
	case ScopeAPIKeyWrite:
		return key.Scopes.APIKeyWrite
	default:
		return false
	}
}

func IsinScope(userScopes APIKeyScope, requiredScopes APIKeyScope) bool {
	userMap := APIScopeToMap(userScopes)
	requiredMap := APIScopeToMap(requiredScopes)
	for scope, required := range requiredMap {
		if required {
			if has, ok := userMap[scope]; !ok || !has {
				return false
			}
		}
	}
	return true
}

// HasScopes 여러 스코프 검증
//func (r *APIKeyDBRepository) HasScopes(key *APIKey, scopes []string) bool {
//	for _, scope := range scopes {
//		if !r.HasScope(key, scope) {
//			return false
//		}
//	}
//	return true
//}

// HasAnyScope 스코프 그룹 검증 (OR 조건)
//func (r *APIKeyDBRepository) HasAnyScope(key *APIKey, scopes []string) bool {
//	for _, scope := range scopes {
//		if r.HasScope(key, scope) {
//			return true
//		}
//	}
//	return false
//}

// IsAPIAdmin 관리자 권한 확인
func (r *APIKeyDBRepository) IsAPIAdmin(key *APIKey) bool {
	return key.Scopes.AdminAPIKeyManage
}

// IsFullAdmin 완전한 관리자 권한 확인
func (r *APIKeyDBRepository) IsFullAdmin(key *APIKey) bool {
	return key.Scopes.AdminAPIKeyManage && key.Scopes.AdminUserManage && key.Scopes.AdminSystemWrite && key.Scopes.AdminSymbolManage && key.Scopes.AdminSystemRead
}

// HasAnyAdminScope 관리자 스코프 중 하나라도 있는지 확인
func (r *APIKeyDBRepository) HasAnyAdminScope(key *APIKey) bool {
	return key.Scopes.AdminAPIKeyManage || key.Scopes.AdminUserManage || key.Scopes.AdminSystemRead || key.Scopes.AdminSystemWrite || key.Scopes.AdminSymbolManage
}
