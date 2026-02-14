#!/bin/bash
# Find available ports for OpenVeth and save to .env

ENV_FILE=".env"

# Check if openveth containers are already running
openveth_running() {
    docker ps --format '{{.Names}}' | grep -q "openveth"
}

# Check if a port is available (not used by other services)
check_port() {
    local port=$1

    # Check if port is used by other Docker containers (not openveth)
    if docker ps --format '{{.Names}} {{.Ports}}' | grep -v "openveth" | grep -q "0.0.0.0:${port}->"; then
        return 1  # Port is in use by another container
    fi

    # Check if port is in use by non-Docker services
    # We check if something is listening AND it's not from Docker
    local pid=$(ss -tulnp 2>/dev/null | grep ":${port} " | grep -oP 'pid=\K[0-9]+' | head -1)
    if [ -n "$pid" ]; then
        # Check if this PID is from docker-proxy (our containers)
        local cmd=$(ps -p "$pid" -o comm= 2>/dev/null)
        if [ "$cmd" != "docker-proxy" ]; then
            return 1  # Port is in use by non-Docker service
        fi
    fi

    return 0  # Port is available
}

find_available_port() {
    local preferred=$1
    local start=${2:-$preferred}
    local end=${3:-65535}

    # Try preferred port first
    if check_port $preferred; then
        echo $preferred
        return
    fi

    # Search for available port
    for port in $(seq $start $end); do
        if check_port $port; then
            echo $port
            return
        fi
    done

    echo "0"  # No port found
}

# If openveth is already running and .env exists, just use existing config
if [ -f "$ENV_FILE" ] && openveth_running; then
    source "$ENV_FILE"
    echo "OpenVeth is running, using existing .env"
    echo "FRONTEND_PORT=$FRONTEND_PORT"
    echo "BACKEND_PORT=$BACKEND_PORT"
    exit 0
fi

# Check if .env exists and ports are available
if [ -f "$ENV_FILE" ]; then
    source "$ENV_FILE"

    if [ -n "$FRONTEND_PORT" ] && [ -n "$BACKEND_PORT" ]; then
        if check_port $FRONTEND_PORT && check_port $BACKEND_PORT; then
            echo "Using existing .env configuration"
            echo "FRONTEND_PORT=$FRONTEND_PORT"
            echo "BACKEND_PORT=$BACKEND_PORT"
            exit 0
        else
            echo "Ports from .env are no longer available, finding new ones..."
        fi
    fi
fi

# Find new available ports
FRONTEND_PORT=$(find_available_port 80 8080 8100)
BACKEND_PORT=$(find_available_port 8080 8081 8200)

# Ensure they're different
if [ "$FRONTEND_PORT" = "$BACKEND_PORT" ]; then
    BACKEND_PORT=$(find_available_port $((BACKEND_PORT + 1)) $((BACKEND_PORT + 1)) 8200)
fi

# Save to .env
cat > "$ENV_FILE" << EOF
# OpenVeth port configuration (auto-generated)
# Delete this file and run 'make up' to detect new ports
FRONTEND_PORT=$FRONTEND_PORT
BACKEND_PORT=$BACKEND_PORT
EOF

echo "Created .env with new ports"
echo "FRONTEND_PORT=$FRONTEND_PORT"
echo "BACKEND_PORT=$BACKEND_PORT"
