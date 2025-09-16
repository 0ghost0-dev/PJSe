package market

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/exchanges"
	"PJS_Exchange/middlewares/auth"
	"PJS_Exchange/middlewares/session"
	"PJS_Exchange/routes/ws"
	t "PJS_Exchange/template"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type OrdersRouter struct{}

func (or *OrdersRouter) RegisterRoutes(router fiber.Router) {
	ordersGroup := router.Group("/orders")

	symbolExist := func(c *fiber.Ctx) error {
		symbol := c.Params("sym")
		exist, _ := postgresApp.Get().SymbolRepo().IsSymbolExist(c.Context(), symbol)
		if !exist {
			return fiber.NewError(fiber.StatusNotFound, "Symbol '"+symbol+"' does not exist.")
		}
		return c.Next()
	}

	ordersGroup.Get("/:sym",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderRead: true,
		}), session.IsOnline(), or.getOrders)
	ordersGroup.Post("/:sym/buy",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderCreate: true,
		}), session.IsOnline(), symbolExist, or.buyOrder)
	ordersGroup.Patch("/:sym/buy",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderModify: true,
		}), session.IsOnline(), symbolExist, or.modifyBuyOrder)
	ordersGroup.Delete("/:sym/buy",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderCancel: true,
		}), session.IsOnline(), symbolExist, or.cancelBuyOrder)
	ordersGroup.Post("/:sym/sell",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderCreate: true,
		}), session.IsOnline(), symbolExist, or.sellOrder)
	ordersGroup.Patch("/:sym/sell",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderModify: true,
		}), session.IsOnline(), symbolExist, or.modifySellOrder)
	ordersGroup.Delete("/:sym/sell",
		auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
			OrderCancel: true,
		}), session.IsOnline(), symbolExist, or.cancelSellOrder)
}

// @Summary 주문 조회
// @Description 사용자의 모든 주문을 조회합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Success		200				{object}	map[string]interface{}	"사용자의 모든 주문 정보"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol} [get]
func (or *OrdersRouter) getOrders(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	user := c.Locals("user").(*postgresql.User)

	orders := make([]t.OrderStatus, 0)

	depth := ws.TempDepth[symbol]
	// 매수 주문
	for price, orderList := range depth.Bids {
		for _, order := range orderList {
			if order.UserID == user.ID {
				orders = append(orders, t.OrderStatus{
					OrderID: order.OrderID,
					Side:    t.SideBuy,
					OrderType: func() string {
						if price == 0 {
							return t.OrderTypeMarket
						} else {
							return t.OrderTypeLimit
						}
					}(),
					Price:    price,
					Quantity: order.Quantity,
				})
			}
		}
	}

	// 매도 주문
	for price, orderList := range depth.Asks {
		for _, order := range orderList {
			if order.UserID == user.ID {
				orders = append(orders, t.OrderStatus{
					OrderID: order.OrderID,
					Side:    t.SideSell,
					OrderType: func() string {
						if price == 0 {
							return t.OrderTypeMarket
						} else {
							return t.OrderTypeLimit
						}
					}(),
					Price:    price,
					Quantity: order.Quantity,
				})
			}
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbol": symbol,
		"orders": orders,
	})
}

/// Buy Orders

// TODO: 추후 protobuf로 변경
// @Summary 매수 주문
// @Description 지정가, 시장가 주문을 접수합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CreateOrderRequest		true	"주문 정보"
// @Success		201				{object}	map[string]string	"주문이 성공적으로 접수되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/buy [post]
func (or *OrdersRouter) buyOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	orderRequest := t.OrderRequest{}

	if err := c.BodyParser(&orderRequest); err != nil {
		return t.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 서버 측에서 설정
	orderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	orderRequest.OrderID = uuid.NewString()
	orderRequest.Symbol = symbol
	orderRequest.Side = t.SideBuy
	orderRequest.Status = t.StatusOpen
	orderRequest.ResultChan = make(chan t.Result, 1)

	// 주문 처리
	select {
	case exchanges.OP.OrderRequestChan <- orderRequest:
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	select {
	case result := <-orderRequest.ResultChan:
		if !result.Success {
			return t.ErrorHandler(c, result.Code, "Failed to place buy order: "+result.Message)
		}
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Buy order placed successfully",
		"orderID": orderRequest.OrderID,
	})
}

// @Summary 매수 주문 수정
// @Description 기존 매수 주문을 수정합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.ModifyOrderRequest	true	"수정할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 수정되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/buy [patch]
func (or *OrdersRouter) modifyBuyOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	orderRequest := t.OrderRequest{}

	if err := c.BodyParser(&orderRequest); err != nil {
		return t.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	orderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	orderRequest.Symbol = symbol
	orderRequest.Side = t.SideBuy
	orderRequest.Status = t.StatusModified
	orderRequest.ResultChan = make(chan t.Result, 1)

	// 주문 처리
	select {
	case exchanges.OP.OrderRequestChan <- orderRequest:
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	select {
	case result := <-orderRequest.ResultChan:
		if !result.Success {
			return t.ErrorHandler(c, result.Code, "Failed to modify buy order: "+result.Message)
		}
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Modify buy order successfully",
		"orderID": orderRequest.OrderID,
	})
}

// @Summary 매수 주문 취소
// @Description 기존 매수 주문을 취소합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CancelOrderRequest	true	"취소할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 취소되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/buy [delete]
func (or *OrdersRouter) cancelBuyOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	orderRequest := t.OrderRequest{}

	if err := c.BodyParser(&orderRequest); err != nil {
		return t.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	orderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	orderRequest.Symbol = symbol
	orderRequest.Side = t.SideBuy
	orderRequest.Status = t.StatusCanceled
	orderRequest.ResultChan = make(chan t.Result, 1)

	// 주문 처리
	select {
	case exchanges.OP.OrderRequestChan <- orderRequest:
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	select {
	case result := <-orderRequest.ResultChan:
		if !result.Success {
			return t.ErrorHandler(c, result.Code, "Failed to cancel buy order: "+result.Message)
		}
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Cancel buy order successfully",
		"orderID": orderRequest.OrderID,
	})
}

/// Sell Orders

// @Summary 매도 주문
// @Description 지정가, 시장가 주문을 접수합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CreateOrderRequest		true	"주문 정보"
// @Success		201				{object}	map[string]string	"주문이 성공적으로 접수되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/sell [post]
func (or *OrdersRouter) sellOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	orderRequest := t.OrderRequest{}

	if err := c.BodyParser(&orderRequest); err != nil {
		return t.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 서버 측에서 설정
	orderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	orderRequest.OrderID = uuid.NewString()
	orderRequest.Symbol = symbol
	orderRequest.Side = t.SideSell
	orderRequest.Status = t.StatusOpen
	orderRequest.ResultChan = make(chan t.Result, 1)

	// 주문 처리
	select {
	case exchanges.OP.OrderRequestChan <- orderRequest:
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	select {
	case result := <-orderRequest.ResultChan:
		if !result.Success {
			return t.ErrorHandler(c, result.Code, "Failed to place sell order: "+result.Message)
		}
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Sell order placed successfully",
		"orderID": orderRequest.OrderID,
	})
}

// @Summary 매도 주문 수정
// @Description 기존 매도 주문을 수정합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.ModifyOrderRequest	true	"수정할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 수정되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/sell [patch]
func (or *OrdersRouter) modifySellOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	orderRequest := t.OrderRequest{}

	if err := c.BodyParser(&orderRequest); err != nil {
		return t.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 서버 측에서 설정
	orderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	orderRequest.Symbol = symbol
	orderRequest.Side = t.SideSell
	orderRequest.Status = t.StatusModified
	orderRequest.ResultChan = make(chan t.Result, 1)

	// 주문 처리
	select {
	case exchanges.OP.OrderRequestChan <- orderRequest:
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	select {
	case result := <-orderRequest.ResultChan:
		if !result.Success {
			return t.ErrorHandler(c, result.Code, "Failed to modify sell order")
		}
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Modify sell order successfully",
		"orderID": orderRequest.OrderID,
	})
}

// @Summary 매도 주문 취소
// @Description 기존 매도 주문을 취소합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: NVDA)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CancelOrderRequest	true	"취소할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 취소되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Failure		503				{object}	map[string]string	"장이 닫혔을 때 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/sell [delete]
func (or *OrdersRouter) cancelSellOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	orderRequest := t.OrderRequest{}

	if err := c.BodyParser(&orderRequest); err != nil {
		return t.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	orderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	orderRequest.Symbol = symbol
	orderRequest.Side = t.SideSell
	orderRequest.Status = t.StatusCanceled
	orderRequest.ResultChan = make(chan t.Result, 1)

	// 주문 처리
	select {
	case exchanges.OP.OrderRequestChan <- orderRequest:
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	select {
	case result := <-orderRequest.ResultChan:
		if !result.Success {
			return t.ErrorHandler(c, result.Code, "Failed to cancel sell order")
		}
	case <-time.After(5 * time.Second):
		return t.ErrorHandler(c, fiber.StatusServiceUnavailable, "Order processing is busy, please try again later")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Cancel sell order successfully",
		"orderID": orderRequest.OrderID,
	})
}
