package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	app := fiber.New()

	app.Use(recover.New())  // panic 보호
	app.Use(logger.New())   // 요청 로그
	app.Use(cors.New())     // CORS
	app.Use(compress.New()) // gzip/br
	//app.Use(limiter.New(limiter.Config{
	//	Max:        120, // 최대 요청 수
	//	Expiration: 60 * time.Second,
	//	LimitReached: func(c *fiber.Ctx) error {
	//		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
	//			"error": "Too many requests",
	//			"code":  fiber.StatusTooManyRequests,
	//		})
	//	},
	//}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, Fiber!")
	})

	err := app.Listen(":3000")
	if err != nil {
		return
	}
}
