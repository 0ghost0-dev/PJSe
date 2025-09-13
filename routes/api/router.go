package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// Router 인터페이스
type Router interface {
	SetupRoutes(router fiber.Router)
	GetPrefix() string
	GetMiddlewares() []fiber.Handler
}

// RouterRegistry 구조체
type RouterRegistry struct {
	routers []Router
}

// NewRouterRegistry 생성자
func NewRouterRegistry() *RouterRegistry {
	return &RouterRegistry{
		routers: make([]Router, 0),
	}
}

// Register 라우터 등록
func (r *RouterRegistry) Register(router Router) {
	r.routers = append(r.routers, router)
}

// SetupAll 모든 라우터 설정
func (r *RouterRegistry) SetupAll(app *fiber.App) {
	for _, router := range r.routers {
		group := app.Group(router.GetPrefix(), router.GetMiddlewares()...)
		router.SetupRoutes(group)

		fmt.Printf("✓ Router registered: %s\n", router.GetPrefix())
	}
}
