package session

import (
	"PJS_Exchange/exchanges"

	"github.com/gofiber/fiber/v2"
)

func IsOnline() fiber.Handler {
	return func(c *fiber.Ctx) error {
		//user := c.Locals("user").(*postgresql.User)
		//userTradableSession := user.TradableSessions
		if exchanges.MarketStatus == "closed" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Market is closed",
				"code":  fiber.StatusServiceUnavailable,
			})
		}

		//if !userTradableSession[exchanges.MarketStatus] {
		//	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		//		"error": "Trading not allowed in the current market session",
		//		"code":  fiber.StatusForbidden,
		//	})
		//}
		return c.Next()
	}
}
