package v1

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgres"
	"PJS_Exchange/middleware"
	"PJS_Exchange/template"
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type HealthRouter struct{}

func (hr *HealthRouter) RegisterRoutes(router fiber.Router) {
	healthGroup := router.Group("/health")

	healthGroup.Use(limiter.New(limiter.Config{
		Max:        5, // 최대 요청 수
		Expiration: 15 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
				"code":  fiber.StatusTooManyRequests,
			})
		},
	}))

	healthGroup.Get("/", hr.healthGet)
	healthGroup.Post("/", middleware.AuthAPIKeyMiddleware(), hr.healthPost)
}

// === 핸들러 함수들 ===

// @Summary		서버 상태 확인
// @Description	서버 상태 확인 및 데이터베이스 연결 상태 점검
// @Tags			Health
// @Accept			json
// @Produce		json
//
//	@Success		200	{object}	map[string]interface{} "성공 시 서버 상태 반환"
//
// @Router			/api/v1/health [get]
func (hr *HealthRouter) healthGet(c *fiber.Ctx) error {
	ctx := context.Background()
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Web Working": true,
		"DB1 Working": app.GetApp().DB.Ping(ctx) == nil,
		"time":        time.Now().Format(time.RFC3339),
	})
}

// @Summary		API 키 상태 확인 및 서버 상태 확인
// @Description	API 키 상태 확인 및 서버 상태 확인
// @Tags			Health
// @Accept			json
// @Produce		json
// @Param			Authorization	header		string	true	"Bearer {API_KEY}"
//
//	@Success		200				{object}	map[string]interface{} "성공 시 서버 상태 및 API 키 정보 반환"
//
//	@Failure		401				{object}	map[string]interface{} "인증 실패 시 에러 메시지 반환"
//
// @Router			/api/v1/health [post]
func (hr *HealthRouter) healthPost(c *fiber.Ctx) error {
	apiKey := c.Locals("apiKey").(*postgres.APIKey)
	ctx := context.Background()

	if apiKey != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"serverStatus": fiber.Map{
				"Web Working": true,
				"DB1 Working": app.GetApp().DB.Ping(ctx) == nil,
			},
			"status":    apiKey.Status,
			"scopes":    postgres.FilterTrueScopes(postgres.APIScopeToMap(apiKey.Scopes)),
			"createdAt": apiKey.CreatedAt,
			"expiresAt": apiKey.ExpiresAt,
			"time":      time.Now().Format(time.RFC3339),
		})
	} else {
		return template.ErrorHandler(c, fiber.StatusUnauthorized, "Invalid or Expired API Key")
	}
}
