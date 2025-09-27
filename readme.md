# Project. Stock Exchange - Simple

---
> #### 이 프로젝트는 주식 거래 시뮬레이터에서 거래소 역할을 하는 서버입니다.
> #### 클라이언트는 이 서버에 연결하여 주식 거래를 수행할 수 있습니다.

---
## 기능
- [x] 일일 실시간 호가 / 거래(시세) 데이터 가져오기 및 구독
- [x] 특정 티커에 대한 매수/매도 주문 생성 및 취소
- [x] 매수/매도 주문 매칭 및 체결
---
## .env 파일 설정
<details>
<summary>펼쳐보기</summary>

```
# Swagger API 문서 접근용 계정
SWAGGER_USER=
SWAGGER_PASSWORD=
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
- B-Tree (데이터 구조)