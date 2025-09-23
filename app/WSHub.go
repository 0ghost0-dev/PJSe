package app

import (
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/websocket/v2"
)

type Client struct {
	ID          int
	ConnID      string
	Username    string
	Data        map[string]interface{} // Additional data if needed
	Conn        *websocket.Conn
	Syncing     bool
	syncLock    sync.Mutex
	pendingMsgs []PendingMessage
}

type PendingMessage struct {
	ID          int
	MessageType int
	Data        []byte
}

type WSHub struct {
	clients           sync.Map
	messages          []Message
	lock              sync.Mutex
	AllowMultiConnect bool // true: 여러개 허용, false: 한개만 허용
}

type Message struct {
	ID        int
	Timestamp int64
	Data      []byte
}

func NewWSHub(multiConnection bool) *WSHub {
	return &WSHub{
		AllowMultiConnect: multiConnection,
	}
}

func (hub *WSHub) RegisterClient(client *Client) {
	conns, _ := hub.clients.LoadOrStore(client.ID, &sync.Map{})
	connMap := conns.(*sync.Map)

	if !hub.AllowMultiConnect {
		// 기존 연결 모두 끊기
		connMap.Range(func(_, v interface{}) bool {
			oldClient := v.(*Client)
			err := oldClient.Conn.Close()
			if err != nil {
				return false
			}
			connMap.Delete(oldClient.ConnID)
			return true
		})
	}
	connMap.Store(client.ConnID, client)
}

func (hub *WSHub) DisconnectAll() {
	hub.clients.Range(func(_, v interface{}) bool {
		connMap := v.(*sync.Map)
		connMap.Range(func(_, v interface{}) bool {
			client := v.(*Client)
			err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			if err != nil {
				return false
			}
			err = client.Conn.Close()
			if err != nil {
				log.Error("WebSocket 연결 종료 오류:", err)
			}
			return true
		})
		return true
	})
	hub.clients = sync.Map{} // 모든 클라이언트 맵 초기화
}

func (hub *WSHub) UnregisterClient(client *Client) {
	if conns, ok := hub.clients.Load(client.ID); ok {
		connMap := conns.(*sync.Map)
		connMap.Delete(client.ConnID)

		isEmpty := true
		connMap.Range(func(key, value interface{}) bool {
			isEmpty = false
			return false // Exit early
		})
		if isEmpty {
			hub.clients.Delete(client.ID)
		}
	}
}

func (hub *WSHub) GetClient(userID int, connID string) (*Client, bool) {
	if conns, ok := hub.clients.Load(userID); ok {
		connMap := conns.(*sync.Map)
		if client, ok := connMap.Load(connID); ok {
			return client.(*Client), true
		}
	}
	return nil, false
}

func (hub *WSHub) BroadcastMessage(timestamp int64, messageType int, message []byte) {
	hub.lock.Lock()
	hub.messages = append(hub.messages, Message{
		ID:        0,
		Timestamp: timestamp,
		Data:      message,
	})
	hub.lock.Unlock()

	hub.clients.Range(func(key, value interface{}) bool {
		connMap := value.(*sync.Map)
		connMap.Range(func(_, v interface{}) bool {
			client := v.(*Client)

			// 동기화 중인 클라이언트는 메시지를 버퍼에 저장
			client.syncLock.Lock()
			if client.Syncing {
				client.pendingMsgs = append(client.pendingMsgs, PendingMessage{
					ID:          0,
					MessageType: messageType,
					Data:        message,
				})
				client.syncLock.Unlock()
				return true
			}
			client.syncLock.Unlock()

			err := client.Conn.WriteMessage(messageType, message)
			if err != nil {
				log.Error("WebSocket 전송 오류:", err)
				hub.UnregisterClient(client)
				err := client.Conn.Close()
				if err != nil {
					return false
				}
				return true // Continue to next client
			}
			return true
		})
		return true
	})
}

func (hub *WSHub) SendMessageToUser(userID int, timestamp int64, messageType int, message []byte) {
	hub.lock.Lock()
	hub.messages = append(hub.messages, Message{
		ID:        userID,
		Timestamp: timestamp,
		Data:      message,
	})
	hub.lock.Unlock()

	if conns, ok := hub.clients.Load(userID); ok {
		connMap := conns.(*sync.Map)
		connMap.Range(func(_, v interface{}) bool {
			client := v.(*Client)

			// 동기화 중인 클라이언트는 메시지를 버퍼에 저장
			client.syncLock.Lock()
			if client.Syncing {
				client.pendingMsgs = append(client.pendingMsgs, PendingMessage{
					ID:          userID,
					MessageType: messageType,
					Data:        message,
				})
				client.syncLock.Unlock()
				return true
			}
			client.syncLock.Unlock()

			err := client.Conn.WriteMessage(messageType, message)
			if err != nil {
				log.Error("WebSocket 전송 오류:", err)
				hub.UnregisterClient(client)
				err := client.Conn.Close()
				if err != nil {
					return false
				}
				return true // Continue to next client
			}
			return true
		})
	}
}

func (hub *WSHub) SendMessageToUserSince(client *Client, since string) {
	client.syncLock.Lock()
	client.Syncing = true
	client.syncLock.Unlock()

	defer func() {
		client.syncLock.Lock()
		client.Syncing = false
		client.syncLock.Unlock()
	}()

	hub.lock.Lock()
	messages := make([]Message, len(hub.messages))
	copy(messages, hub.messages)
	hub.lock.Unlock()

	// since 문자열을 int64로 변환
	var sinceInt int64
	_, err := fmt.Sscan(since, &sinceInt)
	if err != nil {
		log.Error("since 파라미터 변환 오류:", err)
		return
	}

	for _, msg := range messages {
		if (msg.ID == 0 || msg.ID == client.ID) && msg.Timestamp > sinceInt {
			err := client.Conn.WriteMessage(websocket.TextMessage, msg.Data)
			if err != nil {
				log.Error("WebSocket 전송 오류:", err)
				return
			}
		}
	}

	defer func() {
		client.syncLock.Lock()
		client.Syncing = false

		// 동기화 중 대기된 메시지들 전송
		for _, pendingMsg := range client.pendingMsgs {
			if pendingMsg.ID == 0 || pendingMsg.ID == client.ID {
				err := client.Conn.WriteMessage(pendingMsg.MessageType, pendingMsg.Data)
				if err != nil {
					log.Error("대기된 메시지 전송 오류:", err)
					break
				}
			}
		}
		client.pendingMsgs = nil // 버퍼 초기화

		client.syncLock.Unlock()
	}()
}

func (hub *WSHub) ClearMessages() {
	hub.lock.Lock()
	hub.messages = nil
	hub.lock.Unlock()
}
