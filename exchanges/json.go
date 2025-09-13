package exchanges

import (
	"encoding/json"
	"os"
)

type ExchangeType struct {
	Name                   string             `json:"name"`
	ShortName              string             `json:"short_name"`
	Country                string             `json:"country"`
	DefaultCurrency        string             `json:"default_currency"`
	DefaultUTCOffset       int                `json:"default_utc_offset"`
	DefaultTimezone        string             `json:"default_timezone"`
	AvailableTypes         []string           `json:"available_types"`
	Url                    string             `json:"url"`
	Logo                   string             `json:"logo"`
	Description            string             `json:"description"`
	PreMarketSessions      map[string]Session `json:"pre_market_sessions"`
	RegularTradingSessions map[string]Session `json:"regular_trading_sessions"`
	PostMarketSessions     map[string]Session `json:"post_market_sessions"`
	Holidays               []Holiday          `json:"holidays"`
}

type Session struct {
	Open  *string `json:"open"`
	Close *string `json:"close"`
}

type Holiday struct {
	Date                   string  `json:"date"`
	Name                   string  `json:"name"`
	PreMarketSessions      Session `json:"pre_market_sessions"`
	RegularTradingSessions Session `json:"regular_trading_sessions"`
	PostMarketSessions     Session `json:"post_market_sessions"`
}

// Load 거래소 정보 로드
func Load() (*ExchangeType, error) {
	data, err := os.ReadFile("./exchanges/PJSe.json")
	if err != nil {
		return nil, err
	}

	// JSON 파싱
	var jsonData ExchangeType
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		return nil, err
	}

	return &jsonData, nil
}

// Edit 반드시 Load 후에 호출할 것
func Edit(e *ExchangeType) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile("./exchanges/PJSe.json", data, 0644)
	if err != nil {
		return err
	}

	return nil
}
