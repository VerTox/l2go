# L2Go Development Makefile

.PHONY: help db-up db-down db-restart db-logs db-clean build run-login run-game test js-client run-e2e stop-server show-log

help: ## Show this help message
	@echo "L2Go Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Shell and logging setup for orchestration
SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.ONESHELL:

LOG_DIR := .logs
TIME ?= 20
LOG_FILE := $(LOG_DIR)/run-$(shell date +%Y%m%d-%H%M%S).log

# Database Commands
db-up: ## Start PostgreSQL database
	docker-compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	@echo "PostgreSQL is ready at localhost:5432"
	@echo "Database: l2go_login"
	@echo "User: postgres"
	@echo "Password: postgres"

db-down: ## Stop PostgreSQL database
	docker-compose down

db-restart: ## Restart PostgreSQL database
	docker-compose restart postgres

db-logs: ## Show PostgreSQL logs
	docker-compose logs -f postgres

db-clean: ## Remove PostgreSQL data (DESTRUCTIVE!)
	docker-compose down -v
	docker volume rm l2go_postgres_data 2>/dev/null || true

adminer-up: ## Start Adminer (Database Web UI)
	docker-compose up -d adminer
	@echo "Adminer available at http://localhost:8080"
	@echo "Server: postgres"
	@echo "Username: postgres"
	@echo "Password: postgres"
	@echo "Database: l2go_login"

# Application Commands
build: ## Build the L2Go server (legacy)
	go build -o l2go .

build-loginserver: ## Build the LoginServer binary
	cd cmd/loginserver && go build -o loginserver .

run-login: build ## Run the login server (legacy, requires PostgreSQL)
	./l2go -mode=0

run-loginserver: build-loginserver ## Run the new LoginServer (requires PostgreSQL)
	cd cmd/loginserver && ./loginserver

run-game: build ## Run the game server
	./l2go -mode=1 -server=1

# Development Commands
deps: ## Download and tidy dependencies
	go mod download
	go mod tidy

test: ## Run tests
	go test ./...

clean: ## Clean build artifacts
	rm -f l2go

# Docker Commands
docker-up: ## Start all services (PostgreSQL + LoginServer + Adminer)
	docker-compose up -d

docker-down: ## Stop all services
	docker-compose down

docker-logs: ## Show logs for all services
	docker-compose logs -f

docker-loginserver-logs: ## Show LoginServer logs only
	docker-compose logs -f loginserver

docker-build: ## Build LoginServer Docker image
	docker-compose build loginserver

docker-restart-loginserver: ## Restart LoginServer container
	docker-compose restart loginserver

# Load Testing Commands
loadtest-setup: ## Setup load testing dependencies
	cd loadtest && npm install

loadtest-simple: ## Run simple load test (100 clients, login-only)
	cd loadtest && node login-only-test.js --clients=100

loadtest-medium: ## Run medium load test (1000 clients, login-only)
	cd loadtest && node login-only-test.js --clients=1000 --concurrency=50

loadtest-large: ## Run large load test (5000 clients, login-only)
	cd loadtest && node login-only-test.js --clients=5000 --concurrency=100

loadtest-custom: ## Run custom load test (use CLIENTS=N CONCURRENCY=M)
	cd loadtest && node login-only-test.js --clients=$(or $(CLIENTS),1000) --concurrency=$(or $(CONCURRENCY),50) --verbose

loadtest-vs-docker: ## Test against Docker LoginServer
	cd loadtest && node login-only-test.js --clients=1000 --host=localhost --port=2106

loadtest-full: ## Run full client test (with GameServer, requires l2jsclient)
	cd loadtest && node --expose-gc loadtest.js --clients=$(or $(CLIENTS),100) --concurrency=$(or $(CONCURRENCY),10)

loadtest-full-small: ## Run small full client test (10 clients)
	cd loadtest && node --expose-gc loadtest.js --clients=10 --concurrency=5 --verbose

loadtest-full-medium: ## Run medium full client test (100 clients)
	cd loadtest && node --expose-gc loadtest.js --clients=100 --concurrency=10

loadtest-global: ## Run load test with global event handlers (no memory leaks)
	cd loadtest && node loadtest-global.js --clients=$(or $(CLIENTS),1000) --concurrency=$(or $(CONCURRENCY),50)

loadtest-global-large: ## Run large load test with global handlers (5000 clients)
	cd loadtest && node loadtest-global.js --clients=5000 --concurrency=100

# Native Go Client Load Tests
loadtest-go: ## Run Go native client load test
	cd loadtest/go-client && go run main.go -clients=$(or $(CLIENTS),100) -concurrency=$(or $(CONCURRENCY),50)

loadtest-go-solo: ## Run small Go client test (10 clients)
	cd loadtest/go-client && go run main.go -clients=1 -concurrency=1

loadtest-go-small: ## Run small Go client test (10 clients)
	cd loadtest/go-client && go run main.go -clients=4 -concurrency=1 -verbose

loadtest-go-medium: ## Run medium Go client test (1000 clients)
	cd loadtest/go-client && go run main.go -clients=1000 -concurrency=100

loadtest-go-large: ## Run large Go client test (5000 clients)
	cd loadtest/go-client && go run main.go -clients=5000 -concurrency=200

loadtest-go-build: ## Build Go client binary
	cd loadtest/go-client && go build -o loadtest-client main.go

js-client:
	node ../../WebstormProjects/l2client/main.js

# Orchestrated end-to-end run: start server, run JS inclient, then stop server
run-e2e: build
	@mkdir -p $(LOG_DIR)
	@echo "==> Starting server" | tee -a $(LOG_FILE)
	@./l2go -mode=0 >> $(LOG_FILE) 2>&1 & echo $$! > .server.pid
	@echo "Server PID: $$(cat .server.pid)" | tee -a $(LOG_FILE)
	@trap 'echo "==> Interrupt received, stopping server" | tee -a $(LOG_FILE); $(MAKE) --no-print-directory stop-server' INT TERM EXIT; \
	echo "==> Waiting 1s for server to warm up" | tee -a $(LOG_FILE); \
	sleep 1; \
	echo "==> Starting JS client" | tee -a $(LOG_FILE); \
	if node ../../WebstormProjects/l2client/main.js >> $(LOG_FILE) 2>&1; then \
		echo "==> Client finished (or disconnected)" | tee -a $(LOG_FILE); \
	else \
		echo "==> Client exited with error" | tee -a $(LOG_FILE); \
	fi; \
	echo "==> Client is done; stopping server now" | tee -a $(LOG_FILE); \
	$(MAKE) --no-print-directory stop-server; \
	trap - INT TERM EXIT

# Graceful server stop using stored PID
stop-server:
	@if [ -f .server.pid ]; then \
		PID=$$(cat .server.pid); \
		kill $$PID >/dev/null 2>&1 || true; \
		for i in $$(seq 1 10); do \
			if kill -0 $$PID >/dev/null 2>&1; then sleep 0.5; else break; fi; \
		done; \
		kill -9 $$PID >/dev/null 2>&1 || true; \
		rm -f .server.pid; \
	else \
		echo "No .server.pid file; server not running?"; \
	fi

# Quick look at the latest combined log
show-log:
	@latest=$$(ls -1t $(LOG_DIR)/*.log 2>/dev/null | head -n 1); \
	if [ -n "$$latest" ]; then \
		echo "Log file: $$latest"; \
		tail -n 100 "$$latest" || true; \
	else \
		echo "No log files found in $(LOG_DIR)"; \
	fi