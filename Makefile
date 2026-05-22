.PHONY: help dev dev-build dev-down dev-logs app-logs app-shell psql mysql lint test theme-gen migrate-up migrate-down migrate-status prod-up prod-down prod-logs

DEV          := docker compose -f docker-compose.dev.yml
PROD         := docker compose -f docker-compose.yml
APP          := $(DEV) exec app
DEV_CP_URL   := postgres://dbuser:dbpass@db:5432/cp?sslmode=disable
ACTIVE_THEME := $(shell $(APP) go run ./cmd/read_theme)
THEME_TAG    := theme_$(ACTIVE_THEME)

CYAN   := [0;36m
GREEN  := [0;32m
YELLOW := [0;33m
BOLD   := [1m
NC     := [0m

help:
	@echo "$(BOLD)Usage:$(NC) make [target]"
	@echo "$(CYAN)========================================================$(NC)"
	@echo "$(GREEN)Dev environment:$(NC)"
	@echo "  $(BOLD)dev$(NC)                          Start dev stack"
	@echo "  $(BOLD)dev-build$(NC)                    Rebuild and start dev stack"
	@echo "  $(BOLD)dev-down$(NC)                     Stop dev stack"
	@echo "  $(BOLD)dev-logs$(NC)                     Tail logs (all services)"
	@echo "  $(BOLD)app-logs$(NC)                     Tail app logs only"
	@echo "  $(BOLD)app-shell$(NC)                    Shell into app container"
	@echo "  $(BOLD)psql$(NC)                         psql into Postgres (cp)"
	@echo "  $(BOLD)mysql$(NC)                        mariadb shell into rA mock (main)"
	@echo "$(CYAN)========================================================$(NC)"
	@echo "$(GREEN)Quality:$(NC)"
	@echo "  $(BOLD)lint$(NC)                         golangci-lint in container"
	@echo "  $(BOLD)test$(NC)                         go test in container"
	@echo "$(CYAN)========================================================$(NC)"
	@echo "$(GREEN)Migrations (dev):$(NC)"
	@echo "  $(BOLD)migrate-up$(NC)                   Apply migrations to cp and cp_test"
	@echo "  $(BOLD)migrate-down$(NC)                 Roll back one migration on cp"
	@echo "  $(BOLD)migrate-status$(NC)               Show migration status"
	@echo "$(CYAN)========================================================$(NC)"
	@echo "$(YELLOW)Production:$(NC)"
	@echo "  $(BOLD)prod-up$(NC)                      Start production stack"
	@echo "  $(BOLD)prod-down$(NC)                    Stop production stack"
	@echo "  $(BOLD)prod-logs$(NC)                    Tail production logs"

dev:
	$(DEV) up -d

dev-build:
	$(DEV) up -d --build

dev-down:
	$(DEV) down

dev-logs:
	$(DEV) logs -f

app-logs:
	$(DEV) logs -f app

app-shell:
	$(APP) sh

psql:
	$(DEV) exec db psql -U dbuser -d cp

mysql:
	$(DEV) exec mock-ra-db mariadb -udbuser -pdbpass main

lint:
	$(APP) go tool golangci-lint run --build-tags=$(THEME_TAG) ./...

test:
	$(APP) go test -tags $(THEME_TAG) ./...

theme-gen:
	$(APP) go run ./cmd/themegen

migrate-up:
	$(APP) sh scripts/migrate.sh

migrate-down:
	$(APP) go tool goose -dir migrations postgres "$(DEV_CP_URL)" down

migrate-status:
	$(APP) go tool goose -dir migrations postgres "$(DEV_CP_URL)" status

prod-up:
	$(PROD) up -d

prod-down:
	$(PROD) down

prod-logs:
	$(PROD) logs -f
