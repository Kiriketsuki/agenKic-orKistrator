#!/usr/bin/env bash
# run.sh — start the orchestrator (which embeds the HTTP/SSE bridge) and the
# Godot GUI together. Ctrl-C (or either process dying) shuts both down.
#
#   ./run.sh
#
# Ports: gRPC :50051, health :8080, bridge :8081 — override via
# GRPC_ADDR / HEALTH_ADDR / BRIDGE_ADDR env vars (read by the orchestrator).

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ORCH_BIN="$REPO_ROOT/bin/orchestrator"
BRIDGE_HOST="${BRIDGE_ADDR:-127.0.0.1:8081}"

log()  { printf '\033[1;32m[run]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[run] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# --- Locate Godot -----------------------------------------------------------

GODOT=""
for c in godot godot4; do
    if command -v "$c" >/dev/null 2>&1; then GODOT="$c"; break; fi
done
[[ -n "$GODOT" ]] || fail "godot not found on PATH — run ./setup.sh first"

# --- Build orchestrator if missing or stale ---------------------------------

stale_bin() {
    [[ ! -x "$ORCH_BIN" ]] && return 0
    # Any Go source/module file newer than the binary means it is stale.
    [[ -n "$(find "$REPO_ROOT/cmd" "$REPO_ROOT/internal" "$REPO_ROOT/go.mod" \
        -name '*.go' -newer "$ORCH_BIN" -print -quit 2>/dev/null)" ]]
}

if stale_bin; then
    log "orchestrator binary missing or stale — building"
    # protoc-gen-go / protoc-gen-go-grpc live in ~/go/bin, which is not on
    # PATH in every shell.
    PATH="$PATH:$HOME/go/bin" make -C "$REPO_ROOT" generate build
fi

# --- Rebuild Godot import cache if a script is newer than the class cache ---
# A stale .godot/global_script_class_cache.cfg makes new class_name scripts
# fail to parse at runtime.

CLASS_CACHE="$REPO_ROOT/godot/.godot/global_script_class_cache.cfg"
if [[ ! -f "$CLASS_CACHE" ]] || [[ -n "$(find "$REPO_ROOT/godot/scripts" \
    -name '*.gd' -newer "$CLASS_CACHE" -print -quit 2>/dev/null)" ]]; then
    log "Godot class cache missing or stale — reimporting"
    "$GODOT" --headless --path "$REPO_ROOT/godot" --import >/dev/null 2>&1 || true
fi

# --- Cleanup: one handler for Ctrl-C and normal exit ------------------------

ORCH_PID=""
GODOT_PID=""

cleanup() {
    trap - INT TERM EXIT
    log "shutting down"
    [[ -n "$GODOT_PID" ]] && kill "$GODOT_PID" 2>/dev/null || true
    # SIGTERM triggers the orchestrator's graceful shutdown path.
    [[ -n "$ORCH_PID" ]] && kill -TERM "$ORCH_PID" 2>/dev/null || true
    [[ -n "$GODOT_PID" ]] && wait "$GODOT_PID" 2>/dev/null || true
    [[ -n "$ORCH_PID" ]] && wait "$ORCH_PID" 2>/dev/null || true
    log "done"
}
trap cleanup INT TERM EXIT

# --- Start orchestrator -----------------------------------------------------

log "starting orchestrator"
"$ORCH_BIN" &
ORCH_PID=$!

# Wait for the bridge port to accept connections (up to 10s).
bridge_up=false
for _ in $(seq 1 50); do
    if ! kill -0 "$ORCH_PID" 2>/dev/null; then
        ORCH_PID=""
        fail "orchestrator exited during startup"
    fi
    if (exec 3<>"/dev/tcp/${BRIDGE_HOST%:*}/${BRIDGE_HOST#*:}") 2>/dev/null; then
        exec 3>&- 3<&-
        bridge_up=true
        break
    fi
    sleep 0.2
done
$bridge_up || fail "bridge did not come up on $BRIDGE_HOST within 10s"
log "bridge ready on $BRIDGE_HOST"

# --- Start GUI --------------------------------------------------------------

log "starting Godot GUI"
"$GODOT" --path "$REPO_ROOT/godot" &
GODOT_PID=$!

# --- Wait: exit when either process ends (or Ctrl-C fires the trap) ---------

wait -n "$ORCH_PID" "$GODOT_PID" 2>/dev/null || true
# cleanup runs via the EXIT trap
