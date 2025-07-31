include .env

GOOSE_DRIVER=postgres
GOOSE_DBSTRING=postgresql://$(PG_USER):$(PG_PASSWORD)@$(PG_HOST):$(PG_PORT)/$(PG_NAME)?sslmode=$(PG_SSLMODE)
GOOSE_MIGRATION_DIR=./db/migrations/
GOOSE_ENV=GOOSE_DRIVER="$(GOOSE_DRIVER)" GOOSE_DBSTRING="$(GOOSE_DBSTRING)" GOOSE_MIGRATION_DIR="$(GOOSE_MIGRATION_DIR)"

all: build

run: build
	@./app

build:
	@go mod tidy
	@go build -o ./app ./cmd/main.go

clean:
	@rm ./app

test:
	@go test -v ./...

compose-up:
	@docker-compose up

compose-down:
	@docker-compose down

goose-up:
	@$(GOOSE_ENV) goose up

goose-down:
	@$(GOOSE_ENV) goose down

goose-reset:
	@$(GOOSE_ENV) goose reset

goose-migration:
	@if [ -z "$(name)" ]; then echo "ERROR: 'name' variable is required." && exit 1; fi
	@$(GOOSE_ENV) goose create -s $(name) sql
