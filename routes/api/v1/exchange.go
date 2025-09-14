package v1

import (
	"PJS_Exchange/exchanges"
	"PJS_Exchange/template"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type ExchangeRouter struct{}

func (hr *ExchangeRouter) RegisterRoutes(router fiber.Router) {
	healthGroup := router.Group("/exchange")

	healthGroup.Use(limiter.New(limiter.Config{
		Max:        5, // 최대 요청 수
		Expiration: 60 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
				"code":  fiber.StatusTooManyRequests,
			})
		},
	}))

	healthGroup.Get("/", hr.getExchangeData)
}

// === 핸들러 함수들 ===

func (hr *ExchangeRouter) getExchangeData(c *fiber.Ctx) error {
	data, err := exchanges.Load()
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to load exchange data: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(data)
}
