package main

import (
	"PJS_Exchange/exchanges"
	"PJS_Exchange/exchanges/channels"
	router "PJS_Exchange/routes"
	"PJS_Exchange/utils"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// @title			Project. Stock Exchange API
// @version		1.0
// @description	이것은 Project. Stock Exchange API 문서입니다.
// @license.name	MIT
// @license.url	https://github.com/0ghost0-dev/PJSe/blob/master/license
// @host			localhost:4000
// @BasePath		/
func main() {
	utils.InitEnv()

	// 거래 처리 시스템 초기화
	exo := channels.NewProcessOrders()
	go exo.Create()
	defer exo.Destroy()
	channels.OP = exo

	// Redis 초기화
	//redisClient := databases.NewRedisClient()
	//defer func(redisClient *databases.RedisClient) {
	//	err := redisClient.Close()
	//	if err != nil {
	//		println("Failed to close Redis client: " + err.Error())
	//	}
	//}(redisClient)

	ex, err := exchanges.Load()
	if err != nil {
		panic("Failed to load exchange info: " + err.Error())
	}
	_ = exchanges.UpdateMarketStatus()
	println("Loaded exchange: " + ex.Name + " in " + ex.Country + " | Session: " + exchanges.MarketStatus)

	go channels.RunWorkerPool()

	// Fiber 앱 생성
	sv := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
				"code":  code,
			})
		},
	})

	// 프로파일러 미들웨어 (배포시 제거 하기)
	//sv.Use(pprof.New())

	sv.Use(recover.New())
	sv.Use(logger.New())
	sv.Use(cors.New())
	sv.Use(compress.New())

	// 테스트용 DB 초기화
	//testDB(st)

	// 라우터 설정
	router.SetupAPIRoutes(sv)
	router.SetupWebSocketRoutes(sv)

	log.Fatal(sv.Listen(":" + utils.GetEnv("PORT", "4000")))
}
