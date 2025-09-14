# Project. Stock Exchange

---
> #### 이 프로젝트는 주식 거래 시뮬레이터에서 거래소 역할을 하는 서버입니다.
> #### 클라이언트는 이 서버에 연결하여 주식 거래를 수행할 수 있습니다.

---
## 기능
- [x] 유저(브로커) 등록 및 인증
- [ ] 특정 티커의 과거 차트 데이터 가져오기
- [ ] 특정 티커의 실시간 호가 / 거래(시세) 데이터 가져오기
- [ ] 특정 티커에 대한 매수/매도 주문 생성 및 취소
- [ ] 매수/매도 주문 매칭 및 체결
- [ ] 유저(브로커)별 잔고 및 보유 주식 관리 (* 이 기능은 클라이언트에서 구현할 수도 있습니다.)
- [ ] 거래 내역(원시 데이터) 기록 및 조회
- [x] 관리자 기능 (유저(브로커) 관리, 심볼 관리 등)
---
## .env 파일 설정
```
SALT=
# Swagger API 문서 접근용 계정
SWAGGER_USER=
SWAGGER_PASSWORD=
# ACID 중요 데이터 보관용 PostgreSQL DB 설정
POSTGRES_DB_HOST=localhost
POSTGRES_DB_PORT=5432
POSTGRES_DB_USER=postgres
POSTGRES_DB_PASSWORD=1234
POSTGRES_DB_NAME=exchange_data
POSTGRES_DB_SSLMODE=disable
```
---
## 기술 스택
- Go (Golang)
- Fiber (웹 프레임워크)
- PostgreSQL (데이터베이스)
- Redis (인메모리 데이터 저장소)