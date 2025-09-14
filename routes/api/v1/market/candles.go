package v1

import (
	"PJS_Exchange/template"

	"github.com/gofiber/fiber/v2"
)

type CandlesRouter struct{}

func (cr *CandlesRouter) RegisterRoutes(router fiber.Router) {
	candlesGroup := router.Group("/candles")

	candlesGroup.Get("/:sym", cr.getCandles)
}

func (cr *CandlesRouter) getCandles(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	return template.ErrorHandler(c, fiber.StatusNotImplemented, "Not implemented: GET /candles/"+symbol)
}
