package main

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/exchanges"
	"PJS_Exchange/exchanges/channels"
	router "PJS_Exchange/routes"
	"PJS_Exchange/sys"
	"PJS_Exchange/utils"
	"context"
	"errors"
	"time"

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

	// 싱글톤 앱 초기화
	st := postgresApp.Get()
	defer st.Close()

	// 거래 처리 시스템 초기화
	exo := channels.NewProcessOrders()
	go exo.Create()
	defer exo.Destroy()
	channels.OP = exo

	// Redis 초기화
	redisClient := databases.NewRedisClient()
	defer func(redisClient *databases.RedisClient) {
		err := redisClient.Close()
		if err != nil {
			println("Failed to close Redis client: " + err.Error())
		}
	}(redisClient)

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

	sv.Use(recover.New())
	sv.Use(logger.New())
	sv.Use(cors.New())
	sv.Use(compress.New())

	// 테스트용 DB 초기화
	//testDB(st)

	// 서버를 처름 시작하는 경우 관리자 계정 생성용 accept code 생성
	sysInfo, err := sys.Get()
	if err != nil {
		println("Failed to get system info: " + err.Error())
	} else {
		if sysInfo.InitialAcceptCode == false {
			ctx := context.Background()
			timeA := time.Now().Add(60 * time.Minute)
			acceptCode, _, err := st.AcceptCodeRepo().GenerateAcceptCodeForAdmin(ctx, &timeA)
			if err != nil {
				println("Failed to create initial accept code: " + err.Error())
			} else {
				sysInfo.InitialAcceptCode = true
				errUS := sys.Edit(sysInfo)
				if errUS != nil {
					println("Failed to update system info: " + errUS.Error())
				} else {
					println("Initial accept code created: " + acceptCode + " (valid for 60 minutes)")
					println("Check the 'guides/admin.md' for how to use it to create the admin account.")
				}
			}
		}
	}

	// 라우터 설정
	router.SetupAPIRoutes(sv)
	router.SetupWebSocketRoutes(sv)

	log.Fatal(sv.Listen(":" + utils.GetEnv("PORT", "4000")))
}

func testDB(st *postgresApp.App) {
	ctx := context.Background()

	/* 테스트 유저 생성, 활성화, API 키 발급 */
	timeA := time.Now().Add(60 * time.Minute)

	acceptCode, _, _ := st.AcceptCodeRepo().GenerateAcceptCode(ctx, &timeA)
	//acceptCode := "TESTCODE1234"
	pw, _ := utils.HashPassword("password123")
	_, errUC := st.UserRepo().CreateUser(ctx, "테스트 증권사", "test@example.com", pw, acceptCode)
	if errUC != nil {
		println("Failed to create test user: " + errUC.Error())
	} else {
		errUE := st.UserRepo().EnableUser(ctx, 1)
		if errUE != nil {
			println("Failed to enable test user: " + errUE.Error())
		}
		apiKey, _ := st.APIKeyRepo().GetUserAPIKeys(ctx, "1")
		if apiKey == nil {
			apiKey, _, err := st.APIKeyRepo().CreateAPIKey(ctx, "1", "Test API Key", postgresql.APIKeyScope{APIKeyRead: true, APIKeyWrite: true, AdminSystemRead: true, AdminSystemWrite: true, AdminSymbolManage: true, AdminUserManage: true, AdminAPIKeyManage: true}, &timeA)
			if err != nil {
				println("Failed to create API key for test user: ", err.Error())
			} else {
				println("Test user API key created:", apiKey)
			}
		}
	}
	/* 테스트 유저 생성, 활성화, API 키 발급 */

	/* 테스트 심볼 상장 */
	sym := &postgresql.Symbol{
		Symbol:               "sml",
		Name:                 "샘플주식회사",
		Detail:               "샘플 주식 회사입니다.",
		Url:                  "https://example.com/test-stock",
		Logo:                 "https://example.com/test-stock/logo.png",
		Market:               "PJSe",
		Type:                 "stock",
		MinimumOrderQuantity: 1,
		TickSize:             1,
		Status: postgresql.Status{
			Status: postgresql.StatusInactive,
			Reason: "",
		},
	}

	_, errLS := st.SymbolRepo().SymbolListing(ctx, sym)
	if errLS != nil {
		println("Failed to list test symbol: " + errLS.Error())
	} else {
		println("Test symbol listed:", sym.Symbol)
	}
	/* 테스트 심볼 상장 */
}
