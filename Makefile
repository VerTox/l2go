# L2Go Development Makefile

.PHONY: help \
        db-up db-down db-restart db-logs db-clean adminer-up \
        build build-loginserver build-gameserver build-stressbot build-seedbots \
        run-loginserver run-gameserver \
        deps test clean \
        up down build-images logs logs-login logs-game restart-login restart-game \
        stress stress-snap seed-bots check

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Docker Compose v2 (`docker compose`, not the legacy `docker-compose` binary).
COMPOSE := docker compose

help: ## Show this help message
	@echo "L2Go Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ── Database (PostgreSQL only) ──────────────────────────────
db-up: ## Start PostgreSQL only
	$(COMPOSE) up -d postgres
	@echo "PostgreSQL at localhost:5432 (postgres/postgres). DBs: l2go_login + l2go_gameserver (created by init-db.sql)."

db-down: ## Stop PostgreSQL
	$(COMPOSE) stop postgres

db-restart: ## Restart PostgreSQL
	$(COMPOSE) restart postgres

db-logs: ## Tail PostgreSQL logs
	$(COMPOSE) logs -f postgres

db-clean: ## Remove ALL stack data volumes (DESTRUCTIVE!)
	$(COMPOSE) down -v

adminer-up: ## Start Adminer DB web UI (http://localhost:8080)
	$(COMPOSE) up -d adminer

# ── Build native binaries ───────────────────────────────────
build: build-loginserver build-gameserver ## Build both server binaries

build-loginserver: ## Build the LoginServer binary
	cd cmd/loginserver && go build -o loginserver .

build-gameserver: ## Build the GameServer binary
	cd cmd/gameserver && go build -o gameserver .

build-stressbot: ## Build the stress-test bot
	go build -o stressbot ./cmd/stressbot

build-seedbots: ## Build the character seeder
	go build -o seedbots ./cmd/seedbots

# ── Run natively (require PostgreSQL, e.g. `make db-up`) ─────
run-loginserver: build-loginserver ## Run the LoginServer (needs PostgreSQL)
	cd cmd/loginserver && ./loginserver

run-gameserver: build-gameserver ## Run the GameServer (needs PostgreSQL + LoginServer; loads datapack/)
	cd cmd/gameserver && ./gameserver

# ── Go module hygiene ───────────────────────────────────────
deps: ## Download and tidy dependencies
	go mod download
	go mod tidy

test: ## Run the test suite
	go test ./...

clean: ## Remove built binaries
	rm -f stressbot seedbots cmd/loginserver/loginserver cmd/gameserver/gameserver

# ── Full Docker stack ───────────────────────────────────────
# Services: postgres, loginserver, gameserver, adminer, prometheus, grafana.
up: ## Start the full stack in the background
	$(COMPOSE) up -d

down: ## Stop the full stack
	$(COMPOSE) down

build-images: ## (Re)build the LoginServer + GameServer images
	$(COMPOSE) build loginserver gameserver

logs: ## Tail logs for all services
	$(COMPOSE) logs -f

logs-login: ## Tail LoginServer logs
	$(COMPOSE) logs -f loginserver

logs-game: ## Tail GameServer logs
	$(COMPOSE) logs -f gameserver

restart-login: ## Rebuild + restart the LoginServer container
	$(COMPOSE) up -d --build loginserver

restart-game: ## Rebuild + restart the GameServer container
	$(COMPOSE) up -d --build gameserver

# ── Load / stress testing (Go cmd/stressbot — canonical) ────
# N bots online. Needs the stack up and seeded characters (see seed-bots);
# accounts auto-create on the LoginServer (AUTO_CREATE_ACCOUNTS=true).
N ?= 1000
stress: ## Ramp N bots into the world and hold (N=1000; override: make stress N=500)
	go run ./cmd/stressbot -n $(N) -enter -hold 0

stress-snap: ## Print the Prometheus tick-health summary and exit (no fleet run)
	go run ./cmd/stressbot -promsnap

seed-bots: ## Seed stress-test characters into the DB (pass flags via ARGS="-n 1000 ...")
	go run ./cmd/seedbots $(ARGS)

# ── In-game acceptance harness (headless l2client) ──────────
check: ## Run the headless l2client acceptance check against the running stack
	node references/l2client/check.js
