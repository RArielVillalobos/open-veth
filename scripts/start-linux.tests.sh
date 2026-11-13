#!/usr/bin/env bash
# Isolated startup tests. Docker, ss, and curl are mocked; no daemon is touched.
set -Eeuo pipefail

SOURCE_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
TEST_PARENT=$(cd -- "${TMPDIR:-/tmp}" && pwd -P)
TEST_ROOT=$(mktemp -d "$TEST_PARENT/openveth-linux-tests.XXXXXX")
cleanup() {
    # Verify the absolute target belongs to this test before recursive cleanup.
    case "$TEST_ROOT" in "$TEST_PARENT"/openveth-linux-tests.*) rm -rf -- "$TEST_ROOT" ;; *) echo 'Unsafe test cleanup path' >&2; exit 1 ;; esac
}
trap cleanup EXIT
PROJECT="$TEST_ROOT/project with spaces"
mkdir -p "$PROJECT/scripts" "$TEST_ROOT/bin" "$TEST_ROOT/inspect"
cp "$SOURCE_DIR/start-linux.sh" "$SOURCE_DIR/find-ports.sh" "$PROJECT/scripts/"
export MOCK_LOG="$TEST_ROOT/commands" MOCK_INSPECT_DIR="$TEST_ROOT/inspect"
ORIGINAL_PATH=$PATH
export PATH="$TEST_ROOT/bin:$PATH"

cat > "$TEST_ROOT/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$MOCK_LOG"
case "$1" in
    info) echo "${MOCK_ENGINE:-linux}"; exit "${MOCK_INFO_EXIT:-0}" ;;
    image) [[ ${MOCK_ALL_LOCAL:-1} == 1 ]]; exit ;;
    pull) exit "${MOCK_PULL_EXIT:-0}" ;;
    build) exit "${MOCK_IMAGE_BUILD_EXIT:-0}" ;;
    ps) printf '%s\n' "${MOCK_CONTAINER_IDS:-}" ;;
    inspect) id=${!#}; cat "$MOCK_INSPECT_DIR/$id" ;;
    compose)
        for action in "$@"; do
            case "$action" in
                version) exit "${MOCK_COMPOSE_VERSION_EXIT:-0}" ;;
                config) exit 0 ;;
                build) exit "${MOCK_COMPOSE_BUILD_EXIT:-0}" ;;
                up) exit "${MOCK_UP_EXIT:-0}" ;;
                ps|logs) exit 0 ;;
            esac
        done
        exit 1 ;;
    *) exit 1 ;;
esac
MOCK
cat > "$TEST_ROOT/bin/ss" <<'MOCK'
#!/usr/bin/env bash
set -eu
[[ ${MOCK_SS_EXIT:-0} == 0 ]] || exit 1
filter=${!#}; port=${filter##*:}
if [[ ${MOCK_BUSY_ALL:-0} == 1 || ,${MOCK_BUSY_PORTS:-}, == *,"$port",* ]]; then
    # Deliberately provide no PID and an IPv6 listener.
    printf 'LISTEN 0 128 [::]:%s [::]:*\n' "$port"
fi
MOCK
cat > "$TEST_ROOT/bin/curl" <<'MOCK'
#!/usr/bin/env bash
printf 'CURL %s\n' "$*" >> "$MOCK_LOG"
exit "${MOCK_CURL_EXIT:-0}"
MOCK
chmod +x "$TEST_ROOT/bin/docker" "$TEST_ROOT/bin/ss" "$TEST_ROOT/bin/curl"

fail() { echo "FAIL: $*" >&2; cat "$TEST_ROOT/output" >&2 2>/dev/null || true; exit 1; }
assert_log() { grep -Eq -- "$1" "$MOCK_LOG" || fail "$2"; }
assert_no_log() { if grep -Eq -- "$1" "$MOCK_LOG"; then fail "$2"; fi; }
reset_mock() {
    : > "$MOCK_LOG"
    : > "$TEST_ROOT/output"
    rm -f -- "$PROJECT/.env"
    unset FRONTEND_PORT BACKEND_PORT DOCKER_CMD UPDATE_IMAGES
    export STARTUP_TIMEOUT=1 MOCK_ALL_LOCAL=1 MOCK_PULL_EXIT=0 MOCK_ENGINE=linux MOCK_INFO_EXIT=0
    export MOCK_COMPOSE_VERSION_EXIT=0 MOCK_COMPOSE_BUILD_EXIT=0 MOCK_UP_EXIT=0 MOCK_IMAGE_BUILD_EXIT=0
    export MOCK_SS_EXIT=0 MOCK_BUSY_ALL=0 MOCK_BUSY_PORTS='' MOCK_CONTAINER_IDS='' MOCK_CURL_EXIT=0
}
start_ok() { bash "$PROJECT/scripts/start-linux.sh" "$@" > "$TEST_ROOT/output" 2>&1 || fail 'Startup unexpectedly failed'; }
start_fails() { if bash "$PROJECT/scripts/start-linux.sh" "$@" > "$TEST_ROOT/output" 2>&1; then fail 'Startup unexpectedly succeeded'; fi; }

reset_mock
printf 'TOKEN=keep-me\n# keep comment\nFRONTEND_PORT="80"\nBACKEND_PORT=8080 # API\nFRONTEND_PORT=80\n' > "$PROJECT/.env"
start_ok
grep -qx 'TOKEN=keep-me' "$PROJECT/.env" || fail 'Unrelated dotenv values were lost'
grep -qx '# keep comment' "$PROJECT/.env" || fail 'Comments were lost'
[[ $(grep -c '^FRONTEND_PORT=' "$PROJECT/.env") == 1 ]] || fail 'Duplicate dotenv port keys remain'
assert_no_log '^pull ' 'Normal startup must reuse local images'
assert_no_log '^rm ' 'Startup must never force-remove services'
build_line=$(grep -nE '^compose .* build$' "$MOCK_LOG" | cut -d: -f1)
up_line=$(grep -nE '^compose .* up ' "$MOCK_LOG" | cut -d: -f1)
((build_line < up_line)) || fail 'Services changed before the build finished'
[[ $(grep -c '^CURL ' "$MOCK_LOG") == 3 ]] || fail 'API health, frontend and API proxy were not all checked'

reset_mock
export MOCK_BUSY_PORTS=80,8080
start_ok
grep -qx 'FRONTEND_PORT=8081' "$PROJECT/.env" || fail 'Did not detect occupied host ports without PIDs'
grep -qx 'BACKEND_PORT=8082' "$PROJECT/.env" || fail 'Frontend and backend port selection collided'

reset_mock
export MOCK_CONTAINER_IDS='frontend backend' MOCK_BUSY_ALL=1
printf '/openveth-frontend|frontend|%s\n80|80/tcp\n' "$PROJECT" > "$MOCK_INSPECT_DIR/frontend"
printf '/openveth-backend|backend|%s\n8080|8080/tcp\n' "$PROJECT" > "$MOCK_INSPECT_DIR/backend"
start_ok
grep -qx 'FRONTEND_PORT=80' "$PROJECT/.env" || fail 'Did not reuse own frontend port'
grep -qx 'BACKEND_PORT=8080' "$PROJECT/.env" || fail 'Did not reuse own backend port'

reset_mock
export MOCK_CONTAINER_IDS=foreign
printf '/another-openveth-container|other|/another-project\n80|80/tcp\n' > "$MOCK_INSPECT_DIR/foreign"
start_ok
grep -qx 'FRONTEND_PORT=8080' "$PROJECT/.env" || fail 'Ignored a foreign Docker port because its name contained openveth'
printf '/openveth-frontend|frontend|/another-project\n80|80/tcp\n' > "$MOCK_INSPECT_DIR/foreign"
start_fails
assert_no_log '^rm ' 'A foreign container was removed'

reset_mock
export MOCK_ALL_LOCAL=0 MOCK_PULL_EXIT=1
start_ok
[[ $(grep -c '^build -t openveth/base:latest ' "$MOCK_LOG") == 1 ]] || fail 'Shared base was not built exactly once'
base_line=$(grep -n '^build -t openveth/base:latest ' "$MOCK_LOG" | cut -d: -f1)
server_line=$(grep -n '^build -t openveth/server:latest ' "$MOCK_LOG" | cut -d: -f1)
((base_line < server_line)) || fail 'Server built before its base image'

reset_mock
export MOCK_PULL_EXIT=1
start_ok --update-images
[[ $(grep -c '^pull ' "$MOCK_LOG") == 9 ]] || fail 'Explicit update did not attempt every node image'
assert_no_log '^build -t ' 'Failed image updates should retain usable local images'

for scenario in build image-build ports ss engine compose daemon; do
    reset_mock
    printf 'TOKEN=untouched\n' > "$PROJECT/.env"
    case "$scenario" in
        build) export MOCK_COMPOSE_BUILD_EXIT=1 ;;
        image-build) export MOCK_ALL_LOCAL=0 MOCK_PULL_EXIT=1 MOCK_IMAGE_BUILD_EXIT=1 ;;
        ports) export MOCK_BUSY_ALL=1 ;;
        ss) export MOCK_SS_EXIT=1 ;;
        engine) export MOCK_ENGINE=windows ;;
        compose) export MOCK_COMPOSE_VERSION_EXIT=1 ;;
        daemon) export MOCK_INFO_EXIT=1 ;;
    esac
    start_fails
    assert_no_log '^compose .* up ' "Services changed after failure: $scenario"
    [[ $(cat "$PROJECT/.env") == TOKEN=untouched ]] || fail ".env changed after failure: $scenario"
done

reset_mock
printf 'FRONTEND_PORT=$(touch %s/executed)\n' "$TEST_ROOT" > "$PROJECT/.env"
start_fails
[[ ! -f $TEST_ROOT/executed ]] || fail '.env was executed as shell code'
assert_no_log '^compose .* up ' 'Invalid dotenv port reached Compose startup'

reset_mock
export FRONTEND_PORT=9000 BACKEND_PORT=9001
start_ok
grep -qx 'FRONTEND_PORT=9000' "$PROJECT/.env" || fail 'Exported frontend port was ignored'
assert_log '^CURL .*127.0.0.1:9001/health$' 'Readiness used a different port from Compose'

for scenario in up health; do
    reset_mock
    if [[ $scenario == up ]]; then export MOCK_UP_EXIT=1; else export MOCK_CURL_EXIT=1; fi
    start_fails
    assert_log '^compose .* logs --tail 60 backend frontend$' 'Failed startup did not show diagnostics'
    if grep -qx 'OpenVeth is ready!' "$TEST_ROOT/output"; then fail 'Failed startup announced success'; fi
done

# Ensure Make dispatches a single startup recipe even when invoked in parallel.
if command -v make >/dev/null 2>&1; then
    make --no-print-directory -n -j4 -C "$SOURCE_DIR/.." up > "$TEST_ROOT/make-output"
    [[ $(grep -c 'start-linux.sh' "$TEST_ROOT/make-output") == 1 ]] || fail 'make -j up dispatches startup more than once'
fi

# On Linux, also verify the real ss query against an actual listening socket.
# The mocked suite above still runs on Git Bash without Linux socket tools.
if [[ $(uname -s) == Linux ]] && command -v python3 >/dev/null 2>&1; then
    real_ss=$(PATH=$ORIGINAL_PATH command -v ss) || fail 'Install iproute2 for the native socket test'
    (
        source "$SOURCE_DIR/find-ports.sh"
        ss() { "$real_ss" "$@"; }
        python3 -c 'import socket, sys, time
s = socket.socket()
s.bind(("127.0.0.1", 0))
s.listen()
with open(sys.argv[1], "w") as f:
    f.write(str(s.getsockname()[1]))
time.sleep(30)' "$TEST_ROOT/listener-port" &
        listener_pid=$!
        trap 'kill "$listener_pid" 2>/dev/null || true; wait "$listener_pid" 2>/dev/null || true' EXIT
        for attempt in {1..50}; do [[ ! -s $TEST_ROOT/listener-port ]] || break; sleep 0.1; done
        [[ -s $TEST_ROOT/listener-port ]] || fail 'Native TCP listener did not start'
        port=$(cat "$TEST_ROOT/listener-port")
        if check_port "$port" frontend; then fail 'Native ss check accepted an occupied loopback port'; fi
        kill "$listener_pid"
        wait "$listener_pid" 2>/dev/null || true
        trap - EXIT
        check_port "$port" frontend || fail 'Native ss check rejected the released port'
    )
fi
echo 'All Linux startup regression tests passed.'
