.PHONY: vars
vars:
	echo SCHEDULA_DATABASE_URL=$(SCHEDULA_DATABASE_URL)

SCHEDULA_DATABASE_URL ?= postgres://schedula:schedula@localhost:5433/schedula?sslmode=disable

.PHONY: gen
gen:
	go run github.com/bufbuild/buf/cmd/buf@v1.50.0 generate

.PHONY: db-up
db-up:
	docker compose -f deploy/docker-compose.yml up -d

.PHONY: db-down
db-down:
	docker compose -f deploy/docker-compose.yml down

.PHONY: db-migrate
db-migrate:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@v3.24.1 -dir migrations postgres "$(SCHEDULA_DATABASE_URL)" up

.PHONY: tidy
tidy:
	cd backend && go mod tidy

.PHONY: test
test:
	cd backend && go test ./...

.PHONY: run-server
run-server:
	cd backend && go run ./cmd/schedula-server
