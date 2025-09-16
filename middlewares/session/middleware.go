package session

import (
	"PJS_Exchange/exchanges"

	"github.com/gofiber/fiber/v2"
)

func IsOnline() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if exchanges.GetCurrentSession() == "closed" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Market is closed",
				"code":  fiber.StatusServiceUnavailable,
			})
		}
		return c.Next()
	}
}
