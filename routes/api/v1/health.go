package v1

import (
	"PJS_Exchange/app/postgresApp"
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
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"Web Working": true,
		"DB1 Working": postgresApp.Get().DB.Ping(c.Context()) == nil,
		"time":        time.Now().Format(time.RFC3339),
	})
}
