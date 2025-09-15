package ws

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middleware"
	"PJS_Exchange/template"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

var (
	LedgerHub  = app.NewWSHub()
	TempLedger = make(map[string][]template.Ledger) // 심볼별 임시 원장 데이터 저장용 (예: "NVDA" : [{Timestamp: ..., Price: ..., Volume: ...}, ...])
)

type LedgerRouter struct{}

func (lr *LedgerRouter) RegisterRoutes(router fiber.Router) {
	ledgerGroup := router.Group("/ledger", middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
		MarketDataRead: true,
	}))

	ledgerGroup.Get("/", websocket.New(lr.handleLedger))
}

// TODO 추후 protobuf로 변경
// @summary		Ledger WebSocket
// @description	일일 실시간 체결 데이터를 WebSocket을 통해 구독합니다.
// @tags		WebSocket
// @produce		json
// @param		since	query	string	false	"특정 타임스탬프 이후의 데이터를 받기 위한 옵션 (0을 입력하면 오늘 발생한 전체 데이터 수신)"
// @success		200	{string}	string	"WebSocket 연결 성공 및 구독 시작 메시지"
// @failure		400	{object}	map[string]string	"잘못된 요청"
// @failure		401	{object}	map[string]string	"인증 실패"
// @failure		500	{object}	map[string]string	"서버 오류"
// @router		/ws/ledger [get]
func (lr *LedgerRouter) handleLedger(c *websocket.Conn) {
	since := c.Query("since", "-1")
	user := c.Locals("user").(*postgresql.User)

	client := &app.Client{
		ID:       user.ID,
		ConnID:   uuid.NewString(),
		Username: user.Username,
		Conn:     c,
	}

	LedgerHub.RegisterClient(client)
	log.Printf("User %s subscribed to ledger updates", user.Username)
	defer func() {
		LedgerHub.UnregisterClient(client)
		log.Printf("User %s unsubscribed from ledger updates", user.Username)
	}()

	// since 파라미터가 "-1"이 아닌 경우, 해당 타임스탬프 이후의 데이터 전송
	if since != "-1" {
		LedgerHub.SendMessageToUserSince(client, since)
	}

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}
