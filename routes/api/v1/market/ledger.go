package v1

import "github.com/gofiber/fiber/v2"

type LedgerRouter struct{}

func (lr *LedgerRouter) RegisterRoutes(router fiber.Router) {
	ledgerGroup := router.Group("/ledger")

	ledgerGroup.Get("/:sym", lr.getLedger)
}

func (lr *LedgerRouter) getLedger(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	return fiber.NewError(fiber.StatusNotImplemented, "Not implemented: GET /ledger/"+symbol)
}
