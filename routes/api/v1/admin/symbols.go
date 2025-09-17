package admin

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middlewares/auth"
	"PJS_Exchange/routes/ws"
	"PJS_Exchange/template"
	"PJS_Exchange/utils"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type SymbolRouter struct{}

func (sr *SymbolRouter) RegisterRoutes(router fiber.Router) {
	adminSymbolGroup := router.Group("/symbols", auth.APIKeyMiddlewareRequireScopes(auth.Config{
		Bypass: false,
	}, postgresql.APIKeyScope{
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
// @Success		200				{object}	map[string][]postgresql.Symbol	"성공 시 심볼 목록 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol [get]
func (sr *SymbolRouter) symbolList(c *fiber.Ctx) error {
	symbols, err := postgresApp.Get().SymbolRepo().GetSymbols(c.Context())
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
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]postgresql.Symbol	"성공 시 심볼 상세 정보 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol} [get]
func (sr *SymbolRouter) symbolDetail(c *fiber.Ctx) error {
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}

	symbol, err := postgresApp.Get().SymbolRepo().GetSymbolData(c.Context(), symbolParam)
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
// @Param			symbol	body		postgresql.Symbol		true	"상장할 심볼 정보"
// @Param			Authorization	header		string 				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		201		{object}	map[string]postgresql.Symbol	"성공 시 상장된 심볼 정보 반환"
// @Failure		400		{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500		{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401		{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol [post]
func (sr *SymbolRouter) symbolListing(c *fiber.Ctx) error {
	var req struct {
		postgresql.Symbol
	}
	if err := c.BodyParser(&req); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if postgresql.IsSymbolStructComplete(&req.Symbol) == false {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "All fields are required")
	}

	// 요청 값과 상관없이 강제로 상태를 비활성화로 설정
	req.Status = postgresql.Status{
		Status: postgresql.StatusInit,
		Reason: "",
	}

	symbol, err := postgresApp.Get().SymbolRepo().SymbolListing(c.Context(), &req.Symbol)
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to list symbol: "+err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(symbol)
}

// @Summary		심볼 거래 준비 완료 (비활성화 상태로 변경)
// @Description	심볼을 거래 준비 완료 상태로 변경합니다. (비활성화 상태로 설정)
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]string	"성공 시 상태 변경 메시지 반환"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol}/status/inactivate [patch]
func (sr *SymbolRouter) readyTradeSymbol(c *fiber.Ctx) error {
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}

	err := postgresApp.Get().SymbolRepo().UpdateSymbolStatus(c.Context(), symbolParam, postgresql.Status{
		Status: postgresql.StatusInactive,
		Reason: "",
	})
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update symbol status: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Symbol status updated to inactive",
	})
}

// @Summary		심볼 거래 활성화
// @Description	심볼을 거래 활성화 상태로 변경합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param price			header		float64				false	"상장가 ("inactive" 상태에서 "active" 상태로 변경 시에만 사용됨)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]string	"성공 시 상태 변경 메시지 반환"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol}/status/activate [patch]
func (sr *SymbolRouter) enableTradeSymbol(c *fiber.Ctx) error {
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}

	if c.GetRespHeader("price", "") != "" {
		priceHeader := c.GetRespHeader("price", "")
		price, err := strconv.ParseFloat(priceHeader, 64)
		if err != nil || price <= 0 {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid price value")
		}

		sym, err := postgresApp.Get().SymbolRepo().GetSymbolData(c.Context(), symbolParam)
		if err != nil {
			return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to fetch symbol: "+err.Error())
		}

		if sym.Status.Status != postgresql.StatusInactive {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Price can only be set when changing status from 'inactive' to 'active'")
		}

		ledger := template.Ledger{
			Timestamp: time.Now().UnixMilli(),
			Symbol:    symbolParam,
			Price:     price,
			Volume:    0,
		}
		send, _ := json.Marshal(ledger)
		if ws.TempLedger[symbolParam] == nil {
			ws.TempLedger[symbolParam] = utils.NewQueue()
		}
		ws.TempLedger[symbolParam].PushFront(ledger)
		ws.LedgerHub.BroadcastMessage(time.Now().UnixMilli(), websocket.TextMessage, send)
	}

	_ = postgresApp.Get().SymbolRepo().UpdateSymbolStatus(c.Context(), symbolParam, postgresql.Status{
		Status: postgresql.StatusActive,
		Reason: "",
	})
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Symbol status updated to active",
	})
}

// @Summary		심볼 거래 비활성화
// @Description	심볼을 거래 비활성화 상태로 변경합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			reason			header		string				false	"비활성화 사유"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]string	"성공 시 상태 변경 메시지 반환"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol}/status/suspend [patch]
func (sr *SymbolRouter) disableTradeSymbol(c *fiber.Ctx) error {
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}
	reasonHeader := c.GetRespHeader("reason", "")

	err := postgresApp.Get().SymbolRepo().UpdateSymbolStatus(c.Context(), symbolParam, postgresql.Status{
		Status: postgresql.StatusSuspended,
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

// @Summary		심볼 상장 폐지
// @Description	심볼을 상장 폐지 상태로 변경합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			reason			header		string				false	"상장 폐지 사유"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]string	"성공 시 상장 폐지 메시지 반환"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol} [delete]
func (sr *SymbolRouter) symbolDelisting(c *fiber.Ctx) error {
	symbolParam := c.Params("symbol")
	if symbolParam == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Symbol parameter is required")
	}
	reasonHeader := c.GetRespHeader("reason", "")

	err := postgresApp.Get().SymbolRepo().UpdateSymbolStatus(c.Context(), symbolParam, postgresql.Status{
		Status: postgresql.StatusDelisted,
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

// @Summary		틱 사이즈 설정
// @Description	심볼의 틱 사이즈를 설정합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			tick_size		query		number				true	"틱 사이즈 (예: 0.01)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200				{object}	map[string]interface{}	"성공 시 틱 사이즈 변경 메시지 반환"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol}/tick-size [patch]
func (sr *SymbolRouter) setTickSizeSymbol(c *fiber.Ctx) error {
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

	err = postgresApp.Get().SymbolRepo().SetTickSize(c.Context(), symbolParam, float32(tickSize))
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update tick size: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":   "Tick size updated successfully",
		"tick_size": tickSize,
	})
}

// @Summary		최소 주문 수량 설정
// @Description	심볼의 최소 주문 수량을 설정합니다.
// @Tags			Admin - Symbol
// @Produce		json
// @Param			symbol					path		string				true	"심볼 (예: NVDA)"
// @Param			minimum_order_quantity	query		number				true	"최소 주문 수량 (예: 1)"
// @Param			Authorization			header		string				true	"Bearer {API_KEY}"	with	AdminSymbolManage	Scope
// @Success		200						{object}	map[string]interface{}	"성공 시 최소 주문 수량 변경 메시지 반환"
// @Failure		400						{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		500						{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		401						{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Router			/api/v1/admin/symbol/{symbol}/minimum-order-quantity [patch]
func (sr *SymbolRouter) setMinimumOrderQuantitySymbol(c *fiber.Ctx) error {
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

	err = postgresApp.Get().SymbolRepo().SetMinimumOrderQuantity(c.Context(), symbolParam, float32(minOrderQty))
	if err != nil {
		return template.ErrorHandler(c, fiber.StatusInternalServerError, "Failed to update minimum order quantity: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":                "Minimum order quantity updated successfully",
		"minimum_order_quantity": minOrderQty,
	})
}
