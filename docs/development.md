# Development guide

[Back to README](../README.md)

## Architecture

- **API:** Go, Gin, and WebSockets. Primary entrypoint: `cmd/openveth-api/main.go`.
- **Orchestration:** Docker container lifecycle plus Linux netlink operations for veth pairs, bridges, and namespaces.
- **Persistence:** SQLite by default, with PostgreSQL support. Persistent topology and ephemeral runtime information are stored separately.
- **Frontend:** Angular 21 standalone components, NgRx SignalStore, Cytoscape.js, Tailwind CSS, and xterm.js.

The API merges stored topology with runtime container IDs, PIDs, and service ports when responding. Docker events and network events update the UI. Lab activation rebuilds containers and links, restores saved network settings, and reports completion asynchronously. A status endpoint allows the frontend to recover a missed activation result after reconnecting.

Nodes retain Docker management connectivity through `docker0`, leaving `eth0` available as the first lab interface. Switches use a Linux bridge; hubs disable MAC learning. Cloud nodes enable forwarding and masquerade, but the lab's addresses and routes still need configuration.

## Native development

Requires Linux/WSL2, Go 1.23+, a C toolchain for SQLite/CGO, Node.js 22+, npm, and access to a Linux Docker daemon. The backend needs privileges for namespace and netlink operations; an ordinary unprivileged process is not sufficient.

The frontend declares `npm@11.6.2` as its package manager. Verify the Node/npm versions used by your development environment.

Prepare the node images before running a lab:

```bash
make pull-images
```

Run the API in a suitably privileged Linux environment:

```bash
CGO_ENABLED=1 make run-api
```

Run the frontend in another terminal:

```bash
cd frontend
npm ci
npm start
```

The development frontend listens on `http://localhost:4200` and its development configuration targets the API at `http://localhost:8080/api/v1`.

### Development container

If you need a prepared Linux environment with Docker and namespace access:

```bash
make dev-env
docker exec -it openveth-dev bash
```

Inside the container:

```bash
CGO_ENABLED=1 go run ./cmd/openveth-api
```

Run the frontend on the host as shown above. Stop the development container with `make dev-down`.

## Configuration

See [`internal/config/config.go`](../internal/config/config.go) for the complete configuration and validation.

| Variable | Default | Meaning |
|----------|---------|---------|
| `SERVER_ADDRESS` | `:8080` | API listen address |
| `DB_DRIVER` | `sqlite` | `sqlite`, `postgres`, or explicitly temporary `memory` storage |
| `DB_DSN` | `openveth.db` | Database connection string |
| `AUTO_SAVE_INTERVAL` | `30` | Network state autosave interval in seconds |
| `SERVER_SHUTDOWN_TIMEOUT` | `10` | Graceful shutdown timeout in seconds |
| `DOCKER_EXEC_TIMEOUT` | `5` | Docker command execution timeout in seconds |
| `DOCKER_CLEANUP_TIMEOUT` | `5` | Docker cleanup timeout in seconds |

The API stops at startup if its database cannot be initialized. `DB_DRIVER=memory` deliberately loses all configuration when the process exits. The Compose stack uses SQLite at `/app/data/openveth.db` in its `data` volume.

## Checks

Backend checks must run on Linux with CGO enabled:

```bash
go fmt ./...
CGO_ENABLED=1 go test -race ./...
```

Docker integration tests skip when no daemon is available. Namespace tests skip without the necessary capabilities. To require those tests:

```bash
sudo env PATH="$PATH" REQUIRE_NETWORK_TESTS=1 go test ./internal/orchestrator -run '^TestNetwork' -count=1
```

Frontend checks, from `frontend/`:

```bash
npm ci
npm run build
npm test
npx playwright install --with-deps chromium
npm run e2e
```

The production build downloads Google Fonts for inlining and needs network access. Browser tests use mocked API/WebSocket responses; passing them does not verify real Docker or netlink behavior. Use Linux integration tests and actual lab exercises for those paths.

Launcher checks, from the repository root:

```bash
make test-startup
```

Windows launcher checks:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/start-windows.tests.ps1
```

## Operational behavior

Topology mutations return HTTP 409 while a conflicting laboratory operation holds the operation lock. Failed Docker reads preserve saved network configuration. Partial activation reports `lab:activation_failed` with a reason, and cleanup retains nodes whose removal failed so the operation can be retried.

Activation status is held in backend memory and identifies the latest operation. After a backend restart or when a matching result is unavailable, the frontend must not infer successful completion from container presence alone.

Filesystem snapshots are local Docker images. They are separate from the topology database, and neither database persistence nor YAML export alone is a complete backup of a lab's service data.
