package ws

import (
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middleware"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type DepthRouter struct{}

func (dr *DepthRouter) RegisterRoutes(router fiber.Router) {
	depthGroup := router.Group("/depth", middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
		MarketDataRead: true,
	}))

	depthGroup.Get("/:sym", websocket.New(dr.handleDepth))
}

// @summary		Depth WebSocket
// @description	특정 심볼의 실시간 호가(Depth) 데이터를 WebSocket을 통해 구독합니다.
// @tags		WebSocket
// @param		sym	path		string	true	"심볼 (예: NVDA)"
// @produce		json
// @success		200	{string}	string	"WebSocket 연결 성공 및 구독 시작 메시지"
// @failure		400	{object}	fiber.Error	"잘못된 요청"
// @failure		401	{object}	fiber.Error	"인증 실패"
// @failure		500	{object}	fiber.Error	"서버 오류"
// @router		/ws/depth/{sym} [get]
func (dr *DepthRouter) handleDepth(c *websocket.Conn) {
	symbol := c.Params("sym")

	err := c.WriteMessage(websocket.TextMessage, []byte("Subscribed to depth updates for symbol: "+symbol))
	if err != nil {
		return
	}

	for {
		mt, msg, err := c.ReadMessage()
		if err != nil {
			log.Println("읽기 오류:", err)
			break
		}
		log.Println("메시지 타입:", mt)
		log.Printf("받은 메시지: %s", msg)
	}
}
