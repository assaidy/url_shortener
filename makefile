include .env
MAKEFLAGS += --no-print-directory

sqlc:
	@go tool sqlc generate

build: sqlc
	@go mod tidy
	@go fmt ./...
	@go build -o ./bin/app ./cmd/app/

run: build
	@./bin/app

clean:
	@rm -rf ./bin/

_validate-goose-service:
	@if [ -z "$(service)" ]; then                                     \
		echo "error: 'service' is required.";                           \
		echo "usage: make goose-{up,down,reset,new} service=<service>"; \
		exit 1;                                                         \
	fi
_validate-goose-name:
	@if [ -z "$(name)" ]; then                                    \
		echo "error: 'name' is required for 'goose-new' command.";  \
		echo "usage: make goose-new service=<service> name=<name>"; \
		exit 1;                                                     \
	fi

GOOSE_ENV = GOOSE_DRIVER="postgres" GOOSE_DBSTRING="$(POSTGRES_URL)" GOOSE_MIGRATION_DIR="./src/services/$(service)/db/migrations/"
GOOSE_CMD = $(GOOSE_ENV) go tool goose -table="goose_db_version_$(service)"
goose-up: _validate-goose-service
	@$(GOOSE_CMD) up
goose-down: _validate-goose-service
	@$(GOOSE_CMD) down
goose-reset: _validate-goose-service
	@$(GOOSE_CMD) reset
goose-new: _validate-goose-service _validate-goose-name
	@$(GOOSE_CMD) create $(name) sql

SERVICES = $(shell ls -d ./src/services/*/ | xargs -n1 basename)
goose-up-all:
	@for service in $(SERVICES); do                                      \
		[ -d "./src/services/$$service/db/migrations" ] || continue; \
		echo "==> goose up: $$service";                                    \
		make goose-up service=$$service;                                   \
	done
goose-reset-all:
	@for service in $(SERVICES); do                                      \
		[ -d "./src/services/$$service/db/migrations" ] || continue; \
		echo "==> goose reset: $$service";                                 \
		make goose-reset service=$$service;                                \
	done

pg-cli:
	@pgcli "$(POSTGRES_URL)"

redis-cli:
	@redis-cli -p "$(REDIS_PORT)"
