package v1

import "github.com/gofiber/fiber/v2"

type DepthRouter struct{}

func (dr *DepthRouter) RegisterRoutes(router fiber.Router) {
	depthGroup := router.Group("/depth")

	depthGroup.Get("/:sym", dr.getDepth)
}

func (dr *DepthRouter) getDepth(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	return fiber.NewError(fiber.StatusNotImplemented, "Not implemented: GET /depth/"+symbol)
}
