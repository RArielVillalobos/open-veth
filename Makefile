# OpenVeth Makefile

# Variables
GO_CMD=go
DOCKER_CMD=docker
COMPOSE_CMD=docker compose
FRONTEND_DIR=frontend

.PHONY: all images dev-env dev-down run-api run-ui deps-go deps-ui clean help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%%-20s\033[0m %s\n", $$1, $$2}'

all: help

# --- Infrastructure ---
dev-env: ## Start development environment (Docker Compose)
	$(COMPOSE_CMD) -f docker-compose.dev.yml up -d --build

dev-down: ## Stop development environment
	$(COMPOSE_CMD) -f docker-compose.dev.yml down

# --- Node Images ---
images: ## Build base images (Host and Router)
	$(DOCKER_CMD) build -t openveth/host:latest ./images/host-node
	$(DOCKER_CMD) build -t openveth/router:latest ./images/router-node

# --- Backend (Go) ---
deps-go: ## Install Go dependencies
	$(GO_CMD) mod tidy

run-api: deps-go ## Run API server (Backend)
	$(GO_CMD) run cmd/openveth-api/main.go

test-go: ## Run Go tests
	$(GO_CMD) test ./...

# --- Frontend (Angular) ---
deps-ui: ## Install Frontend dependencies
	cd $(FRONTEND_DIR) && npm install

run-ui: ## Run Frontend in development mode
	cd $(FRONTEND_DIR) && npm start

# --- Utilities ---
clean: dev-down ## Clean containers and artifacts
	cd $(FRONTEND_DIR) && rm -rf node_modules .angular
	$(GO_CMD) clean
	@echo "Cleanup completed."
