package channels

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/routes/ws"
	t "PJS_Exchange/template"
	"PJS_Exchange/utils"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/websocket/v2"
	"github.com/google/btree"
)

var (
	OP *ProcessOrders
)

type ProcessOrders struct {
	OrderRequestChan chan t.OrderRequest
	Running          bool
}

func NewProcessOrders() *ProcessOrders {
	return &ProcessOrders{
		OrderRequestChan: make(chan t.OrderRequest, 100),
		Running:          false,
	}
}

func (po *ProcessOrders) Create() {
	po.Running = true

	for po.Running {
		select {
		case orderReq := <-po.OrderRequestChan:
			po.processOrderRequest(orderReq)
		}
	}
}

func (po *ProcessOrders) Destroy() {
	po.Running = false
	close(po.OrderRequestChan)
}

func (po *ProcessOrders) processOrderRequest(orderReq t.OrderRequest) {
	timestamp := time.Now().UnixMilli()
	depth := ws.TempDepth[orderReq.Symbol]
	depthOrderIDIndex := ws.TempDepthOrderIDIndex[orderReq.Symbol]
	depthExecutionSeq := ws.TempDepthExecutionSeq[orderReq.Symbol]
	bidAskOverLabCheck := ws.TempBidAskOverlapCheck[orderReq.Symbol]

	// depth | Asks, Bids, TotalAsks, TotalBids, BidTree, BottomAsk가 nil이면 초기화
	if depth.Asks == nil {
		depth.Asks = make(map[float64]map[string]t.Order)
	}
	if depth.Bids == nil {
		depth.Bids = make(map[float64]map[string]t.Order)
	}
	if depth.TotalAsks == nil {
		depth.TotalAsks = make(map[float64]int)
	}
	if depth.TotalBids == nil {
		depth.TotalBids = make(map[float64]int)
	}
	if depth.BidTree == nil {
		depth.BidTree = btree.New(4)
	}
	if depth.AskTree == nil {
		depth.AskTree = btree.New(4)
	}

	// depthOrderIDIndex | nil이면 초기화
	if depthOrderIDIndex == nil { // (OrderID: [userID, side, price, quantity])
		depthOrderIDIndex = make(map[string][]interface{})
	}

	// depthExecutionSeq | nil이면 초기화
	if depthExecutionSeq == nil { // (price: {"bids": [orderID1, orderID2], "asks": [orderID3, orderID4]}) FIFO
		depthExecutionSeq = make(map[string]map[float64]*utils.Queue)
		depthExecutionSeq["bids"] = make(map[float64]*utils.Queue)
		depthExecutionSeq["asks"] = make(map[float64]*utils.Queue)
	}

	// bidAskOverLabCheck | nil이면 초기화
	if bidAskOverLabCheck == nil { //
		bidAskOverLabCheck = btree.New(4)
	}

	//입력 검증 | OrderID, OrderType, Price, Quantity
	if orderReq.Status != t.StatusOpen && (orderReq.OrderID == "" || depthOrderIDIndex[orderReq.OrderID] == nil) { // OrderID는 빈값이거나 존재하지 않는 ID (수정, 취소시)
		orderReq.ResultChan <- t.Result{
			Timestamp: timestamp,
			Success:   false,
			Message:   "Invalid OrderID",
			Code:      400,
		}
		return
	}
	if orderReq.Status != t.StatusOpen && depthOrderIDIndex[orderReq.OrderID][0].(int) != orderReq.UserID { // UserID와 OrderID가 매칭되지 않는 경우 (수정, 취소시)
		orderReq.ResultChan <- t.Result{
			Timestamp: timestamp,
			Success:   false,
			Message:   "OrderID does not match UserID",
			Code:      400,
		}
		return
	}
	if (orderReq.Status == t.StatusOpen &&
		(orderReq.OrderType != t.OrderTypeLimit && orderReq.OrderType != t.OrderTypeMarket)) ||
		(orderReq.Status != t.StatusOpen &&
			orderReq.OrderType != "" &&
			orderReq.OrderType != t.OrderTypeLimit &&
			orderReq.OrderType != t.OrderTypeMarket) { // limit, market 가능, 수정, 취소는 추가로 "" 가능 (수정의 경우는 ""이면 기존 타입 유지)
		orderReq.ResultChan <- t.Result{
			Timestamp: timestamp,
			Success:   false,
			Message:   "Invalid OrderType",
			Code:      400,
		}
		return
	}
	if orderReq.Price <= 0 && orderReq.OrderType != t.OrderTypeMarket && orderReq.Status != t.StatusCanceled { // 시장가 주문과 취소는 가격 무시
		orderReq.ResultChan <- t.Result{
			Timestamp: timestamp,
			Success:   false,
			Message:   "Invalid Price",
			Code:      400,
		}
		return
	}
	if orderReq.Quantity <= 0 && orderReq.Status != t.StatusCanceled { // 취소는 수량 무시
		orderReq.ResultChan <- t.Result{
			Timestamp: timestamp,
			Success:   false,
			Message:   "Invalid Quantity",
			Code:      400,
		}
		return
	}

	// 주문 수정시 OrderType이 빈값이면 기존 타입 유지
	if (orderReq.Status == t.StatusModified || orderReq.Status == t.StatusCanceled) && orderReq.OrderType == "" {
		previousPrice := depthOrderIDIndex[orderReq.OrderID][2].(float64)
		if previousPrice == 0 {
			orderReq.OrderType = t.OrderTypeMarket
		} else {
			orderReq.OrderType = t.OrderTypeLimit
		}
	}

	// 시장가 주문은 가격을 0으로 설정
	if orderReq.OrderType == t.OrderTypeMarket {
		orderReq.Price = 0
	}

	// 취소 주문은 이전 가격과 수량 유지
	if orderReq.Status == t.StatusCanceled {
		orderReq.Price = depthOrderIDIndex[orderReq.OrderID][2].(float64)
		orderReq.Quantity = depthOrderIDIndex[orderReq.OrderID][3].(int)
	}

	// 주문을 변경할때 가격, 수량 모두 변화가 없으면 무시
	if orderReq.Status == t.StatusModified &&
		orderReq.Price == depthOrderIDIndex[orderReq.OrderID][2].(float64) &&
		orderReq.Quantity == depthOrderIDIndex[orderReq.OrderID][3].(int) {
		orderReq.ResultChan <- t.Result{
			Timestamp: timestamp,
			Success:   false,
			Message:   "No changes in Price or Quantity",
			Code:      400,
		}
		return
	}

	orderReq.ResultChan <- t.Result{
		Timestamp: timestamp,
		Success:   true,
		Message:   "Order processed successfully",
		Code:      200,
	}

	// 주문 등록
	switch orderReq.Side {
	case t.SideBuy:
		//log.Printf("Processing Buy Order: %+v", orderReq)
		// 매수 주문 처리 로직 구현
		switch orderReq.Status {
		case t.StatusOpen:
			// 신규 주문 처리 로직
			processOpen(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)
		case t.StatusModified:
			// 주문 수정 처리 로직
			processModify(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)
		case t.StatusCanceled:
			// 주문 취소 처리 로직
			processCancel(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)
		}

		timestamp = time.Now().UnixMilli()
		orderReq.Timestamp = timestamp
		updateDepth(t.UpdateDepth{
			Timestamp: timestamp,
			Symbol:    orderReq.Symbol,
			Side:      t.Bids,
			Price:     orderReq.Price,
			Quantity:  depth.TotalBids[orderReq.Price],
		})
		notifyUser(orderReq)
	case t.SideSell:
		//log.Printf("Processing Sell Order: %+v", orderReq)
		// 매도 주문 처리 로직 구현
		switch orderReq.Status {
		case t.StatusOpen:
			// 신규 주문 처리 로직
			processOpen(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)
		case t.StatusModified:
			// 주문 수정 처리 로직
			processModify(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)
		case t.StatusCanceled:
			// 주문 취소 처리 로직
			processCancel(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)
		}

		timestamp = time.Now().UnixMilli()
		orderReq.Timestamp = timestamp
		updateDepth(t.UpdateDepth{
			Timestamp: timestamp,
			Symbol:    orderReq.Symbol,
			Side:      t.Asks,
			Price:     orderReq.Price,
			Quantity:  depth.TotalAsks[orderReq.Price],
		})
		notifyUser(orderReq)
	}

	// 주문 체결
	processOrder(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthExecutionSeq)

	//log.Println("---- Order Processed ----")
	//log.Println("Updated Depth:", depth)
	//log.Println("Updated DepthOrderIDIndex:", depthOrderIDIndex)
	//log.Println("Updated DepthPriceOrder:", depthExecutionSeq)
	//log.Println("Checking Depth.BidTree:", depth.BidTree)
	//log.Println("Checking Depth.AskTree:", depth.AskTree)
	//log.Println("Updated TempBidAskOverlapCheck", bidAskOverLabCheck)
	//log.Println("---- Order Processed ----")
	return
}

func processOpen(orderReq *t.OrderRequest, depth *t.MarketDepth, depthIndex *map[string][]interface{}, bidAskOverLab *btree.BTree, executionSeq *map[string]map[float64]*utils.Queue) {
	//log.Printf("processOpen called with Order: %+v", orderReq)
	price := orderReq.Price
	order := t.Order{
		UserID:   orderReq.UserID,
		Quantity: orderReq.Quantity,
	}
	side := ""

	// depth에 추가
	switch orderReq.Side {
	case t.SideBuy:
		side = t.Bids
		// 호가에 추가
		if depth.Bids[price] == nil {
			depth.Bids[price] = make(map[string]t.Order)
		}
		depth.Bids[price][orderReq.OrderID] = order

		// 전체 주문 수량에 반영
		depth.TotalBids[price] += orderReq.Quantity

		// 매수 호가 트리에 추가
		depth.BidTree.ReplaceOrInsert(t.Float64Item(price))

		// 매수 호가와 매도 호가가 겹치거나 시장가 주문인 경우 체크
		if _, exists := depth.Asks[price]; exists {
			(*bidAskOverLab).ReplaceOrInsert(t.Float64Item(price))
		}

		// 가격대별 주문 순서에 추가
		if (*executionSeq)[t.Bids][price] == nil {
			(*executionSeq)[t.Bids][price] = utils.NewQueue()
		}
		(*executionSeq)[t.Bids][price].Enqueue(orderReq.OrderID)
	case t.SideSell:
		side = t.Asks
		// 호가에 추가
		if depth.Asks[price] == nil {
			depth.Asks[price] = make(map[string]t.Order)
		}
		depth.Asks[price][orderReq.OrderID] = order

		// 전체 주문 수량에 반영
		depth.TotalAsks[price] += orderReq.Quantity

		// 매도 호가 트리에 추가
		depth.AskTree.ReplaceOrInsert(t.Float64Item(price))

		// 매수 호가와 매도 호가가 겹치거나 시장가 주문인 경우 체크
		if _, exists := depth.Bids[price]; exists {
			(*bidAskOverLab).ReplaceOrInsert(t.Float64Item(price))
		}

		// 가격대별 주문 순서에 추가
		if (*executionSeq)[t.Asks][price] == nil {
			(*executionSeq)[t.Asks][price] = utils.NewQueue()
		}
		(*executionSeq)[t.Asks][price].Enqueue(orderReq.OrderID)
	}

	// OrderID 인덱스 맵에 추가
	(*depthIndex)[orderReq.OrderID] = make([]interface{}, 4)
	(*depthIndex)[orderReq.OrderID][0] = orderReq.UserID
	(*depthIndex)[orderReq.OrderID][1] = side
	(*depthIndex)[orderReq.OrderID][2] = price
	(*depthIndex)[orderReq.OrderID][3] = orderReq.Quantity
	//log.Printf("Order %s added to depth at price %.2f with quantity %depth", orderReq.OrderID, price, orderReq.Quantity)
}

func processModify(orderRequest *t.OrderRequest, depth *t.MarketDepth, depthIndex *map[string][]interface{}, bidAskOverLab *btree.BTree, executionSeq *map[string]map[float64]*utils.Queue) {
	//log.Printf("processModify called with Order: %+v", orderRequest)
	previousQuantity := (*depthIndex)[orderRequest.OrderID][3].(int)

	if orderRequest.Quantity < previousQuantity && orderRequest.Price == (*depthIndex)[orderRequest.OrderID][2].(float64) {
		// 가격을 유지하고 수량을 줄이는 경우는 우선순위 유지
		price := (*depthIndex)[orderRequest.OrderID][2].(float64)
		switch orderRequest.Side {
		case t.SideBuy:
			if _, ok := depth.Bids[price]; ok {
				depth.Bids[price][orderRequest.OrderID] = t.Order{
					UserID:   orderRequest.UserID,
					Quantity: orderRequest.Quantity,
				}

				depth.TotalBids[price] -= previousQuantity
				depth.TotalBids[price] += orderRequest.Quantity
			}
		case t.SideSell:
			if _, ok := depth.Asks[price]; ok {
				depth.Asks[price][orderRequest.OrderID] = t.Order{
					UserID:   orderRequest.UserID,
					Quantity: orderRequest.Quantity,
				}

				depth.TotalAsks[price] -= previousQuantity
				depth.TotalAsks[price] += orderRequest.Quantity
			}
		}

		// 인덱스 맵에 수량만 변경
		(*depthIndex)[orderRequest.OrderID][3] = orderRequest.Quantity
	} else {
		// 단일 주문에서 수량을 늘리거나 가격(+시장가, 지정가 변경)을 변경하는 경우는 우선순위 재조정
		processCancel(orderRequest, depth, depthIndex, bidAskOverLab, executionSeq)
		processOpen(orderRequest, depth, depthIndex, bidAskOverLab, executionSeq)
		return
	}
	//log.Printf("Order %s modified in depth to quantity %depth", orderRequest.OrderID, orderRequest.Quantity)
}

func processCancel(orderReq *t.OrderRequest, depth *t.MarketDepth, depthIndex *map[string][]interface{}, bidAskOverLab *btree.BTree, executionSeq *map[string]map[float64]*utils.Queue) {
	//log.Printf("processCancel called with Order: %+v", orderReq)
	price := (*depthIndex)[orderReq.OrderID][2].(float64)

	// depth에서 삭제
	switch orderReq.Side {
	case t.SideBuy:
		if _, ok := depth.Bids[price]; ok {
			// 호가 조정
			delete(depth.Bids[price], orderReq.OrderID)
			// 더이상 호가에 값이 없는 경우 삭제
			if len(depth.Bids[price]) == 0 {
				delete(depth.Bids, price)
			}

			// 전체 주문 수량에서 반영
			depth.TotalBids[price] -= (*depthIndex)[orderReq.OrderID][3].(int)
			if depth.TotalBids[price] <= 0 {
				delete(depth.TotalBids, price)
			}

			// 내 호가에 나만 남아있는 경우 체크
			if _, exists := depth.Bids[price]; !exists {
				// TopBid에서 삭제
				depth.BidTree.Delete(t.Float64Item(price))

				// 매수 호가와 매도 호가가 더이상 겹치지 않음
				bidAskOverLab.Delete(t.Float64Item(price))
			}

			// 가격대별 주문 순서에서 삭제
			if (*executionSeq)[t.Bids] != nil && (*executionSeq)[t.Bids][price] != nil {
				(*executionSeq)[t.Bids][price].RemoveValue(orderReq.OrderID)
			}
		}
	case t.SideSell:
		if _, ok := depth.Asks[price]; ok {
			// 호가 조정
			delete(depth.Asks[price], orderReq.OrderID)
			// 더이상 호가에 값이 없는 경우 삭제
			if len(depth.Asks[price]) == 0 {
				delete(depth.Asks, price)
			}

			// 전체 주문 수량에서 반영
			depth.TotalAsks[price] -= (*depthIndex)[orderReq.OrderID][3].(int)
			if depth.TotalAsks[price] <= 0 {
				delete(depth.TotalAsks, price)
			}

			// 내 호가에 나만 남아있는 경우 체크
			if _, exists := depth.Asks[price]; !exists {
				// BottomAsk에서 삭제
				depth.AskTree.Delete(t.Float64Item(price))

				// 매수 호가와 매도 호가가 더이상 겹치지 않음
				bidAskOverLab.Delete(t.Float64Item(price))
			}

			// 가격대별 주문 순서에서 삭제
			if (*executionSeq)[t.Asks] != nil && (*executionSeq)[t.Asks][price] != nil {
				(*executionSeq)[t.Asks][price].RemoveValue(orderReq.OrderID)
			}
		}
	}

	// 호가 인덱스 조정
	delete(*depthIndex, orderReq.OrderID)
	//log.Printf("Order %s canceled and removed from depth", orderReq.OrderID)
}

func processOrder(orderReq *t.OrderRequest, depth *t.MarketDepth, depthIndex *map[string][]interface{}, bidAskOverLab *btree.BTree, executionSeq *map[string]map[float64]*utils.Queue) {
	//log.Printf("processOrder called with Order: %+v", orderReq)

	// 테스트용 코드 TODO 삭제 예정
	if ws.TempLedger[orderReq.Symbol] == nil {
		ws.TempLedger[orderReq.Symbol] = utils.NewQueue()

		ledger := t.Ledger{
			Timestamp: time.Now().UnixMilli(),
			Symbol:    orderReq.Symbol,
			Price:     10000.0,
			Volume:    0,
		}
		send, _ := json.Marshal(ledger)
		ws.TempLedger[orderReq.Symbol].PushFront(ledger)
		ws.LedgerHub.BroadcastMessage(time.Now().UnixMilli(), websocket.TextMessage, send)
	}
	// 테스트용 코드

	// 현재가를 가져와야함 현재가는 가장 최근에 체결된 가격 -> 전일 종가 -> 공모가 순으로 가져옴
	var currentPrice float64
	if ws.TempLedger[orderReq.Symbol] != nil && ws.TempLedger[orderReq.Symbol].Size() != 0 {
		// 가장 최근 체결 가격(상장 직후라면 상장가)
		currentPrice = ws.TempLedger[orderReq.Symbol].GetFront().(t.Ledger).Price
	} else {
		// 전일 종가로 설정

		// 전일 종가도 없으면 공모가로 설정 (단순 Fallback 용 아마 여기까지 올일은 없을듯)
		ctx := context.Background()
		ipoPrice, err := postgresApp.Get().SymbolRepo().GetIPOPrice(ctx, orderReq.Symbol)
		if err != nil {
			log.Printf("Error fetching IPO price for %s: %v", orderReq.Symbol, err)
			return
		} else {
			currentPrice = ipoPrice
		}
	}
	//log.Printf("Current Price for %s: %.2f", orderReq.Symbol, currentPrice)

	if (*bidAskOverLab).Len() == 0 && orderReq.OrderType != t.OrderTypeMarket {
		//log.Printf("No overlapping prices to process orders")
		return
	}

	// 시장가 주문 처리
	if orderReq.OrderType == t.OrderTypeMarket {
		switch orderReq.Side {
		case t.SideBuy:
			// 매수 시장가 주문 처리
			lowAsk := depth.AskTree.Min()
			if lowAsk == nil {
				// 매도 호가가 없으면 주문 취소
				//log.Printf("No asks available to match market buy order")
				processCancel(orderReq, depth, depthIndex, bidAskOverLab, executionSeq)

				timestamp := time.Now().UnixMilli()
				orderReq.Timestamp = timestamp
				orderReq.Status = t.StatusCanceled
				updateDepth(t.UpdateDepth{
					Timestamp: timestamp,
					Symbol:    orderReq.Symbol,
					Side:      t.Bids,
					Price:     orderReq.Price,
					Quantity:  depth.TotalBids[orderReq.Price],
				})
				notifyUser(*orderReq)
				return
			}
			if orderReq.Slippage != nil && len(orderReq.Slippage) == 2 {
				maxSlippagePrice := orderReq.Slippage[0] * (1 + orderReq.Slippage[1]/100)
				if float64(lowAsk.(t.Float64Item)) > maxSlippagePrice {
					// 최대 슬리피지 가격보다 높으면 주문 취소
					//log.Printf("Lowest ask price %.2f exceeds max slippage price %.2f. Cancelling order.", float64(lowAsk.(t.Float64Item)), maxSlippagePrice)
					processCancel(orderReq, depth, depthIndex, bidAskOverLab, executionSeq)

					timestamp := time.Now().UnixMilli()
					orderReq.Timestamp = timestamp
					orderReq.Status = t.StatusCanceled
					updateDepth(t.UpdateDepth{
						Timestamp: timestamp,
						Symbol:    orderReq.Symbol,
						Side:      t.Bids,
						Price:     orderReq.Price,
						Quantity:  depth.TotalBids[orderReq.Price],
					})
					notifyUser(*orderReq)
					return
				}
			}
		case t.SideSell:
			// 매도 시장가 주문 처리
			highBid := depth.BidTree.Max()
			if highBid == nil {
				// 매수 호가가 없으면 주문 취소
				//log.Printf("No bids available to match market sell order")
				processCancel(orderReq, depth, depthIndex, bidAskOverLab, executionSeq)

				timestamp := time.Now().UnixMilli()
				orderReq.Timestamp = timestamp
				orderReq.Status = t.StatusCanceled
				updateDepth(t.UpdateDepth{
					Timestamp: timestamp,
					Symbol:    orderReq.Symbol,
					Side:      t.Asks,
					Price:     orderReq.Price,
					Quantity:  depth.TotalAsks[orderReq.Price],
				})
				notifyUser(*orderReq)
				return
			}
			if orderReq.Slippage != nil && len(orderReq.Slippage) == 2 {
				minSlippagePrice := orderReq.Slippage[0] * (1 - orderReq.Slippage[1]/100)
				if float64(highBid.(t.Float64Item)) < minSlippagePrice {
					// 최소 슬리피지 가격보다 낮으면 주문 취소
					//log.Printf("Highest bid price %.2f is below min slippage price %.2f. Cancelling order.", float64(highBid.(t.Float64Item)), minSlippagePrice)
					processCancel(orderReq, depth, depthIndex, bidAskOverLab, executionSeq)

					timestamp := time.Now().UnixMilli()
					orderReq.Timestamp = timestamp
					orderReq.Status = t.StatusCanceled
					updateDepth(t.UpdateDepth{
						Timestamp: timestamp,
						Symbol:    orderReq.Symbol,
						Side:      t.Asks,
						Price:     orderReq.Price,
						Quantity:  depth.TotalAsks[orderReq.Price],
					})
					notifyUser(*orderReq)
					return
				}
			}
		}

		processMarketOrder(orderReq, depth, depthIndex, bidAskOverLab, executionSeq, currentPrice)
	}

	// 지정가 주문 처리
	if orderReq.OrderType == t.OrderTypeLimit {
		processLimitOrder(orderReq, depth, depthIndex, bidAskOverLab, executionSeq)
	}
	//log.Printf("Order processing completed")
}

func processMarketOrder(orderReq *t.OrderRequest, depth *t.MarketDepth, depthIndex *map[string][]interface{}, bidAskOverLab *btree.BTree, executionSeq *map[string]map[float64]*utils.Queue, currentPrice float64) {
	//log.Printf("processMarketOrder called with Order: %+v", orderReq)
	//executedQuantity := 0
	//remainingQuantity := orderReq.Quantity

	/*
		반대 진영 물량이 내 물량보다 적으면 전부 소진 후 남은 물량은 취소 처리
	*/

	//log.Printf("Market order processing completed")
}

func processLimitOrder(orderReq *t.OrderRequest, depth *t.MarketDepth, depthIndex *map[string][]interface{}, bidAskOverLab *btree.BTree, executionSeq *map[string]map[float64]*utils.Queue) {
	//log.Printf("processLimitOrder called with Order: %+v", orderReq)

	/*
		체결 우선순위
		1.가격
		2.시간
		3.수량
	*/

	switch orderReq.Side {
	case t.SideBuy:
		// 매수 지정가 주문 처리

	case t.SideSell:
		// 매도 지정가 주문 처리
	}
	//log.Printf("Limit order processing completed")
}

func updateDepth(depth t.UpdateDepth) {
	// 호가 갱신 브로드캐스트
	if depth.Timestamp == 0 {
		depth.Timestamp = time.Now().UnixMilli()
	}
	jsonDepth, err := json.Marshal(depth)
	if err != nil {
		log.Printf("Error marshaling UpdateDepth: %v", err)
		return
	}
	ws.DepthHub.BroadcastMessage(depth.Timestamp, websocket.TextMessage, jsonDepth)
}

func notifyUser(notify t.OrderRequest) {
	// 주문 알림
	if notify.Timestamp == 0 {
		notify.Timestamp = time.Now().UnixMilli()
	}
	jsonNotify, err := json.Marshal(notify)
	if err != nil {
		log.Printf("Error marshaling OrderRequest for notification: %v", err)
		return
	}
	ws.NotifyHub.SendMessageToUser(notify.UserID, notify.Timestamp, websocket.TextMessage, jsonNotify)

	// TODO 주문 알림 DB 저장 (비동기)
}

func broadcastTrade(ledger t.Ledger) {
	// 체결 내역 기록
	if ledger.Timestamp == 0 {
		ledger.Timestamp = time.Now().UnixMilli()
	}
	jsonLedger, err := json.Marshal(ledger)
	if err != nil {
		log.Printf("Error marshaling Ledger: %v", err)
		return
	}
	ws.LedgerHub.BroadcastMessage(ledger.Timestamp, websocket.TextMessage, jsonLedger)

	// TODO 체결 원시 데이터 DB 저장 (비동기)
}

func RestoreExchange() {
	// 서버가 장중 다운되었다가 복구 되었을 때 작동하는 함수

	/*
		시장가 주문은 모두 취소 처리
		지정가 주문은 모두 호가에 복구
		만약 매수 호가와 매도 호가가 동시에 존재하는 가격대가 있는경우 체결 처리 하기
	*/
}
