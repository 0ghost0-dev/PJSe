package middleware

import (
	"PJS_Exchange/app"
	"PJS_Exchange/utils"
	"context"
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// authorization basic 헤더에서 아이디, 비밀번호 추출
		// 아이디, 비밀번호로 유저(브로커) 인증
		// 인증 성공 시, 새로운 API 키 생성 및 DB에 저장
		// 인증 실패 시, 401 Unauthorized 반환
		auth := c.Get("Authorization")

		if auth == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing Authorization header",
				"code":  fiber.StatusUnauthorized,
			})
		}

		if !strings.HasPrefix(auth, "Basic ") {
			return c.Status(401).JSON(fiber.Map{
				"error": "Invalid authorization format",
			})
		}
		encoded := auth[6:] // "Basic "

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
		ctx := context.Background()
		user, err := app.GetApp().UserRepo().GetUserByEmail(ctx, username)
		if err == nil && user != nil {
			// 활성화된 계정인지 확인
			if user.Enabled {
				// 비밀번호 확인
				if utils.CheckPasswordHash(password, user.Password) {
					// 인증 성공, 다음 미들웨어 또는 핸들러로 진행
					c.Locals("user", user)

					return c.Next()
				} else {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Invalid credentials",
						"code":  fiber.StatusUnauthorized,
					})
				}
			} else {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "User account is not enabled",
					"code":  fiber.StatusUnauthorized,
				})
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
			"code":  fiber.StatusUnauthorized,
		})
	}
}
