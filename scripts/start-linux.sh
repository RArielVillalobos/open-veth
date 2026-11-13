#!/usr/bin/env bash
# Prepare images, then update services and verify readiness.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
REPOSITORY=$(cd -- "$SCRIPT_DIR/.." && pwd -P)
source "$SCRIPT_DIR/find-ports.sh"
read -r -a DOCKER_COMMAND <<< "${DOCKER_CMD:-docker}"
BASE_READY=false
STARTED=false

docker() { command "${DOCKER_COMMAND[@]}" "$@"; }
compose() { docker compose --project-directory "$REPOSITORY" -f "$REPOSITORY/docker-compose.yml" "$@"; }

check_docker() {
    command -v "${DOCKER_COMMAND[0]}" >/dev/null 2>&1 || { echo 'ERROR: Docker CLI was not found.' >&2; return 1; }
    local engine
    engine=$(docker info --format '{{.OSType}}') || { echo 'ERROR: Docker is unavailable. Start the daemon and check your Docker permissions.' >&2; return 1; }
    [[ $engine == linux ]] || { echo 'ERROR: OpenVeth requires a Linux Docker engine.' >&2; return 1; }
    docker compose version
}

ensure_node_image() {
    local name=$1 refresh=$2 image="openveth/$1:latest" exists=false
    if docker image inspect "$image" >/dev/null 2>&1; then exists=true; fi
    if [[ $exists == true && $refresh != true ]]; then echo "Using local image: $image"; return 0; fi
    echo "Pulling $image..."
    if docker pull "$image"; then return 0; fi
    if [[ $exists == true ]]; then
        echo "WARNING: Could not update $image; using the existing local image." >&2
        return 0
    fi
    case "$name" in
        server|monitor|storage|zabbix)
            if [[ $BASE_READY != true ]]; then
                if ! docker image inspect openveth/base:latest >/dev/null 2>&1; then
                    docker build -t openveth/base:latest ./images/base-node
                fi
                BASE_READY=true
            fi
            ;;
    esac
    echo "Pull failed; building $image locally..."
    docker build -t "$image" "./images/$name-node"
}

wait_for_app() {
    local backend=$1 frontend=$2 timeout=$3 deadline remaining request_timeout url ready
    deadline=$((SECONDS + timeout))
    while ((SECONDS < deadline)); do
        ready=true
        for url in "http://127.0.0.1:$backend/health" "http://127.0.0.1:$frontend/" "http://127.0.0.1:$frontend/api/v1/laboratories"; do
            remaining=$((deadline - SECONDS))
            if ((remaining <= 0)); then ready=false; break; fi
            request_timeout=3
            if ((remaining < request_timeout)); then request_timeout=$remaining; fi
            if ! curl --silent --fail --noproxy '*' --max-time "$request_timeout" --output /dev/null "$url"; then
                ready=false
                break
            fi
        done
        if [[ $ready == true ]]; then return 0; fi
        remaining=$((deadline - SECONDS))
        if ((remaining > 0)); then
            if ((remaining < 2)); then sleep "$remaining"; else sleep 2; fi
        fi
    done
    echo "ERROR: OpenVeth did not become ready within $timeout seconds. Containers remain available for diagnosis." >&2
    return 1
}

startup_failed() {
    local status=$?
    trap - ERR
    echo "ERROR: OpenVeth startup failed (exit $status)." >&2
    if [[ $STARTED == true ]]; then
        compose ps || true
        compose logs --tail 60 backend frontend || true
    fi
    exit "$status"
}

main() {
    local refresh=false images_only=false check_only=false timeout=${STARTUP_TIMEOUT:-180} name
    case "${UPDATE_IMAGES:-0}" in
        0) ;;
        1) refresh=true ;;
        *) echo 'ERROR: UPDATE_IMAGES must be 0 or 1.' >&2; return 1 ;;
    esac
    while (($#)); do
        case "$1" in
            --update-images) refresh=true ;;
            --images-only) images_only=true ;;
            --check) check_only=true ;;
            *) echo "ERROR: Unknown option: $1" >&2; return 1 ;;
        esac
        shift
    done
    [[ $timeout =~ ^[0-9]{1,4}$ ]] && ((10#$timeout >= 1 && 10#$timeout <= 3600)) || {
        echo 'ERROR: STARTUP_TIMEOUT must be between 1 and 3600 seconds.' >&2; return 1;
    }
    timeout=$((10#$timeout))
    trap startup_failed ERR
    cd -- "$REPOSITORY"
    check_docker
    if [[ $check_only == true ]]; then return 0; fi
    if [[ $images_only != true ]]; then
        require_port_tools
        command -v curl >/dev/null 2>&1 || { echo 'ERROR: Install curl to check service readiness.' >&2; return 1; }
        compose config --quiet
    fi
    for name in host router switch server monitor tester haproxy storage zabbix; do ensure_node_image "$name" "$refresh"; done
    if [[ $images_only == true ]]; then return 0; fi

    echo 'Building application images before changing running services...'
    compose build
    configure_ports "$REPOSITORY/.env" "$REPOSITORY"
    export FRONTEND_PORT=$SELECTED_FRONTEND_PORT BACKEND_PORT=$SELECTED_BACKEND_PORT
    STARTED=true
    compose up -d --no-build --timeout 30
    echo "Waiting for the API, frontend, and API proxy (up to $timeout seconds)..."
    wait_for_app "$BACKEND_PORT" "$FRONTEND_PORT" "$timeout"
    echo ''
    echo 'OpenVeth is ready!'
    echo "Frontend: http://localhost:$FRONTEND_PORT"
    echo "Backend:  http://localhost:$BACKEND_PORT"
    trap - ERR
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main "$@"; fi
