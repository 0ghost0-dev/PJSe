package middleware

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgres"
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func AuthAPIKeyMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Bearer 토큰 인증
		auth := c.Get("Authorization")
		if auth == "" || len(auth) < 7 || auth[:7] != "Bearer " {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing Authorization header",
				"code":  fiber.StatusUnauthorized,
			})
		}
		token := auth[7:] // "Bearer "

		ctx := context.Background()
		apiKey, _ := app.GetApp().APIKeyRepo().AuthenticateAPIKey(ctx, token) // 토큰 인증
		if apiKey == nil || apiKey.Status != "active" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 인증 성공, 다음 미들웨어 또는 핸들러로 진행
		c.Locals("apiKey", apiKey)

		return c.Next()
	}
}

func AuthAPIKeyMiddlewareRequireScopes(requireScopes postgres.APIKeyScope) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Bearer 토큰 인증
		auth := c.Get("Authorization")
		if auth == "" || len(auth) < 7 || auth[:7] != "Bearer " {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing Authorization header",
				"code":  fiber.StatusUnauthorized,
			})
		}
		token := auth[7:] // "Bearer "

		ctx := context.Background()
		apiKey, _ := app.GetApp().APIKeyRepo().AuthenticateAPIKey(ctx, token) // 토큰 인증
		if apiKey == nil || !postgres.IsinScope(apiKey.Scopes, requireScopes) || apiKey.Status != "active" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key or insufficient scope",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 활성화된 계정 인지 확인
		id, _ := strconv.Atoi(apiKey.UserID)
		user, _ := app.GetApp().UserRepo().GetUserByID(ctx, id)
		if user == nil || user.Enabled != true {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User account is not enabled",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 인증 성공, 다음 미들웨어 또는 핸들러로 진행
		c.Locals("apiKey", apiKey)
		c.Locals("user", user)

		return c.Next()
	}
}
