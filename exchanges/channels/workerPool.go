package channels

import (
	"PJS_Exchange/app/postgresApp"
	"PJS_Exchange/exchanges"
	"PJS_Exchange/routes/ws"
	"PJS_Exchange/template"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

type Job struct {
	ID   int
	Type string
	Data string
}

func processJob(job Job) error {
	switch job.Type {
	case "get_session":
		return processGetSession()
	case "clear_expired_api_keys":
		return processClearExpiredAPIKeys()
	case "clear_redis_cache":
		return processClearRedisCache()
	}
	return nil
}

// TODO 추후 protobuf로 변경
func processGetSession() error {
	previousStatus := exchanges.MarketStatus

	// 세션 상태 업데이트
	err := exchanges.UpdateMarketStatus()
	if err != nil {
		return fmt.Errorf("UpdateMarketStatus error: %v", err)
	}

	// 프리장 시작 30분 전, 5분 전, 1분 전 알림
	sessionTime := exchanges.GetChangeSessionTime()
	if sessionTime == nil {
		return nil
	}

	preT := (*sessionTime)["pre"].Add(-30 * time.Minute)
	preF := (*sessionTime)["pre"].Add(-5 * time.Minute)
	preO := (*sessionTime)["pre"].Add(-1 * time.Minute)
	nowTime, _ := time.Parse("15:04", time.Now().Format("15:04"))

	if nowTime.Equal(preT) {
		sender, err := json.Marshal(template.SessionStatus{
			Session: "pre-30m",
		})
		if err != nil {
			return fmt.Errorf("failed to marshal session status: %v", err)
		}
		ws.SessionHub.BroadcastMessage(time.Now().UnixMilli(), websocket.TextMessage, sender)
	} else if nowTime.Equal(preF) {
		sender, err := json.Marshal(template.SessionStatus{
			Session: "pre-5m",
		})
		if err != nil {
			return fmt.Errorf("failed to marshal session status: %v", err)
		}
		ws.SessionHub.BroadcastMessage(time.Now().UnixMilli(), websocket.TextMessage, sender)
	} else if nowTime.Equal(preO) {
		sender, err := json.Marshal(template.SessionStatus{
			Session: "pre-1m",
		})
		if err != nil {
			return fmt.Errorf("failed to marshal session status: %v", err)
		}
		ws.SessionHub.BroadcastMessage(time.Now().UnixMilli(), websocket.TextMessage, sender)
	}

	if previousStatus != exchanges.MarketStatus {
		// 세션 상태가 변경된 경우에만 알림 전송
		//log.Printf("Market status changed from %s to %s", previousStatus, MarketStatus)
		sender, err := json.Marshal(template.SessionStatus{
			Session: exchanges.MarketStatus,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal session status: %v", err)
		}
		ws.SessionHub.BroadcastMessage(time.Now().UnixMilli(), websocket.TextMessage, sender)
	}

	// 장 종료 10분 후 모든 클라이언트 연결 종료 처리 (세션 WS 제외)
	// 이전 상태가 "post"였고 현재 상태가 "closed"인 경우
	if previousStatus == "post" && exchanges.MarketStatus == "closed" {
		time.AfterFunc(10*time.Minute, func() {
			ws.DepthHub.DisconnectAll()
			ws.LedgerHub.DisconnectAll()
			ws.NotifyHub.DisconnectAll()
		})
	}
	return nil
}

func processClearExpiredAPIKeys() error {
	ctx := context.Background()
	return postgresApp.Get().APIKeyRepo().CleanupExpiredKeys(ctx)
}

func processClearRedisCache() error {
	// 프리장 시작 30분 전에 Redis 캐시 비우기
	sessionTime := exchanges.GetChangeSessionTime()
	if sessionTime == nil {
		return nil
	}

	preOpen := (*sessionTime)["pre"]
	nowTime, _ := time.Parse("15:04", time.Now().Format("15:04"))
	if exchanges.MarketStatus == "closed" && nowTime.Equal(preOpen.Add(-30*time.Minute)) {
		// TODO: Redis 캐시 비우기
		return nil
	}
	return nil
}

func worker(id int, jobChan <-chan Job, wg *sync.WaitGroup) {
	defer wg.Done()
	//log.Printf("워커 %d 시작\n", id)

	for {
		select {
		case job, ok := <-jobChan:
			if !ok {
				//log.Printf("워커 %d 종료 (채널 닫힘)\n", id)
				return
			}

			//log.Printf("워커 %d: 작업 %d 처리 중... (%s)\n", id, job.ID, job.Data)

			err := processJob(job)
			if err != nil {
				log.Printf("워커 %d: 작업 %d 실패 - %v\n", id, job.ID, err)
			} else {
				//log.Printf("워커 %d: 작업 %d 완료\n", id, job.ID)
			}
		}
	}
}

func scheduler(jobChan chan<- Job, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(jobChan)

	jobID := 1

	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	//log.Printf("다음 실행 시간: %s (대기: %v)\n",
	//	nextMinute.Format("15:04:05"),
	//	time.Until(nextMinute))

	// 첫 번째 정각까지 대기
	time.Sleep(time.Until(nextMinute))

	if !createAndSendJob(jobChan, &jobID) {
		return
	}

	// 이후 1분마다 실행
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !createAndSendJob(jobChan, &jobID) {
				return
			}
		}
	}
}

func createAndSendJob(jobChan chan<- Job, jobID *int) bool {
	currentTime := time.Now().Format("15:04:05")

	jobTypes := []string{"get_session", "clear_expired_api_keys", "clear_redis_cache"}

	for _, jobType := range jobTypes {
		job := Job{
			ID:   *jobID,
			Type: jobType,
			Data: fmt.Sprintf("%s 작업 - %s", jobType, currentTime),
		}

		select {
		case jobChan <- job:
			//log.Printf("%s - %s 작업 %d 생성됨\n", currentTime, jobType, *jobID)
			*jobID++
		default:
			log.Printf("%s - 워커들이 바쁨, 작업 건너뜀\n", currentTime)
		}
	}

	return true
}

func RunWorkerPool() {
	jobChan := make(chan Job, 5)

	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go worker(i, jobChan, &wg)
	}

	wg.Add(1)
	go scheduler(jobChan, &wg)

	log.Printf("Job Scheduler 시작됨 (PID: %d)\n", os.Getpid())
}
