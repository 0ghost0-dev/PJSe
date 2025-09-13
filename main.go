package main

import (
	"PJS_Exchange/databases"
	"PJS_Exchange/databases/postgres"
	"PJS_Exchange/routes/api"
	"PJS_Exchange/utils"
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	utils.InitEnv()

	// Fiber 앱 생성
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

	ctx := context.Background()

	// DB 설정
	postgresDB, errDB := databases.NewPostgresDBPool(databases.NewPostgresDBConfig())
	if errDB != nil {
		panic("Failed to connect to Postgres DB: " + errDB.Error())
	}
	defer postgresDB.Close()

	// DB Repo 설정
	UserRepo := postgres.NewUserRepository(postgresDB)
	errUT := UserRepo.CreateUsersTable(ctx)
	if errUT != nil {
		println("Failed to create users table: " + errUT.Error())
	}

	SymbolRepo := postgres.NewSymbolRepository(postgresDB)
	errST := SymbolRepo.CreateSymbolsTable(ctx)
	if errST != nil {
		println("Failed to create symbols table: " + errST.Error())
	}

	/* 테스트 유저 생성, 활성화, API 키 발급 */
	pw, _ := utils.HashPassword("password123")
	_, errUC := UserRepo.CreateUser(ctx, "테스트 증권사", "example.com", pw)
	if errUC != nil {
		println("Failed to create test user: " + errUC.Error())
	}
	errUE := UserRepo.EnableUser(ctx, 1)
	if errUE != nil {
		println("Failed to enable test user: " + errUE.Error())
	}
	key, errUA := UserRepo.NewAPIKey(ctx, 1)
	if errUA != nil {
		println("Failed to create API key for test user: " + errUA.Error())
	} else {
		println("Test user API key:", key)
	}
	/* 테스트 유저 생성, 활성화, API 키 발급 */

	/* 테스트 심볼 상장 */
	sym := &postgres.Symbol{
		Symbol:               "sml",
		Name:                 "샘플주식회사",
		Detail:               "샘플 주식 회사입니다.",
		Url:                  "https://example.com/test-stock",
		Logo:                 "https://example.com/test-stock/logo.png",
		Market:               "PJSe",
		Type:                 "stock",
		MinimumOrderQuantity: 1,
		TickSize:             1,
		Status: postgres.Status{
			Status: postgres.StatusInactive,
			Reason: "",
		},
	}

	_, errLS := SymbolRepo.ListingSymbol(ctx, sym)
	if errLS != nil {
		println("Failed to list test symbol: " + errLS.Error())
	} else {
		println("Test symbol listed:", sym.Symbol)
	}
	/* 테스트 심볼 상장 */

	// 라우터 설정
	apiRouterRegistry := api.NewRouterRegistry()

	apiRouterRegistry.SetupAll(app)

	err := app.Listen(":4000")
	if err != nil {
		return
	}
}
