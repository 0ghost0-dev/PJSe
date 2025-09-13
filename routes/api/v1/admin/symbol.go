package admin

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgres"
	"PJS_Exchange/middleware"
	"context"

	"github.com/gofiber/fiber/v2"
)

type SymbolRouter struct{}

func (sr *SymbolRouter) RegisterRoutes(router fiber.Router) {
	adminSymbolGroup := router.Group("/symbol", middleware.AuthAPIKeyMiddlewareRequireScopes(postgres.APIKeyScope{
		AdminSymbolManage: true,
	}))

	adminSymbolGroup.Get("/", sr.symbolList)
	adminSymbolGroup.Get("/:symbol", sr.symbolDetail)
	adminSymbolGroup.Post("/", sr.symbolListing)
	adminSymbolGroup.Delete("/:symbol", sr.symbolDelisting)
	adminSymbolGroup.Patch("/:symbol/status/active", sr.enableTradeSymbol)
	adminSymbolGroup.Patch("/:symbol/status/suspend", sr.disableTradeSymbol)
	adminSymbolGroup.Patch("/:symbol/tick-size", sr.setTickSizeSymbol)
	adminSymbolGroup.Patch("/:symbol/minimum-order-quantity", sr.setMinimumOrderQuantitySymbol)
}

// === 핸들러 함수들 ===

// symbolList 상장된 심볼 리스트 조회
func (sr *SymbolRouter) symbolList(c *fiber.Ctx) error {
	ctx := context.Background()
	symbols, err := app.GetApp().SymbolRepo().GetSymbols(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch symbols: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbols": symbols,
	})
}

// symbolDetail 상장된 특정 심볼 조회
func (sr *SymbolRouter) symbolDetail(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	symbol, err := app.GetApp().SymbolRepo().GetSymbolData(ctx, symbolParam)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch symbol: " + err.Error(),
			"code":  fiber.StatusInternalServerError,
		})
	}
	if symbol == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Symbol not found",
			"code":  fiber.StatusNotFound,
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbol": symbol,
	})
}

// symbolListing 심볼 상장 요청
func (sr *SymbolRouter) symbolListing(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Not implemented",
		"code":  fiber.StatusNotImplemented,
	})
}

// symbolDelisting 심볼 상장 폐지
func (sr *SymbolRouter) symbolDelisting(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Not implemented",
		"code":  fiber.StatusNotImplemented,
	})
}

// enableTradeSymbol 심볼 거래 활성화
func (sr *SymbolRouter) enableTradeSymbol(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Not implemented",
		"code":  fiber.StatusNotImplemented,
	})
}

// disableTradeSymbol 심볼 거래 비활성화
func (sr *SymbolRouter) disableTradeSymbol(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Not implemented",
		"code":  fiber.StatusNotImplemented,
	})
}

// setTickSizeSymbol 심볼 틱 사이즈 설정
func (sr *SymbolRouter) setTickSizeSymbol(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Not implemented",
		"code":  fiber.StatusNotImplemented,
	})
}

// setMinimumOrderQuantitySymbol 심볼 최소 주문 수량 설정
func (sr *SymbolRouter) setMinimumOrderQuantitySymbol(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Not implemented",
		"code":  fiber.StatusNotImplemented,
	})
}
