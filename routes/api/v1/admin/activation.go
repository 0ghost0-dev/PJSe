package admin

import (
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middleware"
	"PJS_Exchange/singletons/postgresApp"
	"PJS_Exchange/template"
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type ActivationRouter struct{}

func (ar *ActivationRouter) RegisterRoutes(router fiber.Router) {
	activationGroup := router.Group("/activation", middleware.AuthMiddleware(middleware.AuthConfig{Bypass: true}))

	activationGroup.Use(limiter.New(limiter.Config{
		Max:        2, // 최대 요청 수
		Expiration: 120 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
				"code":  fiber.StatusTooManyRequests,
			})
		},
	}))

	activationGroup.Patch("/", ar.activation)
}

// === 핸들러 함수들 ===

// @Summary 첫 번째 관리자 계정 활성화
// @Description 첫 번째 관리자 계정을 활성화합니다. 이 작업은 primary admin만 수행할 수 있습니다.
// @Tags Admin - Activation
// @Accept json
// @Produce json
// @Param			Authorization	header		string 				true	"Basic {BASE64_ENCODED_CREDENTIALS}"
// @Success 200 {object} map[string]string "성공 시 관리자 계정 활성화 메시지 반환"
// @Failure 403 {object} map[string]string "접근 거부 또는 권한 없음"
// @Failure 500 {object} map[string]string "서버 오류 발생 시 에러 메시지 반환"
// @Router /api/v1/admin/activation [patch]
func (ar *ActivationRouter) activation(c *fiber.Ctx) error {
	user := c.Locals("user").(*postgresql.User)
	if !user.Admin {
		return template.ErrorHandler(c, fiber.StatusForbidden, "Access denied")
	}
	if user.ID != 1 {
		return template.ErrorHandler(c, fiber.StatusForbidden, "Only the primary admin can perform this action")
	}

	ctx := context.Background()
	err := postgresApp.Get().UserRepo().EnableUser(ctx, user.ID)
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to activate user: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User activated successfully",
	})
}
