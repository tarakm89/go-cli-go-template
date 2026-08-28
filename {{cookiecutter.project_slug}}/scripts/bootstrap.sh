#!/usr/bin/env bash
# Prepare a macOS or Linux machine to work on this repository.
#
# Installs: Go module dependencies, the pinned dev tools, pre-commit, and the
# git hooks. Safe to re-run. The Windows equivalent is scripts/bootstrap.ps1.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

info()  { printf '\033[1;34m==>\033[0m %s\n' "$1"; }
warn()  { printf '\033[1;33m warn\033[0m %s\n' "$1" >&2; }
die()   { printf '\033[1;31merror\033[0m %s\n' "$1" >&2; exit 1; }

# ------------------------------------------------------------------------ Go
command -v go >/dev/null 2>&1 || die "Go is not installed. See https://go.dev/dl/"

REQUIRED_GO_MINOR=$(awk '/^go /{split($2, v, "."); print v[2]; exit}' go.mod)
ACTUAL_GO_MINOR=$(go env GOVERSION | awk '{split($0, v, "."); print v[2]}')
if [ "${ACTUAL_GO_MINOR:-0}" -lt "${REQUIRED_GO_MINOR:-0}" ]; then
  die "Go 1.${REQUIRED_GO_MINOR} or newer is required, found $(go env GOVERSION)"
fi
info "Go $(go env GOVERSION) on $(go env GOOS)/$(go env GOARCH)"

info "downloading modules"
go mod download

info "installing pinned dev tools into ./bin"
make tools

# ---------------------------------------------------------------- pre-commit
install_pre_commit() {
  if command -v pre-commit >/dev/null 2>&1; then
    info "pre-commit $(pre-commit --version | awk '{print $2}') already installed"
    return 0
  fi

  # Try the tool managers most likely to be present, best first.
  if command -v uv >/dev/null 2>&1; then
    info "installing pre-commit with uv"
    uv tool install pre-commit && return 0
  fi
  if command -v pipx >/dev/null 2>&1; then
    info "installing pre-commit with pipx"
    pipx install pre-commit && return 0
  fi
  if command -v brew >/dev/null 2>&1; then
    info "installing pre-commit with Homebrew"
    brew install pre-commit && return 0
  fi
  for py in python3 python; do
    if command -v "$py" >/dev/null 2>&1; then
      info "installing pre-commit with $py -m pip --user"
      "$py" -m pip install --user --upgrade pre-commit && return 0
    fi
  done

  return 1
}

if install_pre_commit && command -v pre-commit >/dev/null 2>&1; then
  info "installing git hooks"
  pre-commit install --install-hooks
else
  warn "could not install pre-commit automatically."
  warn "install it from https://pre-commit.com/#install and then run: make hooks"
fi

info "done. Next: make check"
