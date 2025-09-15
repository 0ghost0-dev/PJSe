package template

/* Market/Order Types */

type OrderStatus struct {
	OrderID   string  `json:"order_id"`
	Side      string  `json:"side"` // "buy" or "sell"
	OrderType string  `json:"type"` // e.g., "limit", "market", "stop-limit"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type OrderRequest struct {
	Timestamp int64   `json:"timestamp"` // on Server side, ignore client input
	UserID    int     `json:"user_id"`   // on Server side, ignore client input
	OrderID   string  `json:"order_id"`  // on Server side, ignore client input
	Symbol    string  `json:"symbol"`    // on Server side, ignore client input
	Status    string  `json:"status"`    // on Server side, ignore client input
	Side      string  `json:"side"`      // on Server side, ignore client input
	OrderType string  `json:"type"`      // e.g., "limit", "market", "stop-limit"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
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
	OrderType string  `json:"type"` // e.g., "limit", "market", "stop-limit"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type ModifyOrderRequest struct {
	OrderID   string  `json:"order_id"`
	OrderType string  `json:"type"` // e.g., "limit", "market", "stop-limit"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type CancelOrderRequest struct {
	OrderID string `json:"order_id"`
}

/* Depth WebSocket */

type Order struct {
	UserID   int    `json:"userID"`
	OrderID  string `json:"orderID"`
	Quantity int    `json:"quantity"`
}

type MarketDepth struct {
	Bids      map[float64][]Order `json:"bids"`
	Asks      map[float64][]Order `json:"asks"`
	TotalBids map[float64]int     `json:"totalBids"`
	TotalAsks map[float64]int     `json:"totalAsks"`
}

/* Ledger WebSocket */

type Ledger struct {
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
	Volume    int     `json:"volume"`
}
