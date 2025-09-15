package market

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/exchanges"
	"PJS_Exchange/middleware"
	"PJS_Exchange/routes/ws"
	"PJS_Exchange/template"
	json2 "encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

var (
	OrderTypeLimit        = "limit"
	OrderTypeMarket       = "market"
	OrderTypeStopLimit    = "stop-limit"
	SideBuy               = "buy"
	SideSell              = "sell"
	StatusOpen            = "open"
	StatusModified        = "modified"
	StatusPartiallyFilled = "partially_filled"
	StatusFilled          = "filled"
	StatusCanceled        = "canceled"
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
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderRead: true,
		}), or.getOrders)
	ordersGroup.Post("/:sym/buy",
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderCreate: true,
		}), symbolExist, or.buyOrder, exchanges.ProcessOrders())
	ordersGroup.Patch("/:sym/buy",
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderModify: true,
		}), symbolExist, or.modifyBuyOrder, exchanges.ProcessOrders())
	ordersGroup.Delete("/:sym/buy",
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderCancel: true,
		}), symbolExist, or.cancelBuyOrder)
	ordersGroup.Post("/:sym/sell",
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderCreate: true,
		}), symbolExist, or.sellOrder, exchanges.ProcessOrders())
	ordersGroup.Patch("/:sym/sell",
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderModify: true,
		}), symbolExist, or.modifySellOrder, exchanges.ProcessOrders())
	ordersGroup.Delete("/:sym/sell",
		middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
			OrderCancel: true,
		}), symbolExist, or.cancelSellOrder)
}

// @Summary 주문 조회
// @Description 사용자의 모든 주문을 조회합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Success		200				{object}	map[string]interface{}	"사용자의 모든 주문 정보"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol} [get]
func (or *OrdersRouter) getOrders(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	user := c.Locals("user").(*postgresql.User)

	orders := make([]template.OrderStatus, 0)

	depth := ws.TempDepth[symbol]
	// 매수 주문
	for price, orderList := range depth.Bids {
		for _, order := range orderList {
			if order.UserID == user.ID {
				orders = append(orders, template.OrderStatus{
					OrderID: order.OrderID,
					Side:    SideBuy,
					OrderType: func() string {
						if price == 0 {
							return OrderTypeMarket
						} else {
							return OrderTypeLimit
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
				orders = append(orders, template.OrderStatus{
					OrderID: order.OrderID,
					Side:    SideSell,
					OrderType: func() string {
						if price == 0 {
							return OrderTypeMarket
						} else {
							return OrderTypeLimit
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
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CreateOrderRequest		true	"주문 정보"
// @Success		201				{object}	map[string]string	"주문이 성공적으로 접수되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/buy [post]
func (or *OrdersRouter) buyOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	createBuyOrderRequest := template.OrderRequest{}

	if err := c.BodyParser(&createBuyOrderRequest); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 서버 측에서 설정
	createBuyOrderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	createBuyOrderRequest.OrderID = uuid.NewString()
	createBuyOrderRequest.Timestamp = c.Context().Time().UnixMilli()
	createBuyOrderRequest.Symbol = symbol
	createBuyOrderRequest.Side = SideBuy
	createBuyOrderRequest.Status = StatusOpen

	// 입력 검증
	if createBuyOrderRequest.Quantity <= 0 {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Quantity must be greater than zero")
	}
	if createBuyOrderRequest.OrderType != OrderTypeMarket && createBuyOrderRequest.OrderType != OrderTypeLimit && createBuyOrderRequest.OrderType != OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid order type")
	}

	// 주문 처리
	depth := ws.TempDepth[symbol]

	if depth.TotalBids == nil {
		depth.TotalBids = make(map[float64]int)
	}
	if depth.Bids == nil {
		depth.Bids = make(map[float64][]template.Order)
	}

	if createBuyOrderRequest.OrderType == OrderTypeLimit {
		if createBuyOrderRequest.Price <= 0 {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Price must be greater than zero for limit orders")
		}

		depth.TotalBids[createBuyOrderRequest.Price] += createBuyOrderRequest.Quantity
		depth.Bids[createBuyOrderRequest.Price] = append(depth.Bids[createBuyOrderRequest.Price], template.Order{
			UserID:   createBuyOrderRequest.UserID,
			OrderID:  createBuyOrderRequest.OrderID,
			Quantity: createBuyOrderRequest.Quantity,
		})
	}
	if createBuyOrderRequest.OrderType == OrderTypeMarket {
		createBuyOrderRequest.Price = 0 // 시장가 주문의 경우 가격은 무시됨

		depth.TotalBids[createBuyOrderRequest.Price] += createBuyOrderRequest.Quantity
		depth.Bids[createBuyOrderRequest.Price] = append(depth.Bids[createBuyOrderRequest.Price], template.Order{
			UserID:   createBuyOrderRequest.UserID,
			OrderID:  createBuyOrderRequest.OrderID,
			Quantity: createBuyOrderRequest.Quantity,
		})
	}
	if createBuyOrderRequest.OrderType == OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Not implemented: Stop-Limit order")
	}

	ws.TempDepth[symbol] = depth

	// 호가 갱신 브로드캐스트
	update := template.UpdateDepth{
		Timestamp: createBuyOrderRequest.Timestamp,
		Symbol:    symbol,
		Side:      "bids",
		Price:     createBuyOrderRequest.Price,
		Quantity:  depth.TotalBids[createBuyOrderRequest.Price],
	}
	newDepth, _ := json2.Marshal(update)
	ws.DepthHub.BroadcastMessage(createBuyOrderRequest.Timestamp, websocket.TextMessage, newDepth)

	// 주문 접수 알림
	orderCheck, _ := json2.Marshal(createBuyOrderRequest)
	ws.NotifyHub.SendMessageToUser(createBuyOrderRequest.UserID, createBuyOrderRequest.Timestamp, websocket.TextMessage, orderCheck)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Buy order placed successfully",
		"orderID": createBuyOrderRequest.OrderID,
	})
}

// @Summary 매수 주문 수정
// @Description 기존 매수 주문을 수정합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.ModifyOrderRequest	true	"수정할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 수정되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/buy [patch]
func (or *OrdersRouter) modifyBuyOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	user := c.Locals("user").(*postgresql.User)
	modifyBuyOrderRequest := template.OrderRequest{}

	if err := c.BodyParser(&modifyBuyOrderRequest); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	modifyBuyOrderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	modifyBuyOrderRequest.Timestamp = c.Context().Time().UnixMilli()
	modifyBuyOrderRequest.Symbol = symbol
	modifyBuyOrderRequest.Side = SideBuy
	modifyBuyOrderRequest.Status = StatusModified

	depthIndex := make([]interface{}, 2)

	// 입력 검증
	if modifyBuyOrderRequest.OrderID == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID is required")
	} else {
		// 존재하는 주문인지 확인 TODO 가격별로 확인하는 것은 비효율적이므로, 추후 주문ID로 바로 조회할 수 있도록 개선 필요
		exist := false
		for i, orders := range ws.TempDepth[symbol].Bids {
			for j, order := range orders {
				if order.OrderID == modifyBuyOrderRequest.OrderID && order.UserID == user.ID {
					depthIndex[0] = i
					depthIndex[1] = j
					exist = true
					break
				}
			}
		}
		if !exist {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID does not exist")
		}
	}
	if modifyBuyOrderRequest.Quantity <= 0 {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Quantity must be greater than zero")
	}
	if modifyBuyOrderRequest.OrderType != OrderTypeMarket && modifyBuyOrderRequest.OrderType != OrderTypeLimit && modifyBuyOrderRequest.OrderType != OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid order type")
	}

	// 주문 수정 처리
	depth := ws.TempDepth[symbol]

	// 우선 기존 주문 제거
	price := depthIndex[0].(float64)
	index := depthIndex[1].(int)
	depth.TotalBids[price] -= depth.Bids[price][index].Quantity
	depth.Bids[price] = append(depth.Bids[price][:index], depth.Bids[price][index+1:]...)

	if price != modifyBuyOrderRequest.Price {
		// 호가 갱신 브로드캐스트
		update := template.UpdateDepth{
			Timestamp: modifyBuyOrderRequest.Timestamp,
			Symbol:    symbol,
			Side:      "bids",
			Price:     price,
			Quantity:  depth.TotalBids[price],
		}
		newDepth, _ := json2.Marshal(update)
		ws.DepthHub.BroadcastMessage(modifyBuyOrderRequest.Timestamp, websocket.TextMessage, newDepth)
	}

	if modifyBuyOrderRequest.OrderType == OrderTypeLimit {
		if modifyBuyOrderRequest.Price <= 0 {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Price must be greater than zero for limit orders")
		}

		// 새로운 주문 추가
		depth.TotalBids[modifyBuyOrderRequest.Price] += modifyBuyOrderRequest.Quantity
		depth.Bids[modifyBuyOrderRequest.Price] = append(depth.Bids[modifyBuyOrderRequest.Price], template.Order{
			UserID:   modifyBuyOrderRequest.UserID,
			OrderID:  modifyBuyOrderRequest.OrderID,
			Quantity: modifyBuyOrderRequest.Quantity,
		})
	}
	if modifyBuyOrderRequest.OrderType == OrderTypeMarket {
		modifyBuyOrderRequest.Price = 0 // 시장가 주문의 경우 가격은 무시됨

		// 새로운 주문 추가
		depth.TotalBids[modifyBuyOrderRequest.Price] += modifyBuyOrderRequest.Quantity
		depth.Bids[modifyBuyOrderRequest.Price] = append(depth.Bids[modifyBuyOrderRequest.Price], template.Order{
			UserID:   modifyBuyOrderRequest.UserID,
			OrderID:  modifyBuyOrderRequest.OrderID,
			Quantity: modifyBuyOrderRequest.Quantity,
		})
	}
	if modifyBuyOrderRequest.OrderType == OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Not implemented: Stop-Limit order modification")
	}

	ws.TempDepth[symbol] = depth

	// 호가 갱신 브로드캐스트
	update := template.UpdateDepth{
		Timestamp: modifyBuyOrderRequest.Timestamp,
		Symbol:    symbol,
		Side:      "bids",
		Price:     modifyBuyOrderRequest.Price,
		Quantity:  depth.TotalBids[modifyBuyOrderRequest.Price],
	}
	newDepth, _ := json2.Marshal(update)
	ws.DepthHub.BroadcastMessage(modifyBuyOrderRequest.Timestamp, websocket.TextMessage, newDepth)

	// 주문 수정 알림
	orderCheck, _ := json2.Marshal(modifyBuyOrderRequest)
	ws.NotifyHub.SendMessageToUser(modifyBuyOrderRequest.UserID, modifyBuyOrderRequest.Timestamp, websocket.TextMessage, orderCheck)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Modify buy order successfully",
		"orderID": modifyBuyOrderRequest.OrderID,
	})
}

// @Summary 매수 주문 취소
// @Description 기존 매수 주문을 취소합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CancelOrderRequest	true	"취소할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 취소되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/buy [delete]
func (or *OrdersRouter) cancelBuyOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	user := c.Locals("user").(*postgresql.User)
	cancelBuyOrderRequest := template.OrderRequest{}

	if err := c.BodyParser(&cancelBuyOrderRequest); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	cancelBuyOrderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	cancelBuyOrderRequest.Timestamp = c.Context().Time().UnixMilli()
	cancelBuyOrderRequest.Symbol = symbol
	cancelBuyOrderRequest.Side = SideBuy
	cancelBuyOrderRequest.Status = StatusCanceled

	depthIndex := make([]interface{}, 2)

	// 입력 검증
	if cancelBuyOrderRequest.OrderID == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID is required")
	} else {
		// 존재하는 주문인지 확인 TODO 가격별로 확인하는 것은 비효율적이므로, 추후 주문ID로 바로 조회할 수 있도록 개선 필요
		exist := false
		for i, orders := range ws.TempDepth[symbol].Bids {
			for j, order := range orders {
				if order.OrderID == cancelBuyOrderRequest.OrderID && order.UserID == user.ID {
					depthIndex[0] = i
					depthIndex[1] = j
					exist = true
					break
				}
			}
		}
		if !exist {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID does not exist")
		}
	}

	// 주문 취소 처리
	depth := ws.TempDepth[symbol]
	price := depthIndex[0].(float64)
	index := depthIndex[1].(int)
	depth.TotalBids[price] -= depth.Bids[price][index].Quantity
	depth.Bids[price] = append(depth.Bids[price][:index], depth.Bids[price][index+1:]...)
	ws.TempDepth[symbol] = depth

	// 호가 갱신 브로드캐스트
	update := template.UpdateDepth{
		Timestamp: cancelBuyOrderRequest.Timestamp,
		Symbol:    symbol,
		Side:      "bids",
		Price:     price,
		Quantity:  depth.TotalBids[price],
	}
	newDepth, _ := json2.Marshal(update)
	ws.DepthHub.BroadcastMessage(cancelBuyOrderRequest.Timestamp, websocket.TextMessage, newDepth)

	// 주문 취소 알림
	orderCheck, _ := json2.Marshal(cancelBuyOrderRequest)
	ws.NotifyHub.SendMessageToUser(cancelBuyOrderRequest.UserID, cancelBuyOrderRequest.Timestamp, websocket.TextMessage, orderCheck)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Cancel buy order successfully",
		"orderID": cancelBuyOrderRequest.OrderID,
	})
}

/// Sell Orders

// @Summary 매도 주문
// @Description 지정가, 시장가 주문을 접수합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CreateOrderRequest		true	"주문 정보"
// @Success		201				{object}	map[string]string	"주문이 성공적으로 접수되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환
func (or *OrdersRouter) sellOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	createSellOrderRequest := template.OrderRequest{}

	if err := c.BodyParser(&createSellOrderRequest); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 서버 측에서 설정
	createSellOrderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	createSellOrderRequest.OrderID = uuid.NewString()
	createSellOrderRequest.Timestamp = c.Context().Time().UnixMilli()
	createSellOrderRequest.Symbol = symbol
	createSellOrderRequest.Side = SideSell
	createSellOrderRequest.Status = StatusOpen

	// 입력 검증
	if createSellOrderRequest.Quantity <= 0 {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Quantity must be greater than zero")
	}
	if createSellOrderRequest.OrderType != OrderTypeMarket && createSellOrderRequest.OrderType != OrderTypeLimit && createSellOrderRequest.OrderType != OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid order type")
	}

	// 주문 처리
	depth := ws.TempDepth[symbol]
	if depth.TotalAsks == nil {
		depth.TotalAsks = make(map[float64]int)
	}
	if depth.Asks == nil {
		depth.Asks = make(map[float64][]template.Order)
	}

	if createSellOrderRequest.OrderType == OrderTypeLimit {
		if createSellOrderRequest.Price <= 0 {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Price must be greater than zero for limit orders")
		}

		depth.TotalAsks[createSellOrderRequest.Price] += createSellOrderRequest.Quantity
		depth.Asks[createSellOrderRequest.Price] = append(depth.Asks[createSellOrderRequest.Price], template.Order{
			UserID:   createSellOrderRequest.UserID,
			OrderID:  createSellOrderRequest.OrderID,
			Quantity: createSellOrderRequest.Quantity,
		})
	}
	if createSellOrderRequest.OrderType == OrderTypeMarket {
		createSellOrderRequest.Price = 0 // 시장가 주문의 경우 가격은 무시됨

		depth.TotalAsks[createSellOrderRequest.Price] += createSellOrderRequest.Quantity
		depth.Asks[createSellOrderRequest.Price] = append(depth.Asks[createSellOrderRequest.Price], template.Order{
			UserID:   createSellOrderRequest.UserID,
			OrderID:  createSellOrderRequest.OrderID,
			Quantity: createSellOrderRequest.Quantity,
		})
	}
	if createSellOrderRequest.OrderType == OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Not implemented: Stop-Limit order")
	}

	ws.TempDepth[symbol] = depth

	// 호가 갱신 브로드캐스트
	update := template.UpdateDepth{
		Timestamp: createSellOrderRequest.Timestamp,
		Symbol:    symbol,
		Side:      "asks",
		Price:     createSellOrderRequest.Price,
		Quantity:  depth.TotalAsks[createSellOrderRequest.Price],
	}
	newDepth, _ := json2.Marshal(update)
	ws.DepthHub.BroadcastMessage(createSellOrderRequest.Timestamp, websocket.TextMessage, newDepth)

	// 주문 접수 알림
	orderCheck, _ := json2.Marshal(createSellOrderRequest)
	ws.NotifyHub.SendMessageToUser(createSellOrderRequest.UserID, createSellOrderRequest.Timestamp, websocket.TextMessage, orderCheck)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Sell order placed successfully",
		"orderID": createSellOrderRequest.OrderID,
	})
}

// @Summary 매도 주문 수정
// @Description 기존 매도 주문을 수정합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.ModifyOrderRequest	true	"수정할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 수정되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/sell [patch]
func (or *OrdersRouter) modifySellOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	modifySellOrderRequest := template.OrderRequest{}

	if err := c.BodyParser(&modifySellOrderRequest); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// 서버 측에서 설정
	modifySellOrderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	modifySellOrderRequest.Timestamp = c.Context().Time().UnixMilli()
	modifySellOrderRequest.Symbol = symbol
	modifySellOrderRequest.Side = SideSell
	modifySellOrderRequest.Status = StatusModified

	depthIndex := make([]interface{}, 2)

	// 입력 검증
	if modifySellOrderRequest.OrderID == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID is required")
	} else {
		// 존재하는 주문인지 확인 TODO 가격별로 확인하는 것은 비효율적이므로, 추후 주문ID로 바로 조회할 수 있도록 개선 필요
		exist := false
		for i, orders := range ws.TempDepth[symbol].Asks {
			for j, order := range orders {
				if order.OrderID == modifySellOrderRequest.OrderID && order.UserID == modifySellOrderRequest.UserID {
					depthIndex[0] = i
					depthIndex[1] = j
					exist = true
					break
				}
			}
		}
		if !exist {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID does not exist")
		}
	}
	if modifySellOrderRequest.Quantity <= 0 {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Quantity must be greater than zero")
	}
	if modifySellOrderRequest.OrderType != OrderTypeMarket && modifySellOrderRequest.OrderType != OrderTypeLimit && modifySellOrderRequest.OrderType != OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid order type")
	}

	// 주문 수정 처리
	depth := ws.TempDepth[symbol]
	// 우선 기존 주문 제거
	price := depthIndex[0].(float64)
	index := depthIndex[1].(int)
	depth.TotalAsks[price] -= depth.Asks[price][index].Quantity
	depth.Asks[price] = append(depth.Asks[price][:index], depth.Asks[price][index+1:]...)

	if price != modifySellOrderRequest.Price {
		// 호가 갱신 브로드캐스트
		update := template.UpdateDepth{
			Timestamp: modifySellOrderRequest.Timestamp,
			Symbol:    symbol,
			Side:      "asks",
			Price:     price,
			Quantity:  depth.TotalAsks[price],
		}
		newDepth, _ := json2.Marshal(update)
		ws.DepthHub.BroadcastMessage(modifySellOrderRequest.Timestamp, websocket.TextMessage, newDepth)
	}

	if modifySellOrderRequest.OrderType == OrderTypeLimit {
		if modifySellOrderRequest.Price <= 0 {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "Price must be greater than zero for limit orders")
		}

		// 새로운 주문 추가
		depth.TotalAsks[modifySellOrderRequest.Price] += modifySellOrderRequest.Quantity
		depth.Asks[modifySellOrderRequest.Price] = append(depth.Asks[modifySellOrderRequest.Price], template.Order{
			UserID:   modifySellOrderRequest.UserID,
			OrderID:  modifySellOrderRequest.OrderID,
			Quantity: modifySellOrderRequest.Quantity,
		})
	}
	if modifySellOrderRequest.OrderType == OrderTypeMarket {
		modifySellOrderRequest.Price = 0 // 시장가 주문의 경우 가격은 무시됨

		// 새로운 주문 추가
		depth.TotalAsks[modifySellOrderRequest.Price] += modifySellOrderRequest.Quantity
		depth.Asks[modifySellOrderRequest.Price] = append(depth.Asks[modifySellOrderRequest.Price], template.Order{
			UserID:   modifySellOrderRequest.UserID,
			OrderID:  modifySellOrderRequest.OrderID,
			Quantity: modifySellOrderRequest.Quantity,
		})
	}
	if modifySellOrderRequest.OrderType == OrderTypeStopLimit {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Not implemented: Stop-Limit order modification")
	}

	ws.TempDepth[symbol] = depth

	// 호가 갱신 브로드캐스트
	update := template.UpdateDepth{
		Timestamp: modifySellOrderRequest.Timestamp,
		Symbol:    symbol,
		Side:      "asks",
		Price:     modifySellOrderRequest.Price,
		Quantity:  depth.TotalAsks[modifySellOrderRequest.Price],
	}
	newDepth, _ := json2.Marshal(update)
	ws.DepthHub.BroadcastMessage(modifySellOrderRequest.Timestamp, websocket.TextMessage, newDepth)

	// 주문 수정 알림
	orderCheck, _ := json2.Marshal(modifySellOrderRequest)
	ws.NotifyHub.SendMessageToUser(modifySellOrderRequest.UserID, modifySellOrderRequest.Timestamp, websocket.TextMessage, orderCheck)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Modify sell order successfully",
		"orderID": modifySellOrderRequest.OrderID,
	})
}

// @Summary 매도 주문 취소
// @Description 기존 매도 주문을 취소합니다.
// @Tags Orders
// @Accept json
// @Produce json
// @Param			symbol			path		string				true	"심볼 (예: AAPL)"
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @Param			order			body		template.CancelOrderRequest	true	"취소할 주문 정보"
// @Success		200				{object}	map[string]string	"주문이 성공적으로 취소되었음을 알리는 메시지"
// @Failure		400				{object}	map[string]string	"잘못된 요청 시 에러 메시지 반환"
// @Failure		401				{object}	map[string]string	"인증 실패 시 에러 메시지 반환"
// @Failure		404				{object}	map[string]string	"심볼을 찾을 수 없을 때 에러 메시지 반환"
// @Failure		500				{object}	map[string]string	"서버 오류 발생 시 에러 메시지 반환"
// @Router			/api/v1/market/orders/{symbol}/sell [delete]
func (or *OrdersRouter) cancelSellOrder(c *fiber.Ctx) error {
	symbol := c.Params("sym")
	user := c.Locals("user").(*postgresql.User)
	cancelSellOrderRequest := template.OrderRequest{}

	if err := c.BodyParser(&cancelSellOrderRequest); err != nil {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "Invalid request body")
	}

	cancelSellOrderRequest.UserID = c.Locals("user").(*postgresql.User).ID
	cancelSellOrderRequest.Timestamp = c.Context().Time().UnixMilli()
	cancelSellOrderRequest.Symbol = symbol
	cancelSellOrderRequest.Side = SideSell
	cancelSellOrderRequest.Status = StatusCanceled

	depthIndex := make([]interface{}, 2)

	// 입력 검증
	if cancelSellOrderRequest.OrderID == "" {
		return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID is required")
	} else {
		// 존재하는 주문인지 확인 TODO 가격별로 확인하는 것은 비효율적이므로, 추후 주문ID로 바로 조회할 수 있도록 개선 필요
		exist := false
		for i, orders := range ws.TempDepth[symbol].Asks {
			for j, order := range orders {
				if order.OrderID == cancelSellOrderRequest.OrderID && order.UserID == user.ID {
					depthIndex[0] = i
					depthIndex[1] = j
					exist = true
					break
				}
			}
		}
		if !exist {
			return template.ErrorHandler(c, fiber.StatusBadRequest, "OrderID does not exist")
		}
	}

	// 주문 취소 처리
	depth := ws.TempDepth[symbol]
	price := depthIndex[0].(float64)
	index := depthIndex[1].(int)
	depth.TotalAsks[price] -= depth.Asks[price][index].Quantity
	depth.Asks[price] = append(depth.Asks[price][:index], depth.Asks[price][index+1:]...)
	ws.TempDepth[symbol] = depth

	// 호가 갱신 브로드캐스트
	update := template.UpdateDepth{
		Timestamp: cancelSellOrderRequest.Timestamp,
		Symbol:    symbol,
		Side:      "asks",
		Price:     price,
		Quantity:  depth.TotalAsks[price],
	}
	newDepth, _ := json2.Marshal(update)
	ws.DepthHub.BroadcastMessage(cancelSellOrderRequest.Timestamp, websocket.TextMessage, newDepth)

	// 주문 취소 알림
	orderCheck, _ := json2.Marshal(cancelSellOrderRequest)
	ws.NotifyHub.SendMessageToUser(cancelSellOrderRequest.UserID, cancelSellOrderRequest.Timestamp, websocket.TextMessage, orderCheck)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Cancel sell order successfully",
		"orderID": cancelSellOrderRequest.OrderID,
	})
}
