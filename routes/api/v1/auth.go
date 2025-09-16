package v1

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middlewares/auth"
	"PJS_Exchange/template"
	"PJS_Exchange/utils"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type AuthRouter struct{}

func (ar *AuthRouter) RegisterRoutes(router fiber.Router) {
	authGroup := router.Group("/auth")

	authGroup.Use(limiter.New(limiter.Config{
		Max:        5, // 최대 요청 수
		Expiration: 15 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return template.ErrorHandler(c,
				fiber.StatusTooManyRequests,
				"Too many requests. Please try again later.")
		},
	}))

	authGroup.Get("/", auth.LoginMiddleware(auth.Config{Bypass: false}), ar.authTest)
	authGroup.Post("/", ar.registerUser)
	authGroup.Get("/api", auth.APIKeyMiddleware(auth.Config{Bypass: false}), ar.getAPIKeyDetails)
	authGroup.Post("/token", auth.LoginMiddleware(auth.Config{Bypass: false}), ar.generateTempAPIKey)
}

// === 핸들러 함수들 ===

// @Summary		인증 테스트
// @Description	유효한 계정으로 인증된 경우 "Authenticated" 메시지와 유저 정보를 반환합니다.
// @Tags			Auth
// @Produce		json
// @Param			Authorization	header		string				true	"Basic {BASE64_ENCODED_CREDENTIALS}"
// @Success		200				{object}	map[string]interface{}	"성공 시 인증 메시지 및 유저 정보 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		403				{object}	map[string]string	"권한 없음 시 에러 메시지 반환"
// @Router			/api/v1/auth/ [get]
func (ar *AuthRouter) authTest(c *fiber.Ctx) error {
	user := c.Locals("user").(*postgresql.User)
	return c.Status(200).JSON(fiber.Map{
		"message": "Authenticated",
		"user": fiber.Map{
			"name":    user.Username,
			"email":   user.Email,
			"enabled": user.Enabled,
			"admin":   user.Admin,
		},
	})
}

// @Summary		유저 등록
// @Description	새로운 유저를 등록합니다. (Access Code 필요)
// @Tags			Auth
// @Accept		json
// @Produce		json
// @Param			user	body		object		true	"유저 등록 정보"
// @Success		201		{object}	map[string]interface{}	"성공 시 유저 정보 반환"
// @Failure		400		{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500     {object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/auth/ [post]
func (ar *AuthRouter) registerUser(c *fiber.Ctx) error {
	var req struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		AcceptCode string `json:"acceptCode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Username == "" || req.Email == "" || req.Password == "" || req.AcceptCode == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "All fields are required")
	}

	pw, err := utils.HashPassword(req.Password)
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to hash password")
	}
	user, err := postgresApp.Get().UserRepo().CreateUser(c.Context(), req.Username, req.Email, pw, req.AcceptCode)
	if err != nil {
		// 만약 err 내용에 "failed to validate accept code"가 포함되어 있으면 400 반환
		if strings.Contains(err.Error(), "failed to validate accept code") || strings.Contains(err.Error(), "invalid or used accept code") {
			//log.Println("Invalid or used access code:", err)
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid or used access code")
		} else {
			log.Println("Error creating user:", err)
			return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to create user")
		}
	}
	log.Println("User created:", user.Username)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"username": user.Username,
		"email":    user.Email,
		"enabled":  user.Enabled,
	})
}

// @Summary		API 키 상태 확인
// @Description	API 키 상태 확인
// @Tags			Auth
// @Accept			json
// @Produce		json
// @Param			Authorization	header		string	true	"Bearer {API_KEY}"
// @Success		200				{object}	map[string]interface{} "성공 시 API 키 정보 반환"
// @Failure		401				{object}	map[string]interface{} "인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/auth/api [get]
func (ar *AuthRouter) getAPIKeyDetails(c *fiber.Ctx) error {
	apiKey := c.Locals("apiKey").(*postgresql.APIKey)

	if apiKey != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":    apiKey.Status,
			"scopes":    postgresql.FilterTrueScopes(postgresql.APIScopeToMap(apiKey.Scopes)),
			"createdAt": apiKey.CreatedAt,
			"expiresAt": apiKey.ExpiresAt,
			"time":      time.Now().Format(time.RFC3339),
		})
	} else {
		return template.ErrorHandler(c, fiber.StatusUnauthorized, "Invalid or Expired API Key")
	}
}

// @Summary		임시 API 키 생성
// @Description	인증된 유저를 위해 새로운 API 키를 생성합니다. (임시 키, 24시간 유효)
// @Tags			Auth
// @Produce		json
// @Param			Authorization	header		string				true	"Basic {base64_encoded_credentials}"
// @Success		200				{object}	map[string]interface{}	"성공 시 새로운 API 키 및 정보 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		403				{object}	map[string]string	"권한 없음 시 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/auth/token [post]
func (ar *AuthRouter) generateTempAPIKey(c *fiber.Ctx) error {
	user := c.Locals("user").(*postgresql.User)

	// 새로운 API 키 생성
	timeA := time.Now().Add(4 * time.Hour)

	var newAPIKey string
	var apiKeyData *postgresql.APIKey
	var err error
	if user.Admin {
		newAPIKey, apiKeyData, err = postgresApp.Get().APIKeyRepo().CreateAPIKey(c.Context(), strconv.Itoa(user.ID), "Temporary Auth API Key (Admin)", postgresql.FullAdminScopes(), &timeA)
	} else {
		newAPIKey, apiKeyData, err = postgresApp.Get().APIKeyRepo().CreateAPIKey(c.Context(), strconv.Itoa(user.ID), "Temporary Auth API Key", postgresql.APIKeyReadWriteScopes(), &timeA)
	}
	if err != nil {
		log.Println("Error creating API key:", err)
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to create API key")
	}
	return c.Status(200).JSON(fiber.Map{
		"apiKey": newAPIKey,
		"scope":  postgresql.FilterTrueScopes(postgresql.APIScopeToMap(apiKeyData.Scopes)),
		"expiry": timeA.Format(time.RFC3339),
	})
}
