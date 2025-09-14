package exchanges

import (
	"PJS_Exchange/singletons/postgresApp"
	"context"
	"fmt"
	"os"
	"sync"
	"time"
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

func processGetSession() error {
	return UpdateMarketStatus()
}

func processClearExpiredAPIKeys() error {
	ctx := context.Background()
	return postgresApp.Get().APIKeyRepo().CleanupExpiredKeys(ctx)
}

func processClearRedisCache() error {
	// 프리장 시작 10분 전에 Redis 캐시 비우기
	openedTime := getOpenedTime()
	if openedTime == nil {
		return nil
	}
	nowTime, _ := time.Parse("15:04", time.Now().Format("15:04"))
	if MarketStatus == "closed" && nowTime.Equal(openedTime.Add(-10*time.Minute)) {
		// TODO: Redis 캐시 비우기
		return nil
	}
	return nil
}

func worker(id int, jobChan <-chan Job, wg *sync.WaitGroup) {
	defer wg.Done()
	//fmt.Printf("워커 %d 시작\n", id)

	for {
		select {
		case job, ok := <-jobChan:
			if !ok {
				//fmt.Printf("워커 %d 종료 (채널 닫힘)\n", id)
				return
			}

			//fmt.Printf("워커 %d: 작업 %d 처리 중... (%s)\n", id, job.ID, job.Data)

			err := processJob(job)
			if err != nil {
				fmt.Printf("워커 %d: 작업 %d 실패 - %v\n", id, job.ID, err)
			} else {
				//fmt.Printf("워커 %d: 작업 %d 완료\n", id, job.ID)
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
	//fmt.Printf("다음 실행 시간: %s (대기: %v)\n",
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
			//fmt.Printf("%s - %s 작업 %d 생성됨\n", currentTime, jobType, *jobID)
			*jobID++
		default:
			fmt.Printf("%s - 워커들이 바쁨, 작업 건너뜀\n", currentTime)
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

	fmt.Printf("Job Scheduler 시작됨 (PID: %d)\n", os.Getpid())
}
