package market

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middlewares/auth"
	s "PJS_Exchange/middlewares/symbol"
	"PJS_Exchange/routes/ws"
	"PJS_Exchange/template"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type SymbolsRouter struct{}

func (sr *SymbolsRouter) RegisterRoutes(router fiber.Router) {
	symbolsGroup := router.Group("/symbols")

	symbolsGroup.Use(limiter.New(limiter.Config{
		Max:        10, // 최대 요청 수
		Expiration: 60 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return template.ErrorHandler(c,
				fiber.StatusTooManyRequests,
				"Too many requests. Please try again later.")
		},
	}), auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
		MarketSymbolRead: true,
	}))

	symbolsGroup.Get("/", sr.symbolList)
	symbolsGroup.Get("/:sym", s.IsViewable(), sr.symbolDetail)
	symbolsGroup.Get("/:sym/now", s.IsViewable(), sr.symbolNow)
}

// === 핸들러 함수들 ===

// TODO: 추후 protobuf로 변경
// @Summary		상장된 모든 심볼 조회
// @Description	상장된 모든 심볼의 리스트를 반환합니다.
// @Tags			Market - Status
// @Produce		json
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Success		200				{object}	map[string][]postgresql.Symbol	"성공 시 심볼 목록 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/market/symbol [get]
func (sr *SymbolsRouter) symbolList(c *fiber.Ctx) error {
	symbols, err := postgresApp.Get().SymbolRepo().GetSymbolsViewable(c.Context())
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Error fetching symbols")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbols": symbols,
	})
}

// TODO: 추후 protobuf로 변경
// @Summary		특정 심볼 정보 조회
// @Description	특정 심볼의 정보를 반환합니다.
// @Tags			Market - Status
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Success		200				{object}	map[string]postgresql.Symbol	"성공 시 심볼 상세 정보 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/market/symbol/{symbol} [get]
func (sr *SymbolsRouter) symbolDetail(c *fiber.Ctx) error {
	symbol := c.Locals("symbolData").(*postgresql.Symbol)

	// 일부 블라인드 처리
	symbol.ID = -1

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbol": symbol,
	})
}

// @Summary		특정 심볼 현재가 조회
// @Description	특정 심볼의 현재가 정보를 반환합니다.
// @Tags			Market - Status
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Success		200				{object}	map[string]string	"성공 시 심볼 현재가 정보 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/market/symbol/{symbol}/now [get]
func (sr *SymbolsRouter) symbolNow(c *fiber.Ctx) error {
	symbolParam := c.Params("symbol")

	// 현재가를 가져와야함 현재가는 가장 최근에 체결된 가격 -> 전일 종가 -> 공모가 순으로 가져옴
	var currentPrice float64
	if ws.TempLedger[symbolParam] != nil && ws.TempLedger[symbolParam].Size() != 0 {
		// 가장 최근 체결 가격(상장 직후라면 상장가)
		currentPrice = ws.TempLedger[symbolParam].GetMostRecent().Price
	} else {
		// TODO 전일 종가로 설정

		// 전일 종가도 없으면 공모가로 설정 (단순 Fallback 용 아마 여기까지 올일은 없을듯)
		ipoPrice, err := postgresApp.Get().SymbolRepo().GetIPOPrice(c.Context(), symbolParam)
		if err != nil {
			//log.Printf("Error fetching IPO price for %s: %v", symbolParam, err)
			return template.ErrorHandler(c, fiber.StatusInternalServerError, "Error fetching current price")
		} else {
			currentPrice = ipoPrice
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbol":       symbolParam,
		"currentPrice": currentPrice,
	})
}
