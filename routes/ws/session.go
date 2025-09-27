package ws

import (
	"PJS_Exchange/app"
	"PJS_Exchange/exchanges"
	"PJS_Exchange/template"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

var (
	SessionHub = app.NewWSHub(false)
)

type SessionRouter struct{}

func (sr *SessionRouter) RegisterRoutes(router fiber.Router) {
	SessionGroup := router.Group("/session")

	SessionGroup.Get("/", websocket.New(sr.handleStatus))
}

// TODO 추후 protobuf로 변경
// @summary		Session WebSocket
// @description	실시간 세션 상태 데이터를 WebSocket을 통해 구독합니다.
// @tags		WebSocket
// @produce		json
// @Param			Authorization	header		string				true	"Bearer {API_KEY}"
// @success		200	{string}	string	"WebSocket 연결 성공 및 구독 시작 메시지"
// @failure		400	{object}	map[string]string	"잘못된 요청"
// @failure		500	{object}	map[string]string	"서버 오류"
// @router		/ws/session [get]
func (sr *SessionRouter) handleStatus(c *websocket.Conn) {

	client := &app.Client{
		ID:       1,
		ConnID:   uuid.NewString(),
		Username: "session_user",
		Conn:     c,
		Syncing:  false,
	}

	const (
		pingInterval = 20 * time.Second // 30초마다 PING
		pongTimeout  = 40 * time.Second // 60초 타임아웃
		writeWait    = 10 * time.Second // 쓰기 대기 시간
	)

	// 초기 세션 상태 전송
	session, err := json.Marshal(template.SessionStatus{
		Session: exchanges.MarketStatus,
	})
	if err != nil {
		log.Printf("Failed to marshal session status for user: %v", err)
		return
	}
	err = c.WriteMessage(websocket.TextMessage, session)
	if err != nil {
		return
	}

	SessionHub.RegisterClient(client)
	log.Printf("User subscribed to session updates")
	defer func() {
		SessionHub.UnregisterClient(client)
		log.Printf("User unsubscribed from session updates")
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
	err = c.SetReadDeadline(time.Now().Add(pongTimeout))
	if err != nil {
		return
	}

	c.SetPongHandler(func(appData string) error {
		//log.Printf("Received pong from user %s: %s", user.Username, appData)
		return c.SetReadDeadline(time.Now().Add(pongTimeout))
	})

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}
