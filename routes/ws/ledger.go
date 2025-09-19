package ws

import (
	"PJS_Exchange/app"
	"PJS_Exchange/databases/postgresql"
	"PJS_Exchange/middlewares/auth"
	"PJS_Exchange/template"
	"PJS_Exchange/utils"
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

var (
	LedgerHub  = app.NewWSHub(false)
	TempLedger = make(map[string]*utils.ChunkedStore[template.Ledger]) // 심볼별 임시 원장 데이터 저장용 (예: "NVDA" : [{Timestamp: ..., Price: ..., Volume: ...}, ...])
)

type LedgerRouter struct{}

func (lr *LedgerRouter) RegisterRoutes(router fiber.Router) {
	ledgerGroup := router.Group("/ledger", auth.APIKeyMiddlewareRequireScopes(auth.Config{Bypass: false}, postgresql.APIKeyScope{
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
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
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
		Syncing:  since != "-1",
	}

	const (
		pingInterval = 20 * time.Second // 30초마다 PING
		pongTimeout  = 40 * time.Second // 60초 타임아웃
		writeWait    = 10 * time.Second // 쓰기 대기 시간
	)

	LedgerHub.RegisterClient(client)
	log.Printf("User %s subscribed to ledger updates since %s", user.Username, since)
	defer func() {
		LedgerHub.UnregisterClient(client)
		log.Printf("User %s unsubscribed from ledger updates", user.Username)
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
		LedgerHub.SendMessageToUserSince(client, since)
	}

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}
