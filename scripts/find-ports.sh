#!/usr/bin/env bash
# Detect host TCP ports without evaluating .env as shell code.
set -euo pipefail

declare -A PORT_OWNERS=()

require_port_tools() {
    local tool
    for tool in docker ss awk mktemp; do
        command -v "$tool" >/dev/null 2>&1 || { echo "ERROR: Required command not found: $tool (ss is provided by iproute2)." >&2; return 1; }
    done
}

get_docker_port_owners() {
    local repository=$1 ids id details name service directory owner port container_port
    local template='{{.Name}}|{{index .Config.Labels "com.docker.compose.service"}}|{{index .Config.Labels "com.docker.compose.project.working_dir"}}
{{range $port, $bindings := .NetworkSettings.Ports}}{{range $bindings}}{{printf "%s|%s\n" .HostPort $port}}{{end}}{{end}}'
    PORT_OWNERS=()
    ids=$(docker ps -q) || return 1
    for id in $ids; do
        details=$(docker inspect --format "$template" "$id") || return 1
        IFS='|' read -r name service directory <<< "$details"
        name=${name#/}
        owner=other
        if [[ $name == openveth-backend || $name == openveth-frontend ]]; then
            if [[ $name != "openveth-$service" || ${directory%/} != "${repository%/}" ]]; then
                echo "ERROR: Container $name belongs to another project. Resolve its name conflict before starting." >&2
                return 1
            fi
            owner=$service
        fi
        while IFS='|' read -r port container_port; do
            [[ $container_port == */tcp && $port =~ ^[0-9]+$ ]] || continue
            if [[ -v PORT_OWNERS[$port] && ${PORT_OWNERS[$port]} != "$owner" ]]; then
                PORT_OWNERS[$port]=other
            else
                PORT_OWNERS[$port]=$owner
            fi
        done < <(printf '%s\n' "$details" | tail -n +2)
    done
}

check_port() {
    local port=$1 service=$2 listeners
    if [[ -v PORT_OWNERS[$port] ]]; then
        [[ ${PORT_OWNERS[$port]} == "$service" ]]
        return
    fi
    # No -p: process visibility and root permissions must not affect detection.
    listeners=$(ss -H -ltn "sport = :$port") || { echo "ERROR: Could not inspect TCP port $port." >&2; return 2; }
    [[ -z $listeners ]]
}

find_available_port() {
    local preferred=$1 first=$2 last=$3 service=$4 reserved=${5:-0} port status
    local -a candidates=("$preferred")
    for ((port=first; port<=last; port++)); do candidates+=("$port"); done
    for port in "${candidates[@]}"; do
        [[ $port != "$reserved" ]] || continue
        if check_port "$port" "$service"; then
            printf '%s\n' "$port"
            return 0
        else
            status=$?
            [[ $status == 1 ]] || return "$status"
        fi
    done
    echo "ERROR: No free port for $service. Free port $preferred or one in $first-$last." >&2
    return 1
}

parse_port() {
    local value=$1 key=$2
    local pattern="^[\"']?([0-9]+)[\"']?[[:space:]]*(#.*)?$"
    if [[ $value =~ $pattern && ${#BASH_REMATCH[1]} -le 5 ]]; then
        value=$((10#${BASH_REMATCH[1]}))
        if ((value >= 1 && value <= 65535)); then printf '%s\n' "$value"; return 0; fi
    fi
    echo "ERROR: Invalid $key: expected a port between 1 and 65535." >&2
    return 1
}

configure_ports() {
    local env_file=$1 repository=$2 line key value temporary
    local frontend=80 backend=8080
    if [[ -f $env_file ]]; then
        while IFS= read -r line || [[ -n $line ]]; do
            line=${line%$'\r'}
            if [[ $line =~ ^[[:space:]]*(export[[:space:]]+)?(FRONTEND_PORT|BACKEND_PORT)[[:space:]]*=[[:space:]]*(.*)$ ]]; then
                key=${BASH_REMATCH[2]}; value=${BASH_REMATCH[3]}
                value=$(parse_port "$value" "$key") || return 1
                if [[ $key == FRONTEND_PORT ]]; then frontend=$value; else backend=$value; fi
            fi
        done < "$env_file"
    fi
    # Match Compose precedence: exported variables override the dotenv file.
    if [[ -n ${FRONTEND_PORT:-} ]]; then frontend=$(parse_port "$FRONTEND_PORT" FRONTEND_PORT) || return 1; fi
    if [[ -n ${BACKEND_PORT:-} ]]; then backend=$(parse_port "$BACKEND_PORT" BACKEND_PORT) || return 1; fi
    get_docker_port_owners "$repository" || return 1
    SELECTED_FRONTEND_PORT=$(find_available_port "$frontend" 8080 8100 frontend) || return 1
    SELECTED_BACKEND_PORT=$(find_available_port "$backend" 8081 8200 backend "$SELECTED_FRONTEND_PORT") || return 1

    temporary=$(mktemp "${env_file}.tmp.XXXXXX") || return 1
    local input=/dev/null
    [[ ! -f $env_file ]] || input=$env_file
    if ! awk -v frontend="$SELECTED_FRONTEND_PORT" -v backend="$SELECTED_BACKEND_PORT" '
        /^[[:space:]]*(export[[:space:]]+)?FRONTEND_PORT[[:space:]]*=/ { if (!front++) print "FRONTEND_PORT=" frontend; next }
        /^[[:space:]]*(export[[:space:]]+)?BACKEND_PORT[[:space:]]*=/ { if (!back++) print "BACKEND_PORT=" backend; next }
        { print }
        END { if (!front) print "FRONTEND_PORT=" frontend; if (!back) print "BACKEND_PORT=" backend }
    ' "$input" > "$temporary"; then rm -f -- "$temporary"; return 1; fi
    if [[ -f $env_file ]] && ! chmod --reference="$env_file" "$temporary"; then rm -f -- "$temporary"; return 1; fi
    if ! mv -f -- "$temporary" "$env_file"; then rm -f -- "$temporary"; return 1; fi
    echo "FRONTEND_PORT=$SELECTED_FRONTEND_PORT"
    echo "BACKEND_PORT=$SELECTED_BACKEND_PORT"
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
    repository=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
    require_port_tools
    configure_ports "$repository/.env" "$repository"
fi
