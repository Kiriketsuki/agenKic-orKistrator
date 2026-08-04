#!/usr/bin/env bash
# setup.sh — install all build/run dependencies for agenKic-orKistrator.
#
# Run as your normal user; sudo is invoked internally for system packages.
#   ./setup.sh
#
# Installs: Go, protoc, tmux, Docker, Godot 4.3+, protoc Go plugins,
# godot-xterm addon, and starts Redis via docker compose.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GODOT_VERSION="4.4.1"

log()  { printf '\033[1;32m[setup]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[setup] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

if [[ "$(id -u)" -eq 0 ]]; then
    fail "Run as a normal user, not root — the script uses sudo where needed. (go install must run as your user.)"
fi

# --- OS detection -----------------------------------------------------------

OS=""
case "$(uname -s)" in
    Linux)
        [[ -r /etc/os-release ]] || fail "cannot detect Linux distro (/etc/os-release missing)"
        # shellcheck disable=SC1091
        . /etc/os-release
        case "${ID}${ID_LIKE:+ $ID_LIKE}" in
            *arch*)           OS=arch ;;
            *debian*|*ubuntu*) OS=debian ;;
            *) fail "unsupported distro: ${ID}. Install manually: go, protobuf, tmux, docker, godot >= 4.3" ;;
        esac
        ;;
    Darwin) OS=macos ;;
    *) fail "unsupported OS: $(uname -s). Note: godot-xterm PTY is Linux/macOS only." ;;
esac
log "detected OS: $OS"

# --- System packages --------------------------------------------------------

case "$OS" in
    arch)
        log "installing system packages via pacman"
        sudo pacman -S --needed --noconfirm go protobuf tmux docker docker-compose godot
        ;;
    debian)
        log "installing system packages via apt"
        sudo apt-get update
        sudo apt-get install -y golang-go protobuf-compiler tmux docker.io docker-compose-plugin unzip curl
        if ! command -v godot >/dev/null 2>&1; then
            log "installing Godot ${GODOT_VERSION} from GitHub releases (not packaged for Debian/Ubuntu)"
            arch_suffix="$(uname -m | sed 's/x86_64/x86_64/; s/aarch64/arm64/')"
            tmp="$(mktemp -d)"
            curl -fsSL -o "$tmp/godot.zip" \
                "https://github.com/godotengine/godot/releases/download/${GODOT_VERSION}-stable/Godot_v${GODOT_VERSION}-stable_linux.${arch_suffix}.zip"
            unzip -q "$tmp/godot.zip" -d "$tmp"
            sudo install -m 0755 "$tmp/Godot_v${GODOT_VERSION}-stable_linux.${arch_suffix}" /usr/local/bin/godot
            rm -rf "$tmp"
        fi
        ;;
    macos)
        command -v brew >/dev/null 2>&1 || fail "Homebrew required: https://brew.sh"
        log "installing system packages via brew"
        brew install go protobuf tmux
        brew install --cask godot docker
        ;;
esac

# --- Docker daemon (Linux) --------------------------------------------------

if [[ "$OS" != "macos" ]]; then
    if ! docker info >/dev/null 2>&1; then
        log "starting docker daemon"
        sudo systemctl enable --now docker
    fi
    if ! id -nG "$USER" | grep -qw docker; then
        log "adding $USER to docker group (takes effect on next login)"
        sudo usermod -aG docker "$USER"
    fi
fi

# --- Go protoc plugins ------------------------------------------------------

log "installing protoc-gen-go and protoc-gen-go-grpc"
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

GOBIN="$(go env GOPATH)/bin"
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$GOBIN"; then
    log "NOTE: add $GOBIN to your PATH (needed by 'make generate'):"
    log "  export PATH=\"\$PATH:$GOBIN\""
fi

# --- godot-xterm addon ------------------------------------------------------

if [[ ! -d "$REPO_ROOT/godot/addons/godot_xterm" ]]; then
    log "installing godot-xterm addon (live PTY terminal)"
    bash "$REPO_ROOT/godot/addons/install_godot_xterm.sh"
else
    log "godot-xterm addon already present"
fi

# --- Generate protobuf + build ----------------------------------------------

log "generating protobuf code and building orchestrator"
PATH="$PATH:$GOBIN" make -C "$REPO_ROOT" generate build

# --- Redis ------------------------------------------------------------------

log "starting Redis via docker compose"
if docker info >/dev/null 2>&1; then
    docker compose -f "$REPO_ROOT/docker-compose.yml" up -d redis
else
    log "NOTE: docker not usable in this shell yet (group change pending) — after re-login run:"
    log "  docker compose up -d redis"
fi

# --- Verify -----------------------------------------------------------------

log "verifying installation"
ok=true
for c in go protoc tmux docker godot; do
    if command -v "$c" >/dev/null 2>&1; then
        printf '  %-10s %s\n' "$c" "$(command -v "$c")"
    else
        printf '  %-10s MISSING\n' "$c"; ok=false
    fi
done
for c in protoc-gen-go protoc-gen-go-grpc; do
    if [[ -x "$GOBIN/$c" ]]; then
        printf '  %-18s %s\n' "$c" "$GOBIN/$c"
    else
        printf '  %-18s MISSING\n' "$c"; ok=false
    fi
done
[[ -x "$REPO_ROOT/bin/orchestrator" ]] && printf '  %-18s built\n' "orchestrator" || { printf '  %-18s NOT BUILT\n' "orchestrator"; ok=false; }

$ok && log "setup complete" || fail "setup finished with missing components — see above"
