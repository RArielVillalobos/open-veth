# OpenVeth Makefile

# Variables
GO_CMD=go
DOCKER_CMD=docker
COMPOSE_CMD=docker compose
FRONTEND_DIR=frontend
SCRIPTS_DIR=scripts

.PHONY: all up down images dev-env dev-down run-api run-ui deps-go deps-ui test-go fmt clean nuke help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

all: help

# --- Quick Start (Docker) ---
up: images ## Start OpenVeth (auto-detects available ports)
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
images: ## Build node images (Host, Router/Debian+FRR, Linux, Server, Monitor, Tester)
	$(DOCKER_CMD) build -t openveth/host:latest ./images/host-node
	$(DOCKER_CMD) build -t openveth/router:latest ./images/router-node
	$(DOCKER_CMD) build -t openveth/linux:latest ./images/linux-node
	$(DOCKER_CMD) build -t openveth/server:latest ./images/server-node
	$(DOCKER_CMD) build -t openveth/monitor:latest ./images/monitor-node
	$(DOCKER_CMD) build -t openveth/tester:latest ./images/tester-node

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

clean: down dev-down ## Stop everything and clean artifacts
	cd $(FRONTEND_DIR) && rm -rf node_modules .angular
	$(GO_CMD) clean
	@echo "Cleanup completed."

reset-ports: ## Delete .env to detect new ports on next 'make up'
	rm -f .env
	@echo "Port configuration cleared. Run 'make up' to detect new ports."

nuke: ## NUCLEAR: Remove ALL OpenVeth containers and database
	@echo "Destroying openveth environment..."
	-$(COMPOSE_CMD) down -v
	-$(DOCKER_CMD) ps -a -q --filter label=openveth=true | xargs -r $(DOCKER_CMD) rm -f
	-rm -f openveth.db .env
	@echo "Environment reset complete."
