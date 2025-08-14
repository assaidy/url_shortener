include .env

DB_STRING=host=$(PG_HOST) port=$(PG_PORT) user=$(PG_USER) password=$(PG_PASSWORD) dbname=$(PG_NAME) sslmode=$(PG_SSL_MODE)
GOOSE_ENV=GOOSE_DRIVER="postgres" GOOSE_DBSTRING="$(DB_STRING)" GOOSE_MIGRATION_DIR="./db/postgres/migrations/"

all: build

run: build
	@./bin/app

build:
	@go mod tidy
	@go build -o ./bin/app ./cmd/main.go

clean:
	@rm -rf ./bin/

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

sqlc:
	@sqlc generate

psql:
	@PGPASSWORD=$(PG_PASSWORD) psql -h $(PG_HOST) -p $(PG_PORT) -U $(PG_USER) -d $(PG_NAME)
