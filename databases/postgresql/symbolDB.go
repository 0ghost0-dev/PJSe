package postgresql

import (
	"PJS_Exchange/databases"
	"context"
	"encoding/json"
	"fmt"
)

const (
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusInit      = "init"
	StatusSuspended = "suspended"
	StatusDelisted  = "delisted"
)

type Status struct {
	Status string `json:"status"` // "active", "inactive", "init", "suspended", "delisted"
	Reason string `json:"reason"` // 거래정지 혹은 상장폐지 사유
}

type Symbol struct {
	ID                   int             `json:"id"`
	Symbol               string          `json:"symbol"`
	Name                 string          `json:"name"`
	Detail               string          `json:"detail"`
	Url                  string          `json:"url"`
	Logo                 string          `json:"logo"`
	Market               string          `json:"market"`
	Type                 string          `json:"type"` // "stock", "index" 등
	MinimumOrderQuantity float32         `json:"minimum_order_quantity"`
	TickSize             float32         `json:"tick_size"`
	TotalStocks          int64           `json:"total_stocks"`
	Tags                 map[string]bool `json:"tags"`
	Status               Status          `json:"status"`
}

type SymbolRepository interface {
}

type SymbolDBRepository struct {
	db *databases.PostgresDBPool
}

func NewSymbolRepository(db *databases.PostgresDBPool) *SymbolDBRepository {
	return &SymbolDBRepository{db: db}
}

func (r *SymbolDBRepository) CreateSymbolsTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS symbols (
		id SERIAL PRIMARY KEY,
		symbol VARCHAR(20) UNIQUE NOT NULL,
		name VARCHAR(100) NOT NULL,
		detail TEXT,
		url TEXT,
		logo TEXT,
		market VARCHAR(50) DEFAULT 'PJSe',
		type VARCHAR(50) DEFAULT 'stock',
		minimum_order_quantity REAL DEFAULT 1,
		tick_size REAL DEFAULT 1,
		total_stocks BIGINT DEFAULT 0,
		tags JSONB DEFAULT '{}'::jsonb,
		status JSONB DEFAULT '{"status": "inactive", "reason": ""}'::jsonb
	);
	`
	_, err := r.db.GetPool().Exec(ctx, query)
	return err
}

func (r *SymbolDBRepository) SymbolListing(ctx context.Context, sym *Symbol) (*Symbol, error) {
	// 심볼이 이미 존재하는지 확인
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM symbols WHERE symbol = $1)`
	err := r.db.GetPool().QueryRow(ctx, checkQuery, sym.Symbol).Scan(&exists)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, fmt.Errorf("symbol '%s' already exists", sym.Symbol)
	}

	// 새 심볼 삽입
	statusJSON, err := json.Marshal(sym.Status)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO symbols (symbol, name, detail, url, logo, market, type, minimum_order_quantity, tick_size, total_stocks, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	err = r.db.GetPool().QueryRow(ctx, query,
		sym.Symbol, sym.Name, sym.Detail, sym.Url, sym.Logo,
		sym.Market, sym.Type, sym.MinimumOrderQuantity, sym.TickSize, sym.TotalStocks, statusJSON).Scan(&sym.ID)

	if err != nil {
		return nil, err
	}

	return sym, nil
}

func (r *SymbolDBRepository) GetSymbols(ctx context.Context) (*[]Symbol, error) {
	query := `SELECT id, symbol, name, detail, url, logo, market, type, minimum_order_quantity, tick_size, total_stocks, status FROM symbols`
	rows, err := r.db.GetPool().Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []Symbol

	for rows.Next() {
		sym := Symbol{}
		var statusJSON []byte

		err := rows.Scan(&sym.ID, &sym.Symbol, &sym.Name, &sym.Detail, &sym.Url, &sym.Logo,
			&sym.Market, &sym.Type, &sym.MinimumOrderQuantity, &sym.TickSize, &sym.TotalStocks, &statusJSON)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(statusJSON, &sym.Status)
		if err != nil {
			return nil, err
		}

		symbols = append(symbols, sym)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return &symbols, nil
}

func (r *SymbolDBRepository) GetSymbolData(ctx context.Context, symbol string) (*Symbol, error) {
	query := `SELECT id, symbol, name, detail, url, logo, market, type, minimum_order_quantity, tick_size, total_stocks, status FROM symbols WHERE symbol = $1`
	sym := &Symbol{}
	var statusJSON []byte

	err := r.db.GetPool().QueryRow(ctx, query, symbol).Scan(
		&sym.ID, &sym.Symbol, &sym.Name, &sym.Detail, &sym.Url, &sym.Logo,
		&sym.Market, &sym.Type, &sym.MinimumOrderQuantity, &sym.TickSize, &sym.TotalStocks, &statusJSON)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(statusJSON, &sym.Status)
	if err != nil {
		return nil, err
	}

	return sym, nil
}

func (r *SymbolDBRepository) IsSymbolExist(ctx context.Context, symbol string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM symbols WHERE symbol = $1)`
	_ = r.db.GetPool().QueryRow(ctx, query, symbol).Scan(&exists)
	return exists, fmt.Errorf("Symbol '" + symbol + "' does not exist.")
}

func (r *SymbolDBRepository) UpdateSymbolStatus(ctx context.Context, symbol string, status Status) error {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return err
	}

	query := `UPDATE symbols SET status = $1 WHERE symbol = $2`
	_, err = r.db.GetPool().Exec(ctx, query, statusJSON, symbol)
	return err
}

func (r *SymbolDBRepository) SetTickSize(ctx context.Context, symbol string, tickSize float32) error {
	query := `UPDATE symbols SET tick_size = $1 WHERE symbol = $2`
	_, err := r.db.GetPool().Exec(ctx, query, tickSize, symbol)
	return err
}

func (r *SymbolDBRepository) SetMinimumOrderQuantity(ctx context.Context, symbol string, minQty float32) error {
	query := `UPDATE symbols SET minimum_order_quantity = $1 WHERE symbol = $2`
	_, err := r.db.GetPool().Exec(ctx, query, minQty, symbol)
	return err
}

func (r *SymbolDBRepository) SumTotalStocks(ctx context.Context, symbol string, delta int64) error {
	query := `UPDATE symbols SET total_stocks = total_stocks + $1 WHERE symbol = $2`
	_, err := r.db.GetPool().Exec(ctx, query, delta, symbol)
	return err
}

// 추가 유틸리티 메서드들

func IsSymbolStructComplete(sym *Symbol) bool {
	return sym.Symbol != "" &&
		sym.Name != "" &&
		sym.Market != "" &&
		sym.Type != "" &&
		sym.MinimumOrderQuantity > 0 &&
		sym.TickSize > 0 &&
		sym.Detail != "" &&
		sym.Url != "" &&
		sym.Logo != "" &&
		sym.TotalStocks >= 0
}
