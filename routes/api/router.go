package router

import (
	v1 "PJS_Exchange/routes/api/v1"
	v1_admin "PJS_Exchange/routes/api/v1/admin"
	"PJS_Exchange/utils"

	"github.com/gofiber/contrib/swagger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
)

// RouteRegistrar 인터페이스 - 모든 라우터가 구현해야 함
type RouteRegistrar interface {
	RegisterRoutes(router fiber.Router)
}

// SetupRoutes 모든 라우트를 자동으로 설정
func SetupRoutes(app *fiber.App) {
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
		FilePath: "/docs/v1/swagger.yaml",
		Path:     "/docs/v1",
	}))

	// 각 버전 및 도메인의 라우터 등록
	registerV1Routes(apiV1)
	registerV1AdminRoutes(apiV1Admin)
}

// registerV1Routes v1 API 라우터들을 자동으로 등록
func registerV1Routes(router fiber.Router) {
	// 각 도메인의 라우터 인스턴스 생성 및 등록
	routers := []RouteRegistrar{
		&v1.AuthRouter{},
		&v1.HealthRouter{},
		&v1.ExchangeRouter{},
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
		&v1_admin.UserRouter{},
		&v1_admin.SymbolRouter{},
		// 새로운 라우터가 추가되면 여기에 추가
	}

	for _, r := range routers {
		r.RegisterRoutes(router)
	}
}
