package routes

import (
	v1 "PJS_Exchange/routes/api/v1"
	v1admin "PJS_Exchange/routes/api/v1/admin"
	v1market "PJS_Exchange/routes/api/v1/market"
	"PJS_Exchange/routes/ws"
	"PJS_Exchange/utils"

	"github.com/gofiber/contrib/swagger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/websocket/v2"
)

// RouteRegistrar 인터페이스 - 모든 라우터가 구현해야 함
type RouteRegistrar interface {
	RegisterRoutes(router fiber.Router)
}

// SetupAPIRoutes 모든 API 라우트를 자동으로 설정
func SetupAPIRoutes(app *fiber.App) {
	api := app.Group("/api")
	apiV1 := api.Group("/v1")

	// V1 Admin 그룹 전역설정
	apiV1Admin := apiV1.Group("/admin")

	// Swagger 설정
	apiV1AdminDocs := apiV1Admin.Group("/docs")
	apiV1AdminDocs.Use(basicauth.New(basicauth.Config{
		Users: map[string]string{
			utils.GetEnv("SWAGGER_USER", "admin"): utils.GetEnv("SWAGGER_PASSWORD", "admin"),
		},
	}), swagger.New(swagger.Config{
		BasePath: "/api/v1/admin",
		FilePath: "./docs/v1/swagger.yaml",
		Path:     "docs",
	}))

	// Market 그룹 전역설정
	apiV1Market := apiV1.Group("/market")

	// 각 버전 및 도메인의 라우터 등록
	registerV1Routes(apiV1)
	registerV1AdminRoutes(apiV1Admin)
	registerV1MarketRoutes(apiV1Market)
}

// SetupWebSocketRoutes 모든 WebSocket 라우트를 자동으로 설정
func SetupWebSocketRoutes(app *fiber.App) {
	_ws := app.Group("/ws")

	// WebSocket 도메인의 라우터 등록
	_ws.Use(func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket 라우터 등록
	registerWebSocketRoutes(_ws)
}

// registerV1Routes v1 API 라우터들을 자동으로 등록
func registerV1Routes(router fiber.Router) {
	// 각 도메인의 라우터 인스턴스 생성 및 등록
	routers := []RouteRegistrar{
		&v1.AuthRouter{},
		&v1.HealthRouter{},
		// 새로운 라우터가 추가되면 여기에 추가
	}

	for _, r := range routers {
		r.RegisterRoutes(router)
	}
}

// registerV1AdminRoutes v1 Admin API 라우터들을 자동으로 등록
func registerV1AdminRoutes(router fiber.Router) {
	// 각 도메인의 라우터 인스턴스 생성 및 등록
	routers := []RouteRegistrar{
		&v1admin.UserRouter{},
		&v1admin.SymbolRouter{},
		&v1admin.ActivationRouter{},
		// 새로운 라우터가 추가되면 여기에 추가
	}

	for _, r := range routers {
		r.RegisterRoutes(router)
	}
}

// registerV1MarketRoutes v1 Market API 라우터들을 자동으로 등록
func registerV1MarketRoutes(router fiber.Router) {
	// 각 도메인의 라우터 인스턴스 생성 및 등록
	routers := []RouteRegistrar{
		&v1market.StatusRouter{},
		&v1market.CandlesRouter{},
		&v1market.OrdersRouter{},
		&v1market.SymbolsRouter{},
		// 새로운 라우터가 추가되면 여기에 추가
	}

	for _, r := range routers {
		r.RegisterRoutes(router)
	}
}

// registerWebSocketRoutes WebSocket 라우터들을 자동으로 등록
func registerWebSocketRoutes(router fiber.Router) {
	// 각 도메인의 라우터 인스턴스 생성 및 등록
	routers := []RouteRegistrar{
		//&ws.TradeRouter{},
		&ws.DepthRouter{},
		&ws.LedgerRouter{},
		&ws.NotifyRouter{},
		&ws.SessionRouter{},
		// 새로운 라우터가 추가되면 여기에 추가
	}

	for _, r := range routers {
		r.RegisterRoutes(router)
	}
}
