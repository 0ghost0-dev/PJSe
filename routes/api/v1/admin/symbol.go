package admin

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgres"
	"PJS_Exchange/middleware"
	"PJS_Exchange/template"
	"context"
	"strconv"

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
	adminSymbolGroup.Patch("/:symbol/status/activate", sr.enableTradeSymbol)
	adminSymbolGroup.Patch("/:symbol/status/suspend", sr.disableTradeSymbol)
	adminSymbolGroup.Patch("/:symbol/status/inactivate", sr.readyTradeSymbol)
	adminSymbolGroup.Patch("/:symbol/tick-size", sr.setTickSizeSymbol)
	adminSymbolGroup.Patch("/:symbol/minimum-order-quantity", sr.setMinimumOrderQuantitySymbol)
}

// === 핸들러 함수들 ===

// @Summary		상장된 모든 심볼 조회
// @Description	상장된 모든 심볼의 리스트를 반환합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string][]postgres.Symbol	"성공 시 심볼 목록 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol [get]
func (sr *SymbolRouter) symbolList(c *fiber.Ctx) error {
	ctx := context.Background()
	symbols, err := app.GetApp().SymbolRepo().GetSymbols(ctx)
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to fetch symbols: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbols": symbols,
	})
}

// @Summary		특정 심볼 정보 조회
// @Description	특정 심볼의 정보를 반환합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]postgres.Symbol	"성공 시 심볼 상세 정보 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol} [get]
func (sr *SymbolRouter) symbolDetail(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}

	symbol, err := app.GetApp().SymbolRepo().GetSymbolData(ctx, symbolParam)
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to fetch symbol: "+err.Error())
	}
	if symbol == nil {
		return template.ErrorHandler(c, fiber.StatusNotFound, "Symbol not found")
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbol": symbol,
	})
}

// @Summary		심볼 상장
// @Description	새로운 심볼을 상장합니다.
// @Tags			Admin - Symbol
// @Accept		json
// @Produce		json
// @Param			symbol	body		postgres.Symbol		true	"상장할 심볼 정보"
// @Param			Authorization	header		string 				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		201		{object}	map[string]postgres.Symbol	"성공 시 상장된 심볼 정보 반환"
// @Failure		400		{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500		{object}	map[string]string	"서버 오류 시 에러 메시지 반환"
// @Failure		401		{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol [post]
func (sr *SymbolRouter) symbolListing(c *fiber.Ctx) error {
	var req struct {
		postgres.Symbol
	}
	if err := c.BodyParser(&req); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 요청 값과 상관없이 강제로 상태를 비활성화로 설정
	req.Status = postgres.Status{
		Status: postgres.StatusInit,
		Reason: "",
	}

	ctx := context.Background()
	symbol, err := app.GetApp().SymbolRepo().SymbolListing(ctx, &req.Symbol)
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to list symbol: "+err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(symbol)
}

// disableTradeSymbol 심볼 거래 비활성화
func (sr *SymbolRouter) readyTradeSymbol(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}

	err := app.GetApp().SymbolRepo().UpdateSymbolStatus(ctx, symbolParam, postgres.Status{
		Status: postgres.StatusInactive,
		Reason: "",
	})
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update symbol status: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Symbol status updated to inactive",
	})
}

// enableTradeSymbol 심볼 거래 활성화
func (sr *SymbolRouter) enableTradeSymbol(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}

	err := app.GetApp().SymbolRepo().UpdateSymbolStatus(ctx, symbolParam, postgres.Status{
		Status: postgres.StatusActive,
		Reason: "",
	})
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update symbol status: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Symbol status updated to active",
	})
}

// disableTradeSymbol 심볼 거래 비활성화
func (sr *SymbolRouter) disableTradeSymbol(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}
	reasonHeader := c.GetRespHeader("reason", "")

	err := app.GetApp().SymbolRepo().UpdateSymbolStatus(ctx, symbolParam, postgres.Status{
		Status: postgres.StatusSuspended,
		Reason: reasonHeader,
	})
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update symbol status: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Symbol status updated to suspended",
		"reason":  reasonHeader,
	})
}

// symbolDelisting 심볼 상장 폐지
func (sr *SymbolRouter) symbolDelisting(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}
	reasonHeader := c.GetRespHeader("reason", "")

	err := app.GetApp().SymbolRepo().UpdateSymbolStatus(ctx, symbolParam, postgres.Status{
		Status: postgres.StatusDelisted,
		Reason: reasonHeader,
	})
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to delist symbol: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Symbol delisted successfully",
		"reason":  reasonHeader,
	})
}

// setTickSizeSymbol 심볼 틱 사이즈 설정
func (sr *SymbolRouter) setTickSizeSymbol(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}
	tickSizeQueryParam := c.Query("tick_size")
	if tickSizeQueryParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "tick_size query parameter is required")
	}
	tickSize, err := strconv.ParseFloat(tickSizeQueryParam, 32)
	if err != nil || tickSize <= 0 {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid tick_size value")
	}

	err = app.GetApp().SymbolRepo().SetTickSize(ctx, symbolParam, float32(tickSize))
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update tick size: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":   "Tick size updated successfully",
		"tick_size": tickSize,
	})
}

// setMinimumOrderQuantitySymbol 심볼 최소 주문 수량 설정
func (sr *SymbolRouter) setMinimumOrderQuantitySymbol(c *fiber.Ctx) error {
	ctx := context.Background()
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}
	minOrderQtyQueryParam := c.Query("minimum_order_quantity")
	if minOrderQtyQueryParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "minimum_order_quantity query parameter is required")
	}
	minOrderQty, err := strconv.ParseFloat(minOrderQtyQueryParam, 32)
	if err != nil || minOrderQty <= 0 {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid minimum_order_quantity value")
	}

	err = app.GetApp().SymbolRepo().SetMinimumOrderQuantity(ctx, symbolParam, float32(minOrderQty))
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update minimum order quantity: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":                "Minimum order quantity updated successfully",
		"minimum_order_quantity": minOrderQty,
	})
}
