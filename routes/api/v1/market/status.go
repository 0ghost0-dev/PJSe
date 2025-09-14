package v1

import (
	"PJS_Exchange/exchanges"
	"PJS_Exchange/template"

	"github.com/gofiber/fiber/v2"
)

type StatusRouter struct{}

func (sr *StatusRouter) RegisterRoutes(router fiber.Router) {
	statusGroup := router.Group("/status")

	statusGroup.Get("/", sr.getExchangeData)
	statusGroup.Get("/session", sr.getSession)
}

// === 핸들러 함수들 ===

// @Summary		거래소 세션 정보 조회
// @Description	거래소의 현재 세션 상태(오픈, 클로즈 등)를 반환합니다.
// @Tags			Market - Status
// @Produce		json
// @Success		200	{object}	map[string]string	"성공 시 세션 상태 반환"
// @Failure		500	{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/status [get]
func (sr *StatusRouter) getSession(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"market_session": exchanges.MarketStatus,
	})
}

// @Summary		거래소 데이터 조회
// @Description	거래소의 현재 데이터(심볼, 티커 등)를 반환합니다.
// @Tags			Market - Status
// @Produce		json
// @Success		200	{object}	map[string]interface{}	"성공 시 거래소 데이터 반환"
// @Failure		500	{object}	map[string]string		"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/status/exchange-data [get]
func (sr *StatusRouter) getExchangeData(c *fiber.Ctx) error {
	data, err := exchanges.Load()
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to load exchange data: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(data)
}
