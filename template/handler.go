package template

import "github.com/gofiber/fiber/v2"

func ErrorHandler(c *fiber.Ctx, code int, err string) error {
	return c.Status(code).JSON(fiber.Map{
		"error": err,
		"code":  code,
	})
}
