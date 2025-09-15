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
	DepthHub  = app.NewWSHub()
	TempDepth = make(map[string]template.MarketDepth) // 심볼별 임시 호가 데이터 저장용 (예: "NVDA" : {Bids: [...], Asks: [...]})
)

type DepthRouter struct{}

func (dr *DepthRouter) RegisterRoutes(router fiber.Router) {
	depthGroup := router.Group("/depth", middleware.AuthAPIKeyMiddlewareRequireScopes(middleware.AuthConfig{Bypass: false}, postgresql.APIKeyScope{
		MarketDataRead: true,
	}))

	depthGroup.Get("/", websocket.New(dr.handleDepth))
	//depthGroup.Get("/:sym", websocket.New(dr.handleSelDepth))
}

// TODO 추후 protobuf로 변경
// @summary		Depth WebSocket
// @description	일일 실시간 호가 데이터를 WebSocket을 통해 구독합니다.
// @tags		WebSocket
// @produce		json
// @param		since	query	string	false	"특정 타임스탬프 이후의 데이터를 받기 위한 옵션 (0을 입력하면 오늘 발생한 전체 데이터 수신)"
// @success		200	{string}	string	"WebSocket 연결 성공 및 구독 시작 메시지"
// @failure		400	{object}	map[string]string	"잘못된 요청"
// @failure		401	{object}	map[string]string	"인증 실패"
// @failure		500	{object}	map[string]string	"서버 오류"
// @router		/ws/depth [get]
func (dr *DepthRouter) handleDepth(c *websocket.Conn) {
	since := c.Query("since", "-1")
	user := c.Locals("user").(*postgresql.User)

	client := &app.Client{
		ID:       user.ID,
		ConnID:   uuid.NewString(),
		Username: user.Username,
		Conn:     c,
	}

	DepthHub.RegisterClient(client)
	log.Printf("User %s subscribed to depth updates since %s.", user.Username, since)
	defer func() {
		DepthHub.UnregisterClient(client)
		log.Printf("User %s unsubscribed from depth updates since %s.", user.Username, since)
	}()

	// since 파라미터가 "-1"이 아닌 경우, 해당 타임스탬프 이후의 데이터 전송
	if since != "-1" {
		DepthHub.SendMessageToUserSince(client, since)
	}

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}
