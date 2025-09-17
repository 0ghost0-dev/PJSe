package auth

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/utils"
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	Bypass bool // 활성화되지 않은 계정도 통과시키는 옵션
}

func LoginMiddleware(config Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")

		if auth == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing Authorization header",
				"code":  fiber.StatusUnauthorized,
			})
		}

		const basicPrefix = "Basic "
		if len(auth) < len(basicPrefix) || auth[:len(basicPrefix)] != basicPrefix {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization format",
				"code":  fiber.StatusUnauthorized,
			})
		}
		encoded := auth[len(basicPrefix):]

		decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid base64 encoding",
				"code":  fiber.StatusUnauthorized,
			})
		}

		credentials := string(decodedBytes)
		parts := strings.SplitN(credentials, ":", 2)
		if len(parts) != 2 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid credentials format",
				"code":  fiber.StatusUnauthorized,
			})
		}

		username := parts[0]
		password := parts[1]

		// 인증 로직
		ctx := c.Context()
		user, err := postgresApp.Get().UserRepo().GetUserByEmail(ctx, username)
		if err != nil || user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid credentials",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 활성화된 계정인지 확인
		if !user.Enabled && !config.Bypass {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User account is not enabled",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 비밀번호 확인
		if !utils.CheckPasswordHash(password, user.Password) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid credentials",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 인증 성공, 다음 미들웨어 또는 핸들러로 진행
		c.Locals("user", user)
		return c.Next()
	}
}

func APIKeyMiddleware(config Config) fiber.Handler {
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

		ctx := c.Context()
		apiKey, err := postgresApp.Get().APIKeyRepo().AuthenticateAPIKey(ctx, token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key authentication failed",
				"code":  fiber.StatusUnauthorized,
			})
		}

		if apiKey == nil || apiKey.Status != "active" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 활성화된 계정 인지 확인
		id, err := strconv.Atoi(apiKey.UserID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Invalid user ID format",
				"code":  fiber.StatusInternalServerError,
			})
		}

		user, err := postgresApp.Get().UserRepo().GetUserByID(ctx, id)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to retrieve user",
				"code":  fiber.StatusInternalServerError,
			})
		}

		if user == nil || (!user.Enabled && !config.Bypass) {
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

func APIKeyMiddlewareRequireScopes(config Config, requireScopes postgresql.APIKeyScope) fiber.Handler {
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

		ctx := c.Context()
		apiKey, err := postgresApp.Get().APIKeyRepo().AuthenticateAPIKey(ctx, token)
		if err != nil || apiKey == nil || !postgresql.IsinScope(apiKey.Scopes, requireScopes) || apiKey.Status != "active" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key or insufficient scope",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 활성화된 계정 인지 확인
		id, err := strconv.Atoi(apiKey.UserID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Invalid user ID format",
				"code":  fiber.StatusInternalServerError,
			})
		}

		user, err := postgresApp.Get().UserRepo().GetUserByID(ctx, id)
		if err != nil || user == nil || (!user.Enabled && !config.Bypass) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User account is not enabled",
				"code":  fiber.StatusUnauthorized,
			})
		}

		// 인증 성공, 다음 미들웨어 또는 핸들러로 진행
		c.Locals("apiKey", apiKey)
		c.Locals("user", user)

		//user := postgresql.User{ // 테스트용
		//	ID:       1,
		//	Username: "testuser",
		//}
		//c.Locals("user", &user)

		return c.Next()
	}
}
