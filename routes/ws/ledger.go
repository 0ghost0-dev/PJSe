package ws

import (
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type LedgerRouter struct{}

func (lr *LedgerRouter) RegisterRoutes(router fiber.Router) {
	ledgerGroup := router.Group("/ledger", middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
		MarketDataRead: true,
	}))

	ledgerGroup.Get("/", websocket.New(lr.handleLedger))
}

// @summary		Ledger WebSocket
// @description	실시간 일일 원장(Ledger) 데이터를 WebSocket을 통해 구독합니다.
// @tags		WebSocket
// @produce		json
// @success		200	{string}	string	"WebSocket 연결 성공 및 구독 시작 메시지"
// @failure		400	{object}	fiber.Error	"잘못된 요청"
// @failure		401	{object}	fiber.Error	"인증 실패"
// @failure		500	{object}	fiber.Error	"서버 오류"
// @router		/ws/ledger [get]
func (lr *LedgerRouter) handleLedger(c *websocket.Conn) {

}
