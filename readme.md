# Project. Stock Exchange

---
> #### 이 프로젝트는 주식 거래 시뮬레이터에서 거래소 역할을 하는 서버입니다.
> #### 클라이언트는 이 서버에 연결하여 주식 거래를 수행할 수 있습니다.

---
## 기능
- [x] 유저(브로커) 등록 및 인증
- [ ] 특정 티커의 과거 차트 데이터 가져오기
- [ ] 실시간 호가 / 거래(시세) 데이터 가져오기
- [x] 특정 티커에 대한 매수/매도 주문 생성 및 취소
- [ ] 매수/매도 주문 매칭 및 체결
- [ ] ~~유저(브로커)별 잔고 및 보유 주식 관리 (* 이 기능은 클라이언트에서 구현할 수도 있습니다.)~~
- [ ] 거래 내역(원시 데이터) 기록 및 조회
- [x] 관리자 기능 (유저(브로커) 관리, 심볼 관리 등)
---
## .env 파일 설정
<details>
<summary>펼쳐보기</summary>

```
# Swagger API 문서 접근용 계정
SWAGGER_USER=
SWAGGER_PASSWORD=
# PostgreSQL DB 설정
POSTGRESQL_DB_HOST=localhost
POSTGRESQL_DB_PORT=5432
POSTGRESQL_DB_USER=postgres
POSTGRESQL_DB_PASSWORD=pjse-user-1234
POSTGRESQL_DB_NAME=exchange-data
POSTGRESQL_DB_SSLMODE=disable
POSTGRESQL_DB_MAX_CONNS=30
POSTGRESQL_DB_MIN_CONNS=10
POSTGRESQL_DB_CONN_MAX_LIFETIME=3600
POSTGRESQL_DB_CONN_MAX_IDLE_TIME=1800
# Redis 설정
REDIS_HOST=localhost:6379
REDIS_USERNAME=pjse
REDIS_PASSWORD=pjse-user-1234
REDIS_DB=0
REDIS_POOL_SIZE=20
REDIS_MIN_IDLE_CONNS=10
REDIS_MAX_RETRIES=3
REDIS_DIAL_TIMEOUT=5
REDIS_READ_TIMEOUT=3
REDIS_WRITE_TIMEOUT=3
REDIS_POOL_TIMEOUT=4
# 웹소켓 설정
WEBSOCKET_PORT=4001
# 서버 설정
SERVER_PORT=4000
SYS_LOG=true
SYS_LOG_LOCATION=./logs
SYS_LOG_RESET_DAYS=7
SYS_LOG_LEVEL=info
```

</details>

---
## 기술 스택
- Go (Golang)
- Fiber (웹 프레임워크)
- WebSocket (실시간 통신)
- Protobuf (데이터 직렬화)
- PostgreSQL (데이터베이스)
- TimeScaleDB (시계열 데이터베이스)
- Redis (인메모리 데이터베이스)
