package symbol

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"

	"github.com/gofiber/fiber/v2"
)

func IsValid() fiber.Handler {
	return func(c *fiber.Ctx) error {
		symbol := c.Params("sym")
		if symbol == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Symbol parameter is required",
				"code":  fiber.StatusBadRequest,
			})
		}

		symbolData, err := postgresApp.Get().SymbolRepo().GetSymbolData(c.Context(), symbol)
		if err != nil || symbolData.Symbol == "" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is not listed.",
				"code":  fiber.StatusNotFound,
			})
		}

		c.Locals("symbolData", symbolData)
		return c.Next()
	}
}

func IsViewable() fiber.Handler {
	return func(c *fiber.Ctx) error {
		symbol := c.Params("sym")
		if symbol == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Symbol parameter is required",
				"code":  fiber.StatusBadRequest,
			})
		}

		symbolData, _ := postgresApp.Get().SymbolRepo().GetSymbolData(c.Context(), symbol)
		switch symbolData.Status.Status {
		case postgresql.StatusActive:
		case postgresql.StatusSuspended:
		case postgresql.StatusDelisted:
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is not listed.",
				"code":  fiber.StatusNotFound,
			})
		case postgresql.StatusInactive:
			// 심볼 정보 조회는 가능하지만, 호가 및 체결 조회는 불가능
		case postgresql.StatusInit:
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is not listed.",
				"code":  fiber.StatusNotFound,
			})
		}

		c.Locals("symbolData", symbolData)
		return c.Next()
	}
}

func IsTradable() fiber.Handler {
	return func(c *fiber.Ctx) error {
		symbol := c.Params("sym")
		if symbol == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Symbol parameter is required",
				"code":  fiber.StatusBadRequest,
			})
		}

		symbolData, err := postgresApp.Get().SymbolRepo().GetSymbolData(c.Context(), symbol)
		if err != nil || symbolData.Symbol == "" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is not listed.",
				"code":  fiber.StatusNotFound,
			})
		}

		switch symbolData.Status.Status {
		case postgresql.StatusActive:
		case postgresql.StatusSuspended:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":  "Symbol '" + symbol + "' is suspended.",
				"reason": symbolData.Status.Reason,
				"code":   fiber.StatusForbidden,
			})
		case postgresql.StatusDelisted:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":  "Symbol '" + symbol + "' is delisted.",
				"reason": symbolData.Status.Reason,
				"code":   fiber.StatusForbidden,
			})
		case postgresql.StatusInactive:
			// 심볼 정보 조회는 가능하지만, 호가 및 체결 조회는 불가능
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is inactive.",
				"code":  fiber.StatusForbidden,
			})
		case postgresql.StatusInit:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is not listed.",
				"code":  fiber.StatusForbidden,
			})
		}

		// tag에 cooldown이 있는지 확인
		if symbolData.Tags["cooldown"] {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Symbol '" + symbol + "' is in cooldown period.",
				"code":  fiber.StatusForbidden,
			})
		}

		//c.Locals("executable", symbol_data.Status.Status == postgresql.StatusActive)
		c.Locals("symbolData", symbolData)
		return c.Next()
	}
}
