package channels

import (
	"PJS_Exchange/routes/ws"
	t "PJS_Exchange/template"
	"PJS_Exchange/utils"
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
	depthPriceOrder := ws.TempDepthPriceOrder[orderReq.Symbol]
	bidAskOverLabCheck := ws.TempBidAskOverlapCheck[orderReq.Symbol]

	// depth | Asks, Bids, TotalAsks, TotalBids, TopBid, BottomAsk가 nil이면 초기화
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
	if depth.TopBid == nil {
		depth.TopBid = btree.New(4)
	}
	if depth.BottomAsk == nil {
		depth.BottomAsk = btree.New(4)
	}

	// depthOrderIDIndex | nil이면 초기화
	if depthOrderIDIndex == nil { // (OrderID: [userID, side, price, quantity])
		depthOrderIDIndex = make(map[string][]interface{})
	}

	// depthPriceOrder | nil이면 초기화
	if depthPriceOrder == nil { // (price: {"bids": [orderID1, orderID2], "asks": [orderID3, orderID4]}) FIFO
		depthPriceOrder = make(map[string]map[float64]*utils.Queue)
		depthPriceOrder["bids"] = make(map[float64]*utils.Queue)
		depthPriceOrder["asks"] = make(map[float64]*utils.Queue)
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
			processOpen(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

			timestamp = time.Now().UnixMilli()
			// 호가 갱신 브로드캐스트
			depthUpdate := t.UpdateDepth{
				Timestamp: timestamp,
				Symbol:    orderReq.Symbol,
				Side:      "bids",
				Price:     orderReq.Price,
				Quantity:  depth.TotalBids[orderReq.Price],
			}
			newDepth, _ := json.Marshal(depthUpdate)
			ws.DepthHub.BroadcastMessage(timestamp, websocket.TextMessage, newDepth)

			// 주문 접수 알림
			newOrderRequest, _ := json.Marshal(orderReq)
			ws.NotifyHub.SendMessageToUser(orderReq.UserID, timestamp, websocket.TextMessage, newOrderRequest)
		case t.StatusModified:
			// 주문 수정 처리 로직
			processModify(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

			timestamp = time.Now().UnixMilli()
			// 호가 갱신 브로드캐스트
			depthUpdate := t.UpdateDepth{
				Timestamp: timestamp,
				Symbol:    orderReq.Symbol,
				Side:      "bids",
				Price:     orderReq.Price,
				Quantity:  depth.TotalBids[orderReq.Price],
			}
			newDepth, _ := json.Marshal(depthUpdate)
			ws.DepthHub.BroadcastMessage(timestamp, websocket.TextMessage, newDepth)

			// 주문 수정 알림
			newOrderRequest, _ := json.Marshal(orderReq)
			ws.NotifyHub.SendMessageToUser(orderReq.UserID, timestamp, websocket.TextMessage, newOrderRequest)
		case t.StatusCanceled:
			// 주문 취소 처리 로직
			processCancel(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

			timestamp = time.Now().UnixMilli()
			// 호가 갱신 브로드캐스트
			depthUpdate := t.UpdateDepth{
				Timestamp: timestamp,
				Symbol:    orderReq.Symbol,
				Side:      "bids",
				Price:     orderReq.Price,
				Quantity:  depth.TotalBids[orderReq.Price],
			}
			newDepth, _ := json.Marshal(depthUpdate)
			ws.DepthHub.BroadcastMessage(timestamp, websocket.TextMessage, newDepth)

			// 주문 취소 알림
			newOrderRequest, _ := json.Marshal(orderReq)
			ws.NotifyHub.SendMessageToUser(orderReq.UserID, timestamp, websocket.TextMessage, newOrderRequest)
		}
	case t.SideSell:
		//log.Printf("Processing Sell Order: %+v", orderReq)
		// 매도 주문 처리 로직 구현
		switch orderReq.Status {
		case t.StatusOpen:
			// 신규 주문 처리 로직
			processOpen(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

			timestamp = time.Now().UnixMilli()
			// 호가 갱신 브로드캐스트
			depthUpdate := t.UpdateDepth{
				Timestamp: timestamp,
				Symbol:    orderReq.Symbol,
				Side:      "asks",
				Price:     orderReq.Price,
				Quantity:  depth.TotalAsks[orderReq.Price],
			}
			newDepth, _ := json.Marshal(depthUpdate)
			ws.DepthHub.BroadcastMessage(timestamp, websocket.TextMessage, newDepth)

			// 주문 접수 알림
			newOrderRequest, _ := json.Marshal(orderReq)
			ws.NotifyHub.SendMessageToUser(orderReq.UserID, timestamp, websocket.TextMessage, newOrderRequest)
		case t.StatusModified:
			// 주문 수정 처리 로직
			processModify(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

			timestamp = time.Now().UnixMilli()
			// 호가 갱신 브로드캐스트
			depthUpdate := t.UpdateDepth{
				Timestamp: timestamp,
				Symbol:    orderReq.Symbol,
				Side:      "asks",
				Price:     orderReq.Price,
				Quantity:  depth.TotalAsks[orderReq.Price],
			}
			newDepth, _ := json.Marshal(depthUpdate)
			ws.DepthHub.BroadcastMessage(timestamp, websocket.TextMessage, newDepth)

			// 주문 수정 알림
			newOrderRequest, _ := json.Marshal(orderReq)
			ws.NotifyHub.SendMessageToUser(orderReq.UserID, timestamp, websocket.TextMessage, newOrderRequest)
		case t.StatusCanceled:
			// 주문 취소 처리 로직
			processCancel(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

			timestamp = time.Now().UnixMilli()
			// 호가 갱신 브로드캐스트
			depthUpdate := t.UpdateDepth{
				Timestamp: timestamp,
				Symbol:    orderReq.Symbol,
				Side:      "asks",
				Price:     orderReq.Price,
				Quantity:  depth.TotalAsks[orderReq.Price],
			}
			newDepth, _ := json.Marshal(depthUpdate)
			ws.DepthHub.BroadcastMessage(timestamp, websocket.TextMessage, newDepth)

			// 주문 취소 알림
			newOrderRequest, _ := json.Marshal(orderReq)
			ws.NotifyHub.SendMessageToUser(orderReq.UserID, timestamp, websocket.TextMessage, newOrderRequest)
		}
	}

	// 주문 처리
	processOrder(&orderReq, &depth, &depthOrderIDIndex, bidAskOverLabCheck, &depthPriceOrder)

	// 변경된 데이터 저장
	ws.TempDepth[orderReq.Symbol] = depth
	ws.TempDepthOrderIDIndex[orderReq.Symbol] = depthOrderIDIndex
	ws.TempDepthPriceOrder[orderReq.Symbol] = depthPriceOrder
	ws.TempBidAskOverlapCheck[orderReq.Symbol] = bidAskOverLabCheck
	log.Println("---- Order Processed ----")
	log.Println("Updated Depth:", depth)
	log.Println("Updated DepthOrderIDIndex:", depthOrderIDIndex)
	log.Println("Updated DepthPriceOrder:", depthPriceOrder)
	log.Println("Checking Depth.TopBid:", depth.TopBid)
	log.Println("Checking Depth.BottomAsk:", depth.BottomAsk)
	log.Println("Updated TempBidAskOverlapCheck", bidAskOverLabCheck)
	log.Println("---- Order Processed ----")
	return
}

func processOpen(o *t.OrderRequest, d *t.MarketDepth, di *map[string][]interface{}, bao *btree.BTree, dpo *map[string]map[float64]*utils.Queue) {
	//log.Printf("processOpen called with Order: %+v", o)
	price := o.Price
	order := t.Order{
		UserID:   o.UserID,
		Quantity: o.Quantity,
	}
	side := ""

	// depth에 추가
	switch o.Side {
	case t.SideBuy:
		side = "bid"
		// 호가에 추가
		if d.Bids[price] == nil {
			d.Bids[price] = make(map[string]t.Order)
		}
		d.Bids[price][o.OrderID] = order

		// 전체 주문 수량에 반영
		d.TotalBids[price] += o.Quantity

		// 매수 호가가 가장 높은 경우 갱신
		if maxPrice := d.TopBid.Max(); d.TopBid.Len() == 0 || maxPrice.(t.Float64Item) < t.Float64Item(price) {
			d.TopBid.ReplaceOrInsert(t.Float64Item(price))
		}

		// 매수 호가와 매도 호가가 겹치는 경우 체크
		if _, exists := d.Asks[price]; exists {
			(*bao).ReplaceOrInsert(t.Float64Item(price))
		}

		// 가격대별 주문 순서에 추가
		if (*dpo)["bids"][price] == nil {
			(*dpo)["bids"][price] = utils.NewQueue()
		}
		(*dpo)["bids"][price].Enqueue(o.OrderID)
	case t.SideSell:
		side = "ask"
		// 호가에 추가
		if d.Asks[price] == nil {
			d.Asks[price] = make(map[string]t.Order)
		}
		d.Asks[price][o.OrderID] = order

		// 전체 주문 수량에 반영
		d.TotalAsks[price] += o.Quantity

		// 매도 호가가 가장 낮은 경우 갱신
		if minPrice := d.BottomAsk.Min(); d.BottomAsk.Len() == 0 || minPrice.(t.Float64Item) > t.Float64Item(price) {
			d.BottomAsk.ReplaceOrInsert(t.Float64Item(price))
		}

		// 매수 호가와 매도 호가가 겹치는 경우 체크
		if _, exists := d.Bids[price]; exists {
			(*bao).ReplaceOrInsert(t.Float64Item(price))
		}

		// 가격대별 주문 순서에 추가
		if (*dpo)["asks"][price] == nil {
			(*dpo)["asks"][price] = utils.NewQueue()
		}
		(*dpo)["asks"][price].Enqueue(o.OrderID)
	}

	// OrderID 인덱스 맵에 추가
	(*di)[o.OrderID] = make([]interface{}, 4)
	(*di)[o.OrderID][0] = o.UserID
	(*di)[o.OrderID][1] = side
	(*di)[o.OrderID][2] = price
	(*di)[o.OrderID][3] = o.Quantity
	//log.Printf("Order %s added to depth at price %.2f with quantity %d", o.OrderID, price, o.Quantity)
}

func processModify(o *t.OrderRequest, d *t.MarketDepth, di *map[string][]interface{}, bao *btree.BTree, dpo *map[string]map[float64]*utils.Queue) {
	//log.Printf("processModify called with Order: %+v", o)
	previousQuantity := (*di)[o.OrderID][3].(int)

	if o.Quantity < previousQuantity && o.Price == (*di)[o.OrderID][2].(float64) {
		// 가격을 유지하고 수량을 줄이는 경우는 우선순위 유지
		price := (*di)[o.OrderID][2].(float64)
		switch o.Side {
		case t.SideBuy:
			if _, ok := d.Bids[price]; ok {
				d.Bids[price][o.OrderID] = t.Order{
					UserID:   o.UserID,
					Quantity: o.Quantity,
				}

				d.TotalBids[price] -= previousQuantity
				d.TotalBids[price] += o.Quantity
			}
		case t.SideSell:
			if _, ok := d.Asks[price]; ok {
				d.Asks[price][o.OrderID] = t.Order{
					UserID:   o.UserID,
					Quantity: o.Quantity,
				}

				d.TotalAsks[price] -= previousQuantity
				d.TotalAsks[price] += o.Quantity
			}
		}

		// 인덱스 맵에 수량만 변경
		(*di)[o.OrderID][3] = o.Quantity
	} else {
		// 단일 주문에서 수량을 늘리거나 가격(+시장가, 지정가 변경)을 변경하는 경우는 우선순위 재조정
		processCancel(o, d, di, bao, dpo)
		processOpen(o, d, di, bao, dpo)
		return
	}
	//log.Printf("Order %s modified in depth to quantity %d", o.OrderID, o.Quantity)
}

func processCancel(o *t.OrderRequest, d *t.MarketDepth, di *map[string][]interface{}, bao *btree.BTree, dpo *map[string]map[float64]*utils.Queue) {
	//log.Printf("processCancel called with Order: %+v", o)
	price := (*di)[o.OrderID][2].(float64)

	// depth에서 삭제
	switch o.Side {
	case t.SideBuy:
		if _, ok := d.Bids[price]; ok {
			// 호가 조정
			delete(d.Bids[price], o.OrderID)
			// 더이상 호가에 값이 없는 경우 삭제
			if len(d.Bids[price]) == 0 {
				delete(d.Bids, price)
			}

			// 전체 주문 수량에서 반영
			d.TotalBids[price] -= (*di)[o.OrderID][3].(int)
			if d.TotalBids[price] <= 0 {
				delete(d.TotalBids, price)
			}

			// 내 호가에 나만 남아있는 경우 체크
			if _, exists := d.Bids[price]; !exists {
				// TopBid에서 삭제
				d.TopBid.Delete(t.Float64Item(price))

				// 매수 호가와 매도 호가가 더이상 겹치지 않음
				bao.Delete(t.Float64Item(price))
			}

			// 가격대별 주문 순서에서 삭제
			(*dpo)["bids"][price].RemoveValue(o.OrderID)
		}
	case t.SideSell:
		if _, ok := d.Asks[price]; ok {
			// 호가 조정
			delete(d.Asks[price], o.OrderID)
			// 더이상 호가에 값이 없는 경우 삭제
			if len(d.Asks[price]) == 0 {
				delete(d.Asks, price)
			}

			// 전체 주문 수량에서 반영
			d.TotalAsks[price] -= (*di)[o.OrderID][3].(int)
			if d.TotalAsks[price] <= 0 {
				delete(d.TotalAsks, price)
			}

			// 내 호가에 나만 남아있는 경우 체크
			if _, exists := d.Asks[price]; !exists {
				// BottomAsk에서 삭제
				d.BottomAsk.Delete(t.Float64Item(price))

				// 매수 호가와 매도 호가가 더이상 겹치지 않음
				bao.Delete(t.Float64Item(price))
			}

			// 가격대별 주문 순서에서 삭제
			(*dpo)["asks"][price].RemoveValue(o.OrderID)
		}
	}

	// 호가 인덱스 조정
	delete(*di, o.OrderID)
	//log.Printf("Order %s canceled and removed from depth", o.OrderID)
}

func processOrder(o *t.OrderRequest, d *t.MarketDepth, di *map[string][]interface{}, bao *btree.BTree, dpo *map[string]map[float64]*utils.Queue) {
	log.Printf("processOrder called with Order: %+v", o)
	// 체결 우선순위
	// 1.가격
	// 2.시간
	// 3.수량
	log.Printf("Order processing completed")
}
