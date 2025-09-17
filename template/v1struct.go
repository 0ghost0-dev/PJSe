package template

import (
	"github.com/google/btree"
)

/* B-Tree */

type Float64Item float64
type StringItem string

func (a Float64Item) Less(b btree.Item) bool {
	return a < b.(Float64Item)
}
func (a StringItem) Less(b btree.Item) bool {
	return a < b.(StringItem)
}

/* Market/Order Types */

var (
	OrderTypeLimit        = "limit"
	OrderTypeMarket       = "market"
	SideBuy               = "buy"
	SideSell              = "sell"
	StatusOpen            = "open"
	StatusModified        = "modified"
	StatusPartiallyFilled = "partially_filled"
	StatusFilled          = "filled"
	StatusCanceled        = "canceled"
	StatusError           = "error"
	Bids                  = "bids"
	Asks                  = "asks"
)

type OrderStatus struct {
	OrderID   string  `json:"order_id"`
	Side      string  `json:"side"` // "buy" or "sell"
	OrderType string  `json:"type"` // e.g., "limit", "market"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type OrderRequest struct {
	Timestamp       int64       `json:"timestamp"` // on Server side, ignore client input
	UserID          int         `json:"user_id"`   // on Server side, ignore client input
	OrderID         string      `json:"order_id"`
	Symbol          string      `json:"symbol"` // on Server side, ignore client input
	Status          string      `json:"status"` // on Server side, ignore client input
	Side            string      `json:"side"`   // on Server side, ignore client input
	OrderType       string      `json:"type"`   // e.g., "limit", "market"
	Price           float64     `json:"price"`
	Quantity        int         `json:"quantity"`
	Slippage        []float64   `json:"slippage,omitempty"`          // optional, for market orders [base_price, max_slippage_percent]
	MarketOrderType string      `json:"market_order_type,omitempty"` // optional, for market orders IOC or FOK
	ResultChan      chan Result `json:"-"`                           // for server to send back result
}

type Result struct {
	Timestamp int64  `json:"timestamp"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
}

type UpdateDepth struct {
	Timestamp int64   `json:"timestamp"` // on Server side, ignore client input
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"` // "bids" or "asks"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

// Template Only Structs Below

type CreateOrderRequest struct {
	OrderType string  `json:"type"` // e.g., "limit", "market"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type ModifyOrderRequest struct {
	OrderID   string  `json:"order_id"`
	OrderType string  `json:"type"` // e.g., "limit", "market"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type CancelOrderRequest struct {
	OrderID string `json:"order_id"`
}

/* Depth WebSocket */

type Order struct {
	UserID   int `json:"userID"`
	Quantity int `json:"quantity"`
}

type MarketDepth struct {
	Bids      map[float64]map[string]Order `json:"bids"`
	Asks      map[float64]map[string]Order `json:"asks"`
	TotalBids map[float64]int              `json:"totalBids"`
	TotalAsks map[float64]int              `json:"totalAsks"`
	BidTree   *btree.BTree                 `json:"bidTree"`
	AskTree   *btree.BTree                 `json:"askTree"`
}

/* Ledger WebSocket */

type Ledger struct {
	Timestamp   int64   `json:"timestamp"`
	Symbol      string  `json:"symbol"`
	Price       float64 `json:"price"`
	Volume      int     `json:"volume"`
	Side        string  `json:"side"` // "buy" or "sell"
	ExecutionID string  `json:"execution_id"`
	BuyOrderID  string  `json:"buy_order_id"`
	SellOrderID string  `json:"sell_order_id"`
	Conditions  string  `json:"conditions"`
}

/* Session WebSocket */

type SessionStatus struct {
	Session string `json:"session"`
}
