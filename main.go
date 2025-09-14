package main

import (
	app2 "PJS_Exchange/app"
	"PJS_Exchange/exchanges"
	router "PJS_Exchange/routes/api"
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

	// 싱글톤 앱 초기화
	st := app2.GetApp()
	defer st.Close()

	ex, err := exchanges.Load()
	if err != nil {
		panic("Failed to load exchange info: " + err.Error())
	}
	println("Loaded exchange: " + ex.Name + " in " + ex.Country)

	// Fiber 앱 생성
	app := fiber.New(fiber.Config{
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

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())
	app.Use(compress.New())

	//ctx := context.Background()

	/* 테스트 유저 생성, 활성화, API 키 발급 */
	//timeA := time.Now().Add(60 * time.Minute)
	//
	//acceptCode, _, _ := st.AcceptCodeRepo().GenerateAcceptCode(ctx, &timeA)
	////acceptCode := "TESTCODE1234"
	//pw, _ := utils.HashPassword("password123")
	//_, errUC := st.UserRepo().CreateUser(ctx, "테스트 증권사", "test@example.com", pw, acceptCode)
	//if errUC != nil {
	//	println("Failed to create test user: " + errUC.Error())
	//} else {
	//	errUE := st.UserRepo().EnableUser(ctx, 1)
	//	if errUE != nil {
	//		println("Failed to enable test user: " + errUE.Error())
	//	}
	//	apiKey, _ := st.APIKeyRepo().GetUserAPIKeys(ctx, "1")
	//	if apiKey == nil {
	//		apiKey, _, err := st.APIKeyRepo().CreateAPIKey(ctx, "1", "Test API Key", postgres.APIKeyScope{APIKeyRead: true, APIKeyWrite: true, AdminSystemRead: true, AdminSystemWrite: true, AdminSymbolManage: true, AdminUserManage: true, AdminAPIKeyManage: true}, &timeA)
	//		if err != nil {
	//			println("Failed to create API key for test user: ", err.Error())
	//		} else {
	//			println("Test user API key created:", apiKey)
	//		}
	//	}
	//}
	/* 테스트 유저 생성, 활성화, API 키 발급 */

	/* 테스트 심볼 상장 */
	//sym := &postgres.Symbol{
	//	Symbol:               "sml",
	//	Name:                 "샘플주식회사",
	//	Detail:               "샘플 주식 회사입니다.",
	//	Url:                  "https://example.com/test-stock",
	//	Logo:                 "https://example.com/test-stock/logo.png",
	//	Market:               "PJSe",
	//	Type:                 "stock",
	//	MinimumOrderQuantity: 1,
	//	TickSize:             1,
	//	Status: postgres.Status{
	//		Status: postgres.StatusInactive,
	//		Reason: "",
	//	},
	//}
	//
	//_, errLS := st.SymbolRepo().SymbolListing(ctx, sym)
	//if errLS != nil {
	//	println("Failed to list test symbol: " + errLS.Error())
	//} else {
	//	println("Test symbol listed:", sym.Symbol)
	//}
	/* 테스트 심볼 상장 */

	// 라우터 설정
	router.SetupRoutes(app)

	log.Fatal(app.Listen(":4000"))
}
