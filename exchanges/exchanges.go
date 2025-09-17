package exchanges

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

var (
	MarketStatus   string
	cachedExchange *ExchangeType
	lastModTime    time.Time
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
	Anniversaries          []Anniversary      `json:"anniversaries"`
}

type Session struct {
	Open  *string `json:"open"`
	Close *string `json:"close"`
}

type Anniversary struct {
	Date                   string  `json:"date"`
	Name                   string  `json:"name"`
	PreMarketSessions      Session `json:"pre_market_sessions"`
	RegularTradingSessions Session `json:"regular_trading_sessions"`
	PostMarketSessions     Session `json:"post_market_sessions"`
}

func Load() (*ExchangeType, error) {
	info, err := os.Stat("./exchanges/PJSe.json")
	if err != nil {
		return nil, err
	}

	if cachedExchange == nil || info.ModTime().After(lastModTime) {
		// 파일 읽기 및 파싱
		data, err := os.ReadFile("./exchanges/PJSe.json")
		if err != nil {
			return nil, err
		}

		var exchange ExchangeType
		err = json.Unmarshal(data, &exchange)
		if err != nil {
			return nil, err
		}

		cachedExchange = &exchange
		lastModTime = info.ModTime()
	}

	return cachedExchange, nil
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

	// 캐시 초기화
	cachedExchange = nil

	return nil
}

// getCurrentSession 현재 세션 반환
func getCurrentSession() string {
	e, err := Load()
	if err != nil {
		return "cannot_load"
	}

	date := time.Now().Format("2006-01-02")
	weekday := time.Now().Weekday().String()
	currentTime := time.Now().Format("15:04")

	// 오늘이 기념일인지 확인
	anniversary := Anniversary{}
	if e.Anniversaries != nil {
		for _, ann := range e.Anniversaries {
			if ann.Date == date {
				anniversary = ann
				break
			}
		}
	}

	pre := Session{}
	regular := Session{}
	post := Session{}
	if anniversary.Date != "" {
		// 기념일 세션 사용
		pre = anniversary.PreMarketSessions
		regular = anniversary.RegularTradingSessions
		post = anniversary.PostMarketSessions
	} else {
		// 일반 세션 사용
		pre = e.PreMarketSessions[weekday]
		regular = e.RegularTradingSessions[weekday]
		post = e.PostMarketSessions[weekday]
	}

	// 세션 상태 확인
	if pre.Open != nil && pre.Close != nil {
		preOpen, _ := time.Parse("15:04", *pre.Open)
		preClose, _ := time.Parse("15:04", *pre.Close)
		currentTimeParsed, _ := time.Parse("15:04", currentTime)
		if (currentTimeParsed.After(preOpen) || currentTimeParsed.Equal(preOpen)) && currentTimeParsed.Before(preClose) {
			return "pre"
		}
	}
	if regular.Open != nil && regular.Close != nil {
		regularOpen, _ := time.Parse("15:04", *regular.Open)
		regularClose, _ := time.Parse("15:04", *regular.Close)
		currentTimeParsed, _ := time.Parse("15:04", currentTime)
		if (currentTimeParsed.After(regularOpen) || currentTimeParsed.Equal(regularOpen)) && currentTimeParsed.Before(regularClose) {
			return "regular"
		}
	}
	if post.Open != nil && post.Close != nil {
		postOpen, _ := time.Parse("15:04", *post.Open)
		postClose, _ := time.Parse("15:04", *post.Close)
		currentTimeParsed, _ := time.Parse("15:04", currentTime)
		if (currentTimeParsed.After(postOpen) || currentTimeParsed.Equal(postOpen)) && currentTimeParsed.Before(postClose) {
			return "post"
		}
	}
	return "closed"
}

// GetChangeSessionTime 오늘 세션이 변경되는 시간 반환 (없으면 nil)
func GetChangeSessionTime() *map[string]time.Time {
	e, err := Load()
	if err != nil {
		return nil
	}

	weekday := time.Now().Weekday().String()
	date := time.Now().Format("2006-01-02")

	// 오늘이 기념일인지 확인
	anniversary := Anniversary{}
	if e.Anniversaries != nil {
		for _, ann := range e.Anniversaries {
			if ann.Date == date {
				anniversary = ann
				break
			}
		}
	}

	pre := Session{}
	regular := Session{}
	post := Session{}
	if anniversary.Date != "" {
		// 기념일 세션 사용
		pre = anniversary.PreMarketSessions
		regular = anniversary.RegularTradingSessions
		post = anniversary.PostMarketSessions
	} else {
		// 일반 세션 사용
		pre = e.PreMarketSessions[weekday]
		regular = e.RegularTradingSessions[weekday]
		post = e.PostMarketSessions[weekday]
	}

	changeTimes := make(map[string]time.Time)

	if pre.Open != nil {
		preOpen, err := time.Parse("15:04", *pre.Open)
		if err == nil {
			changeTimes["pre"] = preOpen
		}
	}
	if regular.Open != nil {
		regularOpen, err := time.Parse("15:04", *regular.Open)
		if err == nil {
			changeTimes["regular"] = regularOpen
		}
	}
	if post.Open != nil {
		postOpen, err := time.Parse("15:04", *post.Open)
		if err == nil {
			changeTimes["post"] = postOpen
		}
	}
	if post.Close != nil {
		postClose, err := time.Parse("15:04", *post.Close)
		if err == nil {
			changeTimes["closed"] = postClose
		}
	}

	if len(changeTimes) == 0 {
		return nil
	}
	return &changeTimes
}

func UpdateMarketStatus() error {
	tmp := getCurrentSession()
	if tmp != "cannot_load" {
		MarketStatus = tmp
		return nil
	}
	return fmt.Errorf("cannot load exchange info")
}
