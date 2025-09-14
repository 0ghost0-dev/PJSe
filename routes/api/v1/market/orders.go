package v1

import "github.com/gofiber/fiber/v2"

type OrdersRouter struct{}

func (or *OrdersRouter) RegisterRoutes(router fiber.Router) {
	ordersGroup := router.Group("/orders")

	ordersGroup.Get("/:sym", or.getOrders)
}

func (or *OrdersRouter) getOrders(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	return fiber.NewError(fiber.StatusNotImplemented, "Not implemented: GET /orders/"+symbol)
}
