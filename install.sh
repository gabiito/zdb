#!/usr/bin/env bash
# Install zdb for the current user.
#
# - Verifies Go is installed.
# - Builds via `make build` (CGO-free, no external deps).
# - Copies the binary into a directory that's on $PATH (prefers
#   ~/.local/bin, then ~/bin, then $GOPATH/bin; falls back to
#   ~/.local/bin and warns if it isn't on $PATH yet).
# - Creates the config and state directories so the first run can write.
#
# Re-running is safe — it overwrites the same destination.

set -euo pipefail

# ── output helpers ────────────────────────────────────────────────────────────
if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
	GREEN=$'\e[32m'
	YELLOW=$'\e[33m'
	RED=$'\e[31m'
	BOLD=$'\e[1m'
	RESET=$'\e[0m'
else
	GREEN=''
	YELLOW=''
	RED=''
	BOLD=''
	RESET=''
fi

info() { echo "${GREEN}▸${RESET} $*"; }
warn() { echo "${YELLOW}!${RESET} $*" >&2; }
fatal() {
	echo "${RED}✗${RESET} $*" >&2
	exit 1
}

# ── work from the repo root regardless of where the user invokes from ────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# ── pre-flight ───────────────────────────────────────────────────────────────
command -v go >/dev/null || fatal "Go is not installed. Get it from https://go.dev/dl/ (≥ 1.21)."
command -v make >/dev/null || fatal "make is not installed. Install your platform's build tools."

GO_VERSION=$(go env GOVERSION)
info "Using $GO_VERSION"

# ── build ────────────────────────────────────────────────────────────────────
info "Building zdb (this auto-resolves go.mod dependencies)…"
make build >/dev/null
[[ -x bin/zdb ]] || fatal "Build did not produce bin/zdb"

# ── pick install destination — prefer one that's ALREADY on $PATH ────────────
GOBIN="$(go env GOPATH)/bin"
case ":$PATH:" in
	*":$HOME/.local/bin:"*) DEST="$HOME/.local/bin" ;;
	*":$HOME/bin:"*)         DEST="$HOME/bin" ;;
	*":$GOBIN:"*)            DEST="$GOBIN" ;;
	*)                       DEST="$HOME/.local/bin" ;;
esac

mkdir -p "$DEST"
install -m 0755 bin/zdb "$DEST/zdb"
info "Installed → ${BOLD}$DEST/zdb${RESET}"

# ── PATH sanity check ────────────────────────────────────────────────────────
case ":$PATH:" in
	*":$DEST:"*)
		info "$DEST is on \$PATH — you can run ${BOLD}zdb${RESET} from anywhere."
		;;
	*)
		warn "$DEST is NOT on your \$PATH yet."
		warn "Add this line to your shell rc (~/.zshrc / ~/.bashrc):"
		echo "    export PATH=\"$DEST:\$PATH\""
		warn "Then reload it: ${BOLD}source ~/.zshrc${RESET} (or ~/.bashrc)."
		;;
esac

# ── prep config + state dirs so first run doesn't trip on missing paths ──────
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/zdb"
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/zdb"
mkdir -p "$CONFIG_DIR" "$STATE_DIR"
info "Config dir → $CONFIG_DIR"
info "State dir  → $STATE_DIR"

# ── final hint ───────────────────────────────────────────────────────────────
echo
info "Done. Try:"
echo "    ${BOLD}zdb${RESET}                                  # welcome screen on first run"
echo "    ${BOLD}ZDB_DEBUG=1 zdb${RESET}                      # debug log → $STATE_DIR/log"
echo "    ${BOLD}./test-data/apply.sh sqlite${RESET}          # populate /tmp/dev.db with sample data"
