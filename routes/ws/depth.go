package ws

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middlewares/auth"
	"PJS_Exchange/template"
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

var (
	DepthHub              = app.NewWSHub(false)
	TempDepth             = make(map[string]template.MarketDepth)     // 심볼별 임시 호가 데이터 저장용 (예: "NVDA" : {Bids: [...], Asks: [...]})
	TempDepthOrderIDIndex = make(map[string]map[string][]interface{}) // 심볼별 주문 ID 인덱스 (예: "NVDA" : {"orderID1": [1, "bid", 123.45, 10, 1], "orderID2": [2, "ask", 678.90, 20, 1]}
)

type DepthRouter struct{}

func (dr *DepthRouter) RegisterRoutes(router fiber.Router) {
	depthGroup := router.Group("/depth", auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
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
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
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
		Syncing:  since != "-1",
	}

	const (
		pingInterval = 20 * time.Second // 30초마다 PING
		pongTimeout  = 40 * time.Second // 60초 타임아웃
		writeWait    = 10 * time.Second // 쓰기 대기 시간
	)

	DepthHub.RegisterClient(client)
	log.Printf("User %s subscribed to depth updates since %s", user.Username, since)
	defer func() {
		DepthHub.UnregisterClient(client)
		log.Printf("User %s unsubscribed from depth updates", user.Username)
	}()

	// PING/PONG 관리용 고루틴
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(pingInterval) // 30초마다 ping
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := c.WriteControl(websocket.PingMessage, []byte("heartbeat"), time.Now().Add(writeWait)); err != nil {
					//log.Printf("Failed to send ping to user %s: %v", user.Username, err)
					err := c.Close()
					if err != nil {
						return
					}
					return
				}
				//log.Printf("Sent heartbeat to user %s", user.Username)
			case <-ctx.Done():
				return
			}
		}
	}()

	// 초기 읽기 데드라인 설정
	err := c.SetReadDeadline(time.Now().Add(pongTimeout))
	if err != nil {
		return
	}

	c.SetPongHandler(func(appData string) error {
		//log.Printf("Received pong from user %s: %s", user.Username, appData)
		return c.SetReadDeadline(time.Now().Add(pongTimeout))
	})

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
