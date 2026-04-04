# OpenVeth Makefile

# Variables
GO_CMD=go
DOCKER_CMD=docker
COMPOSE_CMD=docker compose
FRONTEND_DIR=frontend
SCRIPTS_DIR=scripts

.PHONY: all up down pull-images images dev-env dev-down run-api run-ui deps-go deps-ui test-go fmt clean nuke reset-ports help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

all: help

# --- Quick Start (Docker) ---
up: pull-images ## Start OpenVeth (auto-detects available ports)
	@$(SCRIPTS_DIR)/find-ports.sh
	@echo ""
	@$(COMPOSE_CMD) up -d --build
	@echo ""
	@echo "========================================"
	@echo "  OpenVeth is running!"
	@echo "  Frontend: http://localhost:$$(grep FRONTEND_PORT .env | cut -d= -f2)"
	@echo "  Backend:  http://localhost:$$(grep BACKEND_PORT .env | cut -d= -f2)"
	@echo "========================================"

down: ## Stop OpenVeth
	$(COMPOSE_CMD) down

logs: ## Show logs from all services
	$(COMPOSE_CMD) logs -f

status: ## Show container status and ports
	@echo "Current configuration (.env):"
	@cat .env 2>/dev/null || echo "No .env file found"
	@echo ""
	@echo "Container status:"
	@docker ps --filter "name=openveth" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# --- Development (Docker) ---
dev-env: ## Start development container (for Linux networking)
	$(COMPOSE_CMD) -f docker-compose.dev.yml up -d --build

dev-down: ## Stop development container
	$(COMPOSE_CMD) -f docker-compose.dev.yml down

# --- Development (Native) ---
run-api: deps-go ## Run API server natively (requires Go + Linux)
	$(GO_CMD) run cmd/openveth-api/main.go

run-ui: ## Run frontend dev server natively (requires Node)
	cd $(FRONTEND_DIR) && npm start

deps-go: ## Install Go dependencies
	$(GO_CMD) mod tidy

deps-ui: ## Install frontend dependencies
	cd $(FRONTEND_DIR) && npm install

# --- Node Images ---
pull-images: ## Pull node images from Docker Hub (openveth/*)
	$(DOCKER_CMD) pull openveth/host:latest
	$(DOCKER_CMD) pull openveth/router:latest
	$(DOCKER_CMD) pull openveth/server:latest
	$(DOCKER_CMD) pull openveth/monitor:latest
	$(DOCKER_CMD) pull openveth/tester:latest
	$(DOCKER_CMD) pull openveth/switch:latest

images: ## Build node images locally (Host, Router/Debian+FRR, Server, Monitor, Tester, Switch)
	$(DOCKER_CMD) build -t openveth/base:latest ./images/base-node
	$(DOCKER_CMD) build -t openveth/host:latest ./images/host-node
	$(DOCKER_CMD) build -t openveth/router:latest ./images/router-node
	$(DOCKER_CMD) build -t openveth/server:latest ./images/server-node
	$(DOCKER_CMD) build -t openveth/monitor:latest ./images/monitor-node
	$(DOCKER_CMD) build -t openveth/tester:latest ./images/tester-node
	$(DOCKER_CMD) build -t openveth/switch:latest ./images/switch-node

switch-image: ## Build only the switch node image (Debian + SNMP + Bridge tools)
	$(DOCKER_CMD) build -t openveth/switch:latest ./images/switch-node

router-image: ## Build only the router node image (Debian + FRRouting + snmpd)
	$(DOCKER_CMD) build -t openveth/router:latest ./images/router-node

monitor-image: ## Build only the monitor node image (Grafana + Prometheus + snmp_exporter)
	$(DOCKER_CMD) build -t openveth/monitor:latest ./images/monitor-node

tester-image: ## Build only the tester node image (wrk, k6, siege, iperf3, locust...)
	$(DOCKER_CMD) build -t openveth/tester:latest ./images/tester-node

# --- Testing ---
test-go: ## Run Go tests
	$(GO_CMD) test ./...

# --- Utilities ---
fmt: ## Format Go source code
	$(GO_CMD) fmt ./...
	@echo "Go code formatted."

clean: ## Free disk space: remove all OpenVeth images, snapshots and build cache
	-$(DOCKER_CMD) images --format '{{.ID}}' --filter=reference='openveth/*' | xargs -r $(DOCKER_CMD) rmi -f
	-$(DOCKER_CMD) images --format '{{.ID}}' --filter=reference='openveth-snapshot:*' | xargs -r $(DOCKER_CMD) rmi -f
	-$(DOCKER_CMD) builder prune -a -f
	@echo "Disk space freed. Run 'make up' to re-download images."

reset-ports: ## Delete .env to detect new ports on next 'make up'
	rm -f .env
	@echo "Port configuration cleared. Run 'make up' to detect new ports."

nuke: clean ## NUCLEAR: Remove ALL OpenVeth containers, images and database
	@echo "Destroying openveth environment..."
	-$(COMPOSE_CMD) down -v
	-$(DOCKER_CMD) ps -a -q --filter label=openveth=true | xargs -r $(DOCKER_CMD) rm -f
	-rm -f openveth.db .env
	@echo "Environment reset complete."
